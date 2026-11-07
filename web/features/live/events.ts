import type { AppBootstrapPayload } from "../auth/lib/auth-client";
import type { ConversationItem } from "../chat/model/types";
import type { PartySnapshot } from "../lobby/lib/party-client";
import type { CompactPlayer, FriendRequest } from "../social/types";

// What the server tells a person. Each event is checked where it arrives, so everything past this
// point can trust its type; tests/shared/live-events.json holds an example of each, which the server's
// tests write and these parsers read, so the two sides can't drift apart.

export type Friend = CompactPlayer & { conversationId: string; unread?: number; lastMessageAt?: string };

export type PendingItem = { item: ConversationItem; player: CompactPlayer };

export type SocialState = { friends: Friend[]; outgoing: FriendRequest[]; pending: PendingItem[] };

export type InboxMessage = {
  id: number;
  kind: string;
  title: string;
  body?: string;
  imageUrl?: string;
  modal?: boolean;
  readAt?: string;
  createdAt: string;
};

export type QueueState = { queued: boolean; queuedAt?: number; error?: string };

export type ActiveMatch = { matchId: string; kind: string; mode: string; status: string };

export type StatusState = AppBootstrapPayload["global"];

export type SessionState = {
  userId?: string;
  social: SocialState;
  inbox: { unread: number; messages: InboxMessage[] };
  party?: PartySnapshot;
  queue: QueueState;
  activeMatch?: ActiveMatch;
  status: StatusState;
};

export type LiveEvents = {
  session: SessionState;
  status: StatusState;
  "social.updated": SocialState;
  "friend.presence": { userId: string; presenceStatus: "online" | "offline"; activity?: "in_match" };
  "inbox.added": { message: InboxMessage };
  "inbox.read": { id: number };
  "conversation.item": { item: ConversationItem; notify?: boolean };
  "conversation.item_removed": { conversationId: string; itemId: string };
  "party.updated": { party: PartySnapshot };
  "party.left": { partyId: string; reason: string };
  "queue.updated": QueueState;
  "match.started": { match: ActiveMatch };
};

type Fields = Record<string, unknown>;

function isFields(value: unknown): value is Fields {
  return typeof value === "object" && value !== null;
}

function hasStrings(value: Fields, ...names: string[]) {
  return names.every((name) => typeof value[name] === "string");
}

function isList<T>(value: unknown, item: (entry: unknown) => entry is T): value is T[] {
  return Array.isArray(value) && value.every(item);
}

function isPlayer(value: unknown): value is CompactPlayer {
  return isFields(value) && hasStrings(value, "userId", "displayName");
}

function isFriend(value: unknown): value is Friend {
  return isPlayer(value) && isFields(value) && hasStrings(value, "conversationId");
}

function isFriendRequest(value: unknown): value is FriendRequest {
  return isFields(value) && hasStrings(value, "id", "createdAt") && isPlayer(value.player);
}

export function isItem(value: unknown): value is ConversationItem {
  return isFields(value) && hasStrings(value, "id", "conversationId", "kind", "senderUserId", "createdAt");
}

function isPending(value: unknown): value is PendingItem {
  return isFields(value) && isItem(value.item) && isPlayer(value.player);
}

function isSocial(value: unknown): value is SocialState {
  return isFields(value) && isList(value.friends, isFriend) && isList(value.outgoing, isFriendRequest) && isList(value.pending, isPending);
}

function isInboxMessage(value: unknown): value is InboxMessage {
  return isFields(value) && typeof value.id === "number" && hasStrings(value, "kind", "title", "createdAt");
}

export function isParty(value: unknown): value is PartySnapshot {
  return isFields(value) && hasStrings(value, "id", "inviteCode", "state") && typeof value.revision === "number" && Array.isArray(value.members);
}

function isQueue(value: unknown): value is QueueState {
  return isFields(value) && (value.queued === undefined || typeof value.queued === "boolean");
}

function isActiveMatch(value: unknown): value is ActiveMatch {
  return isFields(value) && hasStrings(value, "matchId");
}

function isStatus(value: unknown): value is StatusState {
  return isFields(value) && typeof value.onlinePlayers === "number" && isFields(value.maintenance);
}

function isSession(value: unknown): value is SessionState {
  return (
    isFields(value) &&
    isSocial(value.social) &&
    isFields(value.inbox) &&
    typeof value.inbox.unread === "number" &&
    isList(value.inbox.messages, isInboxMessage) &&
    (value.party === undefined || isParty(value.party)) &&
    isQueue(value.queue) &&
    isStatus(value.status)
  );
}

/** One check per event: a message that doesn't match is dropped, not half-believed. */
export const eventChecks: { [K in keyof LiveEvents]: (data: unknown) => data is LiveEvents[K] } = {
  session: isSession,
  status: isStatus,
  "social.updated": isSocial,
  "friend.presence": (data): data is LiveEvents["friend.presence"] => isFields(data) && hasStrings(data, "userId", "presenceStatus"),
  "inbox.added": (data): data is LiveEvents["inbox.added"] => isFields(data) && isInboxMessage(data.message),
  "inbox.read": (data): data is LiveEvents["inbox.read"] => isFields(data) && typeof data.id === "number",
  "conversation.item": (data): data is LiveEvents["conversation.item"] => isFields(data) && isItem(data.item),
  "conversation.item_removed": (data): data is LiveEvents["conversation.item_removed"] => isFields(data) && hasStrings(data, "conversationId", "itemId"),
  "party.updated": (data): data is LiveEvents["party.updated"] => isFields(data) && isParty(data.party),
  "party.left": (data): data is LiveEvents["party.left"] => isFields(data) && hasStrings(data, "partyId", "reason"),
  "queue.updated": isQueue,
  "match.started": (data): data is LiveEvents["match.started"] => isFields(data) && isActiveMatch(data.match),
};
