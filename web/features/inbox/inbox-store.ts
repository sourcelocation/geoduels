import { apiFetch, authHeaders } from "../../lib/http";
import { Listeners } from "../../lib/listeners";
import type { InboxMessage } from "../live/events";

export type InboxState = { unread: number; messages: InboxMessage[] };

const EMPTY: InboxState = { unread: 0, messages: [] };

/** A person's inbox. It shows whatever it's given, and knows no kinds of message. */
export class InboxStore {
  private readonly listeners = new Listeners();
  readonly subscribe = this.listeners.subscribe;
  private state: InboxState = EMPTY;

  getState = () => this.state;

  replace(state: InboxState) {
    this.state = state;
    this.listeners.emit();
  }

  reset() {
    this.replace(EMPTY);
  }

  add(message: InboxMessage) {
    if (this.state.messages.some((item) => item.id === message.id)) return;
    this.replace({ unread: this.state.unread + 1, messages: [message, ...this.state.messages] });
  }

  /** Something was read, on any device. Id 0 is everything. */
  applyRead(id: number) {
    const readAt = new Date().toISOString();
    const messages = this.state.messages.map((message) =>
      (id === 0 || message.id === id) && !message.readAt ? { ...message, readAt } : message,
    );
    this.replace({ unread: messages.filter((message) => !message.readAt).length, messages });
  }

  async markRead(accessToken: string, id: number) {
    this.applyRead(id);
    await apiFetch(`/api/me/inbox/${id}/read`, { method: "POST", headers: authHeaders(accessToken) });
  }

  async markAllRead(accessToken: string) {
    this.applyRead(0);
    await apiFetch("/api/me/inbox/read-all", { method: "POST", headers: authHeaders(accessToken) });
  }
}

export const inboxStore = new InboxStore();
