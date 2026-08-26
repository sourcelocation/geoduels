export type ChatEmote = "skull" | "sob" | "thinking" | "sunglasses" | "wave";
export type ChatAudience = "all" | "team";

export type ChatMessage = {
  id: string;
  conversationId?: string;
  matchId: string;
  senderUserId: string;
  senderDisplayName: string;
  kind: "text" | "emote";
  audience?: ChatAudience;
  teamId?: string;
  body?: string;
  emote?: ChatEmote;
  createdAt: string;
};
