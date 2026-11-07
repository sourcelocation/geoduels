import { apiSocketURL } from "../../lib/http";
import { eventChecks, type LiveEvents } from "./events";

const MAX_BACKOFF_MS = 15_000;

/**
 * The one connection to the server for everything social. It only receives: actions are HTTP calls.
 * Every connection starts with a full session, so reconnecting is all it ever takes to catch up.
 */
export class LiveConnection {
  private socket: WebSocket | null = null;
  private retryTimer = 0;
  private attempt = 0;
  private wanted = false;
  private readonly receivers = new Map<string, Set<(data: unknown) => void>>();

  /** Subscribes to one kind of event. Nothing here needs to know what any of them mean. */
  on<K extends keyof LiveEvents>(type: K, handler: (data: LiveEvents[K]) => void) {
    const check = eventChecks[type];
    const receiver = (data: unknown) => {
      if (check(data)) handler(data);
    };
    const receivers = this.receivers.get(type) ?? new Set();
    this.receivers.set(type, receivers);
    receivers.add(receiver);
    return () => {
      receivers.delete(receiver);
    };
  }

  /** Connects, replacing any connection: the session cookie may have changed. */
  connect() {
    this.wanted = true;
    this.close();
    this.attempt = 0;
    this.open();
  }

  disconnect() {
    this.wanted = false;
    this.close();
  }

  private close() {
    window.clearTimeout(this.retryTimer);
    const socket = this.socket;
    this.socket = null;
    socket?.close();
  }

  private open() {
    const socket = new WebSocket(apiSocketURL("/api/live"));
    this.socket = socket;
    socket.onopen = () => {
      this.attempt = 0;
    };
    socket.onmessage = (message) => {
      if (this.socket === socket) this.receive(String(message.data));
    };
    socket.onclose = () => {
      if (this.socket !== socket || !this.wanted) return;
      this.socket = null;
      // Random, so everyone a restart disconnected doesn't come back at once.
      const delay = Math.min(MAX_BACKOFF_MS, 500 * 2 ** this.attempt) * (0.5 + Math.random());
      this.attempt += 1;
      this.retryTimer = window.setTimeout(() => this.open(), delay);
    };
  }

  private receive(raw: string) {
    let envelope: unknown;
    try {
      envelope = JSON.parse(raw);
    } catch {
      return;
    }
    if (typeof envelope !== "object" || envelope === null || !("type" in envelope) || !("data" in envelope)) return;
    if (typeof envelope.type !== "string") return;
    this.receivers.get(envelope.type)?.forEach((receive) => receive(envelope.data));
  }
}

export const liveConnection = new LiveConnection();
