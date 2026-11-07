export type ChatEmote = "skull" | "sob" | "thinking" | "sunglasses" | "wave";
export type ChatAudience = "all" | "team";

/** A button on an item. The server decides what it says and does: post, then open. */
export type ItemAction = { label: string; post?: string; open?: string };

/** One thing in a conversation: a message, or a request waiting for an answer. */
export type ConversationItem = {
  id: string;
  conversationId: string;
  kind: string;
  senderUserId: string;
  senderDisplayName: string;
  senderTeamId?: string;
  matchId?: string;
  audience?: ChatAudience;
  teamId?: string;
  body?: string;
  emote?: ChatEmote;
  createdAt: string;
  actions?: ItemAction[];
};
