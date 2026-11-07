import { Listeners } from "../../../lib/listeners";
import type { SessionController } from "../../auth/controllers/session-controller";
import type { MatchConfig } from "../../matchmaking/lib/queue-client";
import {
  createParty,
  joinParty,
  partyCommand,
  type PartyMember,
  type PartyMode,
  type PartySnapshot,
  type PartyTeamId,
} from "../lib/party-client";

export type PartyRuntimeStatus = "idle" | "admitting" | "ready" | "leaving" | "error";

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

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error && error.message ? error.message : fallback;
}

/**
 * The party this person is in, as the server last said: in answer to something they did, in a
 * party.updated, or when they connect. Whichever comes, the highest revision wins.
 */
export class PartyController {
  private readonly listeners = new Listeners();
  readonly subscribe = this.listeners.subscribe;
  private readonly sessionController: SessionController;
  private state: PartyRuntimeState = initialState;

  constructor(params: { sessionController: SessionController }) {
    this.sessionController = params.sessionController;
  }

  getState = () => this.state;

  reset = () => this.patch(initialState);

  applySnapshot(party: PartySnapshot) {
    const current = this.state.snapshot;
    const samePartyAsShown = current?.id === party.id;
    if (samePartyAsShown && party.revision < current.revision) return;
    const self = party.members.find((member) => member.userId === this.userId()) ?? null;
    const over = party.state === "closed" || party.state === "expired";
    if (!self || over) {
      if (samePartyAsShown) this.reset();
      return;
    }
    this.patch({ status: "ready", partyId: party.id, inviteCode: party.inviteCode, snapshot: party, self, error: "" });
  }

  applySession(party?: PartySnapshot) {
    if (party) this.applySnapshot(party);
    else if (this.state.status !== "admitting") this.reset();
  }

  applyLeft(partyId: string) {
    if (this.state.partyId === partyId) this.reset();
  }

  /** Joins by a link, unless already in that party. */
  admitParty = async (inviteCode: string) => {
    const code = inviteCode.trim().toUpperCase();
    if (!code || this.state.inviteCode === code && (this.state.self || this.state.status === "admitting")) return;
    await this.joinParty(code);
  };

  joinParty = async (requestedCode?: string) => {
    const code = (requestedCode || this.state.inviteCode).trim().toUpperCase();
    if (!code) {
      this.patch({ error: "Party invite is missing." });
      return false;
    }
    this.patch({ ...initialState, status: "admitting", inviteCode: code });
    return this.enter("Could not join party", (token) => joinParty(code, token));
  };

  createParty = async (mode: PartyMode = "duel", matchConfig?: MatchConfig) => {
    this.patch({ ...initialState, status: "admitting" });
    return this.enter("Party unavailable", (token) => createParty(token, mode, matchConfig));
  };

  leaveParty = async () => {
    const { partyId } = this.state;
    if (!partyId) return;
    this.patch({ status: "leaving", error: "" });
    try {
      await partyCommand(await this.token(), partyId, "leave", { method: "POST" }, "Could not leave party");
      this.applyLeft(partyId);
    } catch (error) {
      this.patch({ status: "error", error: errorMessage(error, "Could not leave party") });
    }
  };

  kickMember = (userId: string) => this.command("kick", { method: "POST", body: { userId } }, "Could not kick player");

  transferOwner = (userId: string) => this.command("transfer", { method: "POST", body: { userId } }, "Could not transfer leader");

  startParty = () => this.command("start", { method: "POST" }, "Could not start party");

  updateSettings = (config: MatchConfig, mode?: PartyMode) =>
    this.command("settings", { method: "PUT", body: { config, mode } }, "Could not update party settings");

  switchTeam = (teamId: PartyTeamId) => this.command("team", { method: "PUT", body: { teamId } }, "Could not switch team");

  shuffleTeams = () => this.command("shuffle", { method: "POST" }, "Could not shuffle teams");

  private async enter(failure: string, request: (token: string) => Promise<PartySnapshot>) {
    try {
      const session = await this.sessionController.getPlayableSession();
      if (!session) throw new Error("Could not start a guest session.");
      this.applySnapshot(await request(session.accessToken));
      return this.state.status === "ready";
    } catch (error) {
      this.patch({ status: "error", error: errorMessage(error, failure) });
      return false;
    }
  }

  private async command(action: string, init: Parameters<typeof partyCommand>[3], failure: string) {
    const { partyId } = this.state;
    if (!partyId) return;
    this.patch({ error: "" });
    try {
      const party = await partyCommand(await this.token(), partyId, action, init, failure);
      if (party) this.applySnapshot(party);
    } catch (error) {
      this.patch({ error: errorMessage(error, failure) });
    }
  }

  private async token() {
    const session = await this.sessionController.ensureFreshSession();
    if (!session) throw new Error("Session unavailable");
    return session.accessToken;
  }

  private userId() {
    return this.sessionController.getSessionSnapshot()?.userId ?? "";
  }

  private patch(patch: Partial<PartyRuntimeState>) {
    this.state = { ...this.state, ...patch };
    this.listeners.emit();
  }
}
