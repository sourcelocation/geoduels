import { Listeners } from "../../lib/listeners";
import type { LiveEvents, SocialState } from "../live/events";

export type SocialView = SocialState & { loaded: boolean };

const EMPTY: SocialView = { friends: [], outgoing: [], pending: [], loaded: false };

/** A person's friends as the server last told it: the whole list, then updates to it. */
export class SocialStore {
  private readonly listeners = new Listeners();
  readonly subscribe = this.listeners.subscribe;
  private state = EMPTY;

  getState = () => this.state;

  replace(state: SocialState) {
    this.set({ ...state, loaded: true });
  }

  applyPresence({ userId, presenceStatus, activity }: LiveEvents["friend.presence"]) {
    this.mapFriends((friend) => (friend.userId === userId ? { ...friend, presenceStatus, activity } : friend));
  }

  markRead(conversationId: string) {
    this.mapFriends((friend) => (friend.conversationId === conversationId ? { ...friend, unread: 0 } : friend));
  }

  /** A message arrived in a conversation that isn't open. */
  addUnread(conversationId: string) {
    this.mapFriends((friend) => (friend.conversationId === conversationId ? { ...friend, unread: (friend.unread || 0) + 1 } : friend));
  }

  private mapFriends(update: (friend: SocialState["friends"][number]) => SocialState["friends"][number]) {
    this.set({ ...this.state, friends: this.state.friends.map(update) });
  }

  private set(state: SocialView) {
    this.state = state;
    this.listeners.emit();
  }
}

export const socialStore = new SocialStore();
