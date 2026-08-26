import { ObservableStore } from "../../../lib/observable-store";
import type { RuntimeConfig } from "../../../lib/runtime-config";
import type { SessionController } from "../../auth/controllers/session-controller";
import type { AuthSessionSnapshot } from "../../auth/session";
import type { MatchController } from "../../matchmaking/controllers/match-controller";
import type { MatchConfig } from "../../matchmaking/lib/queue-client";
import {
  applyPartyPatch,
  createParty,
  fetchParty,
  joinParty,
  kickPartyMember,
  leaveParty,
  startParty,
  streamParty,
  touchPartyPresence,
  transferPartyOwner,
  updatePartySettings,
  updatePartyTeam,
  type PartySnapshot,
  type PartyMember,
  type PartyTeamId,
  type PartyMode,
} from "../lib/party-client";

export type PartyRuntimeStatus =
  | "idle"
  | "admitting"
  | "ready"
  | "reconnecting"
  | "leaving"
  | "error";

export type PartyRuntimeState = {
  status: PartyRuntimeStatus;
  partyId: string;
  inviteCode: string;
  snapshot: PartySnapshot | null;
  self: PartyMember | null;
  error: string;
};

const initialState: PartyRuntimeState = {
  status: "idle",
  partyId: "",
  inviteCode: "",
  snapshot: null,
  self: null,
  error: "",
};

function getErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message) return error.message;
  return fallback;
}

export function jitteredPartyDelay(baseMs = 5000): number {
  return Math.max(1000, Math.round(baseMs * (0.8 + Math.random() * 0.4)));
}

export class PartyController extends ObservableStore<PartyRuntimeState> {
  private readonly config: RuntimeConfig;
  private readonly sessionController: SessionController;
  private readonly matchController: MatchController;
  private state: PartyRuntimeState = initialState;
  private streamAbort: AbortController | null = null;
  private streamSession: AuthSessionSnapshot | null = null;
  private presenceInterval: number | null = null;
  private pollInterval: number | null = null;
  private reconnectTimeout: number | null = null;
  private reconnectAttempt = 0;
  private handledMatchId = "";
  private connectRequestId = 0;
  private destroyed = false;

  constructor(params: {
    config: RuntimeConfig;
    sessionController: SessionController;
    matchController: MatchController;
  }) {
    super();
    this.config = params.config;
    this.sessionController = params.sessionController;
    this.matchController = params.matchController;
  }

  getState() {
    return this.state;
  }

  destroy() {
    this.destroyed = true;
    this.stopPresenceLoop();
    this.stopPollLoop();
    this.clearReconnectTimer();
    this.abortStream();
  }

  reset = () => {
    this.stopPresenceLoop();
    this.stopPollLoop();
    this.clearReconnectTimer();
    this.abortStream();
    this.handledMatchId = "";
    this.patchState(initialState);
  };

  admitParty = async (inviteCode: string) => {
    const code = inviteCode.trim().toUpperCase();
    if (!code) return;
    if (
      this.state.inviteCode === code &&
      this.isCurrentUserMember(this.state.snapshot)
    ) {
      await this.ensureStream();
      return;
    }
    if (
      this.state.inviteCode === code &&
      this.state.status === "admitting"
    ) {
      return;
    }
    this.patchState({
      status: "admitting",
      inviteCode: code,
      partyId: "",
      snapshot: null,
      self: null,
      error: "",
    });
    try {
      const session = await this.playableSession();
      if (!session) return;
      const snap = await joinParty(this.config, code, session.accessToken);
      this.assertCurrentUserMember(snap);
      this.patchState({
        partyId: snap.id,
        inviteCode: snap.inviteCode,
        snapshot: snap,
        self: this.currentUserMember(snap),
      });
      this.markExistingMatchHandled(snap);
      await this.connectToParty(session, snap.id, { waitForSnapshot: true });
    } catch (error) {
      this.patchState({
        status: "error",
        error: getErrorMessage(error, "Party unavailable"),
      });
    }
  };

  createParty = async (mode: PartyMode = "duel", matchConfig?: MatchConfig) => {
    this.patchState({ status: "admitting", error: "" });
    try {
      const session = await this.playableSession();
      if (!session) return false;
      const snap = await createParty(
        this.config,
        session.accessToken,
        mode,
        matchConfig,
      );
      this.assertCurrentUserMember(snap);
      this.handledMatchId = "";
      this.patchState({
        status: "admitting",
        partyId: snap.id,
        inviteCode: snap.inviteCode,
        snapshot: snap,
        self: this.currentUserMember(snap),
      });
      this.markExistingMatchHandled(snap);
      await this.connectToParty(session, snap.id, { waitForSnapshot: true });
      return this.state.status === "ready" && !!this.state.snapshot;
    } catch (error) {
      this.patchState({
        status: "error",
        error: getErrorMessage(error, "Party unavailable"),
      });
      return false;
    }
  };

  joinParty = async (requestedInviteCode?: string) => {
    const code = (
      requestedInviteCode ||
      this.state.inviteCode ||
      this.state.snapshot?.inviteCode ||
      ""
    )
      .trim()
      .toUpperCase();
    if (!code) {
      this.patchState({ error: "Party invite is missing." });
      return false;
    }
    this.patchState({ status: "admitting", inviteCode: code, error: "" });
    try {
      const session = await this.playableSession();
      if (!session) return false;
      const snap = await joinParty(this.config, code, session.accessToken);
      this.assertCurrentUserMember(snap);
      this.handledMatchId = "";
      this.patchState({
        status: "admitting",
        partyId: snap.id,
        inviteCode: snap.inviteCode,
        snapshot: snap,
        self: this.currentUserMember(snap),
      });
      this.markExistingMatchHandled(snap);
      await this.connectToParty(session, snap.id, { waitForSnapshot: true });
      return this.state.status === "ready" && !!this.state.snapshot;
    } catch (error) {
      this.patchState({
        status: "error",
        error: getErrorMessage(error, "Could not join party"),
      });
      return false;
    }
  };

  leaveParty = async () => {
    if (!this.state.partyId) return;
    const partyId = this.state.partyId;
    const session = this.sessionController.getSessionSnapshot();
    this.patchState({ status: "leaving", error: "" });
    try {
      if (session) {
        await leaveParty(this.config, partyId, session.accessToken);
      }
      this.reset();
    } catch (error) {
      this.patchState({
        status: "error",
        error: getErrorMessage(error, "Could not leave party"),
      });
    }
  };

  kickMember = async (userId: string) => {
    const session = this.sessionController.getSessionSnapshot();
    if (!this.state.partyId || !session) return;
    this.patchState({ error: "" });
    try {
      const next = await kickPartyMember(
        this.config,
        this.state.partyId,
        session.accessToken,
        userId,
      );
      this.patchSnapshot(next);
    } catch (error) {
      this.patchState({
        error: getErrorMessage(error, "Could not kick player"),
      });
    }
  };

  transferOwner = async (userId: string) => {
    const session = this.sessionController.getSessionSnapshot();
    if (!this.state.partyId || !session) return;
    this.patchState({ error: "" });
    try {
      const next = await transferPartyOwner(
        this.config,
        this.state.partyId,
        session.accessToken,
        userId,
      );
      this.patchSnapshot(next);
    } catch (error) {
      this.patchState({
        error: getErrorMessage(error, "Could not transfer leader"),
      });
    }
  };

  startParty = async () => {
    const session = this.sessionController.getSessionSnapshot();
    if (!this.state.partyId || !session) return;
    this.patchState({ error: "" });
    try {
      const assignment = await startParty(
        this.config,
        this.state.partyId,
        session.accessToken,
      );
      this.handledMatchId = assignment.matchId;
      await this.matchController.resumeResolvedMatch(assignment, {
        playMatchFoundSfx: true,
      });
    } catch (error) {
      this.patchState({
        error: getErrorMessage(error, "Could not start party"),
      });
    }
  };

  updateSettings = async (matchConfig: MatchConfig, mode?: PartyMode) => {
    const session = this.sessionController.getSessionSnapshot();
    if (!this.state.partyId || !session) return;
    this.patchState({ error: "" });
    try {
      const next = await updatePartySettings(
        this.config,
        this.state.partyId,
        session.accessToken,
        matchConfig,
        mode,
      );
      this.patchSnapshot(next);
    } catch (error) {
      this.patchState({
        error: getErrorMessage(error, "Could not update party settings"),
      });
    }
  };

  switchTeam = async (teamId: PartyTeamId) => {
    const session = this.sessionController.getSessionSnapshot();
    if (!this.state.partyId || !session) return;
    this.patchState({ error: "" });
    try {
      const next = await updatePartyTeam(
        this.config,
        this.state.partyId,
        session.accessToken,
        teamId,
      );
      this.patchSnapshot(next);
    } catch (error) {
      this.patchState({
        error: getErrorMessage(error, "Could not switch team"),
      });
    }
  };

  private async playableSession() {
    const session = await this.sessionController.getPlayableSession();
    if (!session) {
      this.patchState({
        status: "error",
        error: "Could not start a guest session.",
      });
      return null;
    }
    return session;
  }

  private async ensureStream() {
    if (!this.state.partyId || !this.isCurrentUserMember(this.state.snapshot)) {
      return;
    }
    const session =
      this.sessionController.getSessionSnapshot() ||
      (await this.sessionController.ensureFreshSession());
    if (!session) return;
    await this.connectToParty(session, this.state.partyId);
  }

  private async connectToParty(
    session: AuthSessionSnapshot,
    partyId: string,
    options?: { waitForSnapshot?: boolean },
  ) {
    this.clearReconnectTimer();
    this.abortStream();
    const controller = new AbortController();
    const requestId = ++this.connectRequestId;
    this.streamAbort = controller;
    this.streamSession = session;
    this.patchState({
      status: options?.waitForSnapshot
        ? "admitting"
        : this.state.snapshot
          ? "reconnecting"
          : "admitting",
      error: "",
    });
    let readyResolve: (() => void) | null = null;
    let readyReject: ((error: Error) => void) | null = null;
    let readyTimeout: number | null = null;
    const ready = options?.waitForSnapshot
      ? new Promise<void>((resolve, reject) => {
          readyResolve = resolve;
          readyReject = reject;
          readyTimeout = window.setTimeout(() => {
            readyReject?.(new Error("Party connection timed out"));
            controller.abort();
          }, 10000);
        })
      : Promise.resolve();
    void streamParty(
      this.config,
      session,
      partyId,
      controller.signal,
      (event) => {
        if (requestId !== this.connectRequestId) return;
        if (event.type === "party_snapshot") {
          if (readyTimeout) window.clearTimeout(readyTimeout);
          this.reconnectAttempt = 0;
          this.stopPollLoop();
          this.startPresenceLoop();
          this.patchSnapshot(event.party, "ready");
          readyResolve?.();
          readyResolve = null;
          readyReject = null;
          return;
        }
        if (event.type === "party_patch") {
          const next = applyPartyPatch(this.state.snapshot, event.patch);
          if (next) {
            this.reconnectAttempt = 0;
            this.startPresenceLoop();
            this.patchSnapshot(next, "ready");
          }
          return;
        }
        if (event.type === "match_assigned") {
          if (this.handledMatchId === event.assignment.matchId) return;
          this.handledMatchId = event.assignment.matchId;
          void this.matchController.resumeResolvedMatch(event.assignment, {
            playMatchFoundSfx: true,
          });
          return;
        }
        if (event.type === "party_error") {
          if (readyTimeout) window.clearTimeout(readyTimeout);
          this.patchState({ status: "error", error: event.message });
          if (event.message.toLowerCase().includes("left this party")) {
            this.reset();
          }
          readyReject?.(new Error(event.message));
        }
      },
    )
      .then(() => {
        if (requestId !== this.connectRequestId) return;
        if (readyReject) {
          if (readyTimeout) window.clearTimeout(readyTimeout);
          readyReject(new Error("Party connection closed"));
          return;
        }
        if (this.state.snapshot && this.state.partyId) {
          this.patchState({ status: "reconnecting" });
          this.startPollLoop();
          this.scheduleReconnect();
        }
      })
      .catch((error) => {
        if (requestId !== this.connectRequestId) return;
        if (error?.name === "AbortError") return;
        if (readyTimeout) window.clearTimeout(readyTimeout);
        this.patchState({
          status: this.state.snapshot ? "reconnecting" : "error",
          error: getErrorMessage(error, "Party connection failed"),
        });
        readyReject?.(
          error instanceof Error
            ? error
            : new Error("Party connection failed"),
        );
        if (!readyReject && this.state.snapshot && this.state.partyId) {
          this.startPollLoop();
          this.scheduleReconnect();
        }
      });
    return ready;
  }

  private abortStream() {
    this.streamAbort?.abort();
    this.streamAbort = null;
    this.streamSession = null;
  }

  private startPresenceLoop() {
    if (this.presenceInterval) return;
    const tick = () => {
      this.presenceInterval = window.setTimeout(tick, jitteredPartyDelay());
      const session = this.streamSession || this.sessionController.getSessionSnapshot();
      const partyId = this.state.partyId;
      if (!session?.accessToken || !partyId || !this.isCurrentUserMember(this.state.snapshot)) return;
      void touchPartyPresence(this.config, partyId, session.accessToken).catch(() => {
        // Presence is advisory; reconnect/polling handles visible recovery.
      });
    };
    tick();
  }

  private stopPresenceLoop() {
    if (this.presenceInterval) window.clearTimeout(this.presenceInterval);
    this.presenceInterval = null;
  }

  private startPollLoop() {
    if (this.pollInterval) return;
    const poll = () => {
      this.pollInterval = window.setTimeout(poll, jitteredPartyDelay());
      const code = this.state.inviteCode || this.state.snapshot?.inviteCode || "";
      if (!code) return;
      const session = this.streamSession || this.sessionController.getSessionSnapshot();
      if (!session) return;
      void fetchParty(this.config, code, session.accessToken)
        .then((snap) => {
          if (snap) this.patchSnapshot(snap, this.state.status === "reconnecting" ? "reconnecting" : "ready");
        })
        .catch(() => {});
    };
    poll();
  }

  private stopPollLoop() {
    if (this.pollInterval) window.clearTimeout(this.pollInterval);
    this.pollInterval = null;
  }

  private clearReconnectTimer() {
    if (this.reconnectTimeout) window.clearTimeout(this.reconnectTimeout);
    this.reconnectTimeout = null;
  }

  private scheduleReconnect() {
    if (this.reconnectTimeout || !this.state.partyId) return;
    const delays = [1000, 2000, 5000, 10000];
    const delay = delays[Math.min(this.reconnectAttempt, delays.length - 1)];
    this.reconnectAttempt += 1;
    this.reconnectTimeout = window.setTimeout(() => {
      this.reconnectTimeout = null;
      void this.ensureStream().catch(() => {
        this.scheduleReconnect();
      });
    }, delay);
  }

  private patchSnapshot(next: PartySnapshot, status: PartyRuntimeStatus = "ready") {
    const snapshot = next;
    const self = this.currentUserMember(snapshot);
    if (!self) {
      this.reset();
      return;
    }
    this.patchState({
      status,
      partyId: snapshot.id,
      inviteCode: snapshot.inviteCode,
      snapshot,
      self,
      error: "",
    });
  }

  private isCurrentUserMember(snapshot: PartySnapshot | null) {
    return !!this.currentUserMember(snapshot);
  }

  private currentUserMember(snapshot: PartySnapshot | null) {
    const session = this.sessionController.getSessionSnapshot();
    if (!snapshot || !session?.userId) return null;
    return snapshot.members.find((member) => member.userId === session.userId) || null;
  }

  private assertCurrentUserMember(snapshot: PartySnapshot) {
    if (!this.isCurrentUserMember(snapshot)) {
      throw new Error("Party admission did not include the current player");
    }
  }

  private markExistingMatchHandled(snapshot: PartySnapshot) {
    if (snapshot.state !== "in_match" && snapshot.state !== "started") return;
    this.handledMatchId = snapshot.activeMatchId || snapshot.startedMatchId || "";
  }

  private patchState(patch: Partial<PartyRuntimeState>) {
    this.state = { ...this.state, ...patch };
    if (!this.destroyed) {
      this.emit();
    }
  }
}
