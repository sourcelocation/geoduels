import { ObservableStore } from "../../../lib/observable-store";
import type { RuntimeConfig } from "../../../lib/runtime-config";
import type { SessionController } from "../../auth/controllers/session-controller";
import type { AuthSessionSnapshot } from "../../auth/session";
import type { MatchConfig } from "../../matchmaking/lib/queue-client";
import {
  applyPartyPatch,
  createParty,
  joinParty,
  type PartySnapshot,
  type PartyMember,
  type PartyTeamId,
  type PartyMode,
} from "../lib/party-client";
import { PartySocket } from "../lib/party-socket";

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
  launchMatchId?: string;
};

const initialState: PartyRuntimeState = {
  status: "idle",
  partyId: "",
  inviteCode: "",
  snapshot: null,
  self: null,
  error: "",
  launchMatchId: "",
};

function getErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message) return error.message;
  return fallback;
}

export class PartyController extends ObservableStore<PartyRuntimeState> {
  private readonly config: RuntimeConfig;
  private readonly sessionController: SessionController;
  private readonly onMatchFound: (matchId: string) => Promise<boolean>;
  private state: PartyRuntimeState = initialState;
  private streamAbort: AbortController | null = null;
  private socket: PartySocket | null = null;
  private reconnectTimeout: number | null = null;
  private reconnectAttempt = 0;
  private handledMatchId = "";
  private connectRequestId = 0;
  private destroyed = false;

  constructor(params: {
    config: RuntimeConfig;
    sessionController: SessionController;
    onMatchFound: (matchId: string) => Promise<boolean>;
  }) {
    super();
    this.config = params.config;
    this.sessionController = params.sessionController;
    this.onMatchFound = params.onMatchFound;
  }

  getState() {
    return this.state;
  }

  destroy() {
    this.destroyed = true;
    this.clearReconnectTimer();
    this.abortStream();
  }

  reset = () => {
    this.clearReconnectTimer();
    this.abortStream();
    this.handledMatchId = "";
    this.patchState(initialState);
  };

  // Restoration only subscribes to server-validated membership; it never joins.
  restoreParty = async (party: { id: string; inviteCode: string }) => {
    if (this.state.status !== "idle" || this.destroyed) return;
    this.patchState({ partyId: party.id, inviteCode: party.inviteCode, status: "reconnecting" });
    try {
      await this.ensureStream();
    } catch {
      this.scheduleReconnect();
    }
  };

  admitParty = async (inviteCode: string) => {
    const code = inviteCode.trim().toUpperCase();
    if (!code) return;
    if (
      this.state.inviteCode === code &&
      this.isCurrentUserMember(this.state.snapshot)
    ) {
      if (!this.socket || this.state.status !== "ready") await this.ensureStream();
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
      const admission = await joinParty(this.config, code, session.accessToken);
      this.patchState({
        partyId: admission.id,
        inviteCode: admission.inviteCode,
        snapshot: null,
        self: null,
      });
      await this.connectToParty(session, admission.id, { waitForSnapshot: true });
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
      const admission = await createParty(
        this.config,
        session.accessToken,
        mode,
        matchConfig,
      );
      this.handledMatchId = "";
      this.patchState({
        status: "admitting",
        partyId: admission.id,
        inviteCode: admission.inviteCode,
        snapshot: null,
        self: null,
      });
      await this.connectToParty(session, admission.id, { waitForSnapshot: true });
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
      const admission = await joinParty(this.config, code, session.accessToken);
      this.handledMatchId = "";
      this.patchState({
        status: "admitting",
        partyId: admission.id,
        inviteCode: admission.inviteCode,
        snapshot: null,
        self: null,
      });
      await this.connectToParty(session, admission.id, { waitForSnapshot: true });
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
    this.patchState({ status: "leaving", error: "" });
    try {
      await this.requireSocket().command("leave", {});
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
      await this.requireSocket().command("kick", { userId });
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
      await this.requireSocket().command("transfer_owner", { userId });
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
      await this.requireSocket().command("start", {});
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
      await this.requireSocket().command("settings", { config: matchConfig, mode });
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
      await this.requireSocket().command("team", { teamId });
    } catch (error) {
      this.patchState({
        error: getErrorMessage(error, "Could not switch team"),
      });
    }
  };

  shuffleTeams = async () => {
    const session = this.sessionController.getSessionSnapshot();
    if (!this.state.partyId || !session) return;
    this.patchState({ error: "" });
    try {
      await this.requireSocket().command("shuffle_teams", {});
    } catch (error) {
      this.patchState({
        error: getErrorMessage(error, "Could not shuffle teams"),
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
    const partyId = this.state.partyId;
    const requestId = this.connectRequestId;
    const userId = this.sessionController.getSessionSnapshot()?.userId;
    if (!partyId || !userId || this.destroyed) return;
    const session = await this.sessionController.ensureFreshSession();
    if (requestId !== this.connectRequestId || partyId !== this.state.partyId || this.destroyed) return;
    if (!session || session.userId !== userId) throw new Error("Session unavailable");
    await this.connectToParty(session, partyId);
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
    const socket = new PartySocket(
      this.config,
      partyId,
      session.accessToken,
      controller.signal,
      (event) => {
        if (requestId !== this.connectRequestId) return;
        if (event.type === "party_snapshot") {
          if (readyTimeout) window.clearTimeout(readyTimeout);
          if (event.party.state === "closed" || event.party.state === "expired") {
            readyReject?.(new Error("Party is closed"));
            this.reset();
            return;
          }
          this.reconnectAttempt = 0;
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
            this.patchSnapshot(next, "ready");
          }
          return;
        }
        if (event.type === "match_found") {
          if (this.handledMatchId === event.matchId) return;
          this.handledMatchId = event.matchId;
          // Publish the navigation target immediately and cancel stale route work
          // through the shared route controller before connecting to this match.
          void this.onMatchFound(event.matchId).then((ok) => {
            if (!ok && requestId === this.connectRequestId) this.handledMatchId = "";
          }).catch((error) => {
            if (requestId !== this.connectRequestId) return;
            this.handledMatchId = "";
            this.patchState({ error: getErrorMessage(error, "Could not join the party's match") });
          });
          this.patchState({ launchMatchId: event.matchId });
          return;
        }
        if (event.type === "party_error") {
          if (readyTimeout) window.clearTimeout(readyTimeout);
          this.patchState({ status: "error", error: event.message });
          if (event.message.toLowerCase().includes("left this party") || event.message === "Party unavailable") {
            this.reset();
          }
          readyReject?.(new Error(event.message));
        }
      },
    );
    this.socket = socket;
    void socket.closed
      .then(() => {
        if (requestId !== this.connectRequestId) return;
        if (readyReject) {
          if (readyTimeout) window.clearTimeout(readyTimeout);
          readyReject(new Error("Party connection closed"));
          return;
        }
        if (this.state.partyId) {
          this.patchState({ status: "reconnecting" });
          this.scheduleReconnect();
        }
      })
      .catch((error) => {
        if (requestId !== this.connectRequestId) return;
        if (error?.name === "AbortError") return;
        if (readyTimeout) window.clearTimeout(readyTimeout);
        this.patchState({
          status: this.state.partyId ? "reconnecting" : "error",
          error: getErrorMessage(error, "Party connection failed"),
        });
        readyReject?.(
          error instanceof Error
            ? error
            : new Error("Party connection failed"),
        );
        if (!readyReject && this.state.partyId) {
          this.scheduleReconnect();
        }
      });
    return ready;
  }

  private abortStream() {
    this.streamAbort?.abort();
    this.streamAbort = null;
    this.socket = null;
    this.connectRequestId += 1;
  }

  private requireSocket() {
    if (!this.socket) throw new Error("Party connection unavailable");
    return this.socket;
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

  private patchState(patch: Partial<PartyRuntimeState>) {
    this.state = { ...this.state, ...patch };
    if (!this.destroyed) {
      this.emit();
    }
  }
}
