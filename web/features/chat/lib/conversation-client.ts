import { apiFetch, authHeaders, expectJSON, mergeHeaders } from "../../../lib/http";
import type { ChatAudience, ChatEmote, ConversationItem } from "../model/types";

function path(conversationId: string, rest = "") {
  return `/api/conversations/${encodeURIComponent(conversationId)}${rest}`;
}

export async function readConversation(token: string, conversationId: string) {
  const response = await apiFetch(path(conversationId), { headers: authHeaders(token) });
  return expectJSON<{ items: ConversationItem[] }>(response, "Chat unavailable");
}

type NewMessage =
  | { kind: "text"; body: string; audience: ChatAudience }
  | { kind: "emote"; emote: ChatEmote; audience: ChatAudience };

export async function sendToConversation(token: string, conversationId: string, message: NewMessage) {
  const response = await apiFetch(path(conversationId, "/messages"), {
    method: "POST",
    headers: mergeHeaders(authHeaders(token), { "Content-Type": "application/json" }),
    body: JSON.stringify(message),
  });
  return expectJSON<ConversationItem>(response, "Message not sent");
}

export async function markConversationRead(token: string, conversationId: string) {
  await apiFetch(path(conversationId, "/read"), { method: "POST", headers: authHeaders(token) });
}
