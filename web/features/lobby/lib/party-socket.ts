import type { RuntimeConfig } from "../../../lib/runtime-config";
import { apiSocketURL } from "../../../lib/http";
import type { MatchConfig } from "../../matchmaking/lib/queue-client";
import type { PartyEvent, PartyMode, PartyTeamId } from "./party-client";

type PartyCommandPayloads = {
  team: { teamId: PartyTeamId };
  shuffle_teams: Record<string, never>;
  settings: { config: MatchConfig; mode?: PartyMode };
  kick: { userId: string };
  transfer_owner: { userId: string };
  start: Record<string, never>;
  leave: Record<string, never>;
};

type PendingCommand = {
  resolve: () => void;
  reject: (error: Error) => void;
  timeout: ReturnType<typeof setTimeout>;
};

export class PartySocket {
  readonly closed: Promise<void>;
  private readonly ws: WebSocket;
  private readonly pending = new Map<string, PendingCommand>();
  private ready = false;
  private sequence = 0;

  constructor(config: RuntimeConfig, partyId: string, accessToken: string, signal: AbortSignal, onEvent: (event: PartyEvent) => void) {
    this.ws = new WebSocket(`${apiSocketURL(config, `/api/v2/parties/${encodeURIComponent(partyId)}/ws`)}?accessToken=${encodeURIComponent(accessToken)}`);
    this.closed = new Promise<void>((resolve, reject) => {
      let settled = false;
      const snapshotTimeout = setTimeout(() => finish(new Error("Party connection timed out")), 10000);
      const finish = (error?: Error) => {
        if (settled) return;
        settled = true;
        this.ready = false;
        clearTimeout(snapshotTimeout);
        signal.removeEventListener("abort", abort);
        for (const pending of this.pending.values()) {
          clearTimeout(pending.timeout);
          pending.reject(error || new Error("Party connection closed; action outcome may be unknown"));
        }
        this.pending.clear();
        this.ws.close();
        if (error) reject(error);
        else resolve();
      };
      const abort = () => finish(new DOMException("Aborted", "AbortError"));
      signal.addEventListener("abort", abort, { once: true });
      this.ws.onerror = () => finish(new Error("Party connection failed"));
      this.ws.onclose = () => finish();
      this.ws.onmessage = (event) => {
        if (settled) return;
        try {
          const message = JSON.parse(String(event.data));
          const payload = message.payload ?? {};
          switch (message.type) {
            case "party_command_result": {
              const pending = this.pending.get(payload.requestId);
              if (!pending) return;
              clearTimeout(pending.timeout);
              this.pending.delete(payload.requestId);
              if (payload.ok) pending.resolve();
              else pending.reject(new Error(payload.error || "Party action failed"));
              break;
            }
            case "party_snapshot":
              clearTimeout(snapshotTimeout);
              this.ready = true;
              onEvent({ type: "party_snapshot", party: payload });
              break;
            case "party_patch":
              onEvent({ type: "party_patch", patch: payload });
              break;
            case "match_found":
              if (typeof payload.matchId === "string" && payload.matchId) {
                onEvent({ type: "match_found", matchId: payload.matchId });
              }
              break;
            case "party_error":
              this.ready = false;
              onEvent({ type: "party_error", message: payload.message || "Party unavailable" });
              break;
          }
        } catch {
          finish(new Error("Invalid party message"));
        }
      };
      if (signal.aborted) abort();
    });
  }

  command<K extends keyof PartyCommandPayloads>(type: K, payload: PartyCommandPayloads[K]): Promise<void> {
    if (!this.ready || this.ws.readyState !== WebSocket.OPEN) {
      return Promise.reject(new Error("Party is reconnecting; try again when connected"));
    }
    const requestId = String(++this.sequence);
    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        this.pending.delete(requestId);
        reject(new Error("Party action timed out; reconnecting to refresh its outcome"));
        this.ready = false;
        this.ws.close();
      }, 25000);
      this.pending.set(requestId, { resolve, reject, timeout });
      try {
        this.ws.send(JSON.stringify({ requestId, type, payload }));
      } catch {
        clearTimeout(timeout);
        this.pending.delete(requestId);
        reject(new Error("Could not send party action"));
      }
    });
  }
}
