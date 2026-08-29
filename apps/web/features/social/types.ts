import type { PlayerBadgeInfo } from "../players/components/PlayerBadge";

export type RelationshipState =
  | "none"
  | "incoming_request"
  | "outgoing_request"
  | "friends"
  | "blocked_by_viewer";

export type CompactPlayer = {
  userId: string;
  displayName: string;
  avatarUrl?: string;
  mmr?: number;
  selectedBadge?: PlayerBadgeInfo | null;
  relationship: RelationshipState;
  requestId?: string;
  presenceStatus?: "online" | "away" | "offline";
  activity?: "in_match" | "in_party";
  lastSeenAt?: string;
  sharedMatchAt?: string;
  partyInvite?: PartyInviteStatus;
};

export type PartyInviteStatus = {
  id: string;
  createdAt: string;
  expiresAt: string;
};

export type FriendRequest = {
  id: string;
  direction: "incoming" | "outgoing";
  player: CompactPlayer;
  createdAt: string;
  expiresAt: string;
};

export type PartyInvitation = {
  id: string;
  partyId: string;
  inviteCode?: string;
  mode: string;
  memberCount: number;
  inviter: CompactPlayer;
  createdAt: string;
  expiresAt: string;
};

export type SocialNotification = {
  id: number;
  type: string;
  payload: Record<string, unknown>;
  createdAt: string;
};

export type SocialSummary = {
  incomingRequests: FriendRequest[];
  outgoingRequests: FriendRequest[];
  partyInvitations: PartyInvitation[];
  notifications: SocialNotification[];
  unreadCount: number;
};

export type SocialSettings = {
  discoverable: boolean;
  presenceVisible: boolean;
  requestsEnabled: boolean;
  partyInvitesEnabled: boolean;
};
