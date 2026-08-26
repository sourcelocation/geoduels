import type { ChatAudience, ChatEmote, ChatMessage } from "../model/types";
import type { RuntimeConfig } from "../../../lib/runtime-config";
import { normalizeWSBase } from "../../../lib/runtime-config";

export type ChatEvent =
  | { type: "chat.history"; conversationId: string; messages: ChatMessage[] }
  | { type: "chat.message"; message: ChatMessage }
  | { type: "chat.error"; message: string };

export type ChatConnection = {
  sendMessage: (body: string, audience?: ChatAudience) => boolean;
  sendEmote: (emote: ChatEmote, audience?: ChatAudience) => boolean;
  close: () => void;
};

function chatWSTarget(config: RuntimeConfig, conversationId: string, accessToken: string) {
  return `${normalizeWSBase(config.queueURL).replace(/\/$/, "")}/chat/ws?conversationId=${encodeURIComponent(conversationId)}&accessToken=${encodeURIComponent(accessToken)}`;
}

export function connectChat(
  config: RuntimeConfig,
  conversationId: string,
  accessToken: string,
  onEvent: (event: ChatEvent) => void,
): ChatConnection {
  const ws = new WebSocket(chatWSTarget(config, conversationId, accessToken));
  const send = (type: string, payload: Record<string, unknown>) => {
    if (ws.readyState !== WebSocket.OPEN) return false;
    ws.send(JSON.stringify({ type, payload }));
    return true;
  };
  ws.onmessage = (evt) => {
    let msg: any;
    try {
      msg = JSON.parse(String(evt.data));
    } catch {
      onEvent({ type: "chat.error", message: "Chat connection failed" });
      return;
    }
    const payload = msg?.payload ?? {};
    if (msg?.type === "chat.history") {
      onEvent({
        type: "chat.history",
        conversationId: typeof payload.conversationId === "string" ? payload.conversationId : conversationId,
        messages: Array.isArray(payload.messages) ? payload.messages : [],
      });
      return;
    }
    if (msg?.type === "chat.message") {
      onEvent({ type: "chat.message", message: payload as ChatMessage });
      return;
    }
    if (msg?.type === "chat.error") {
      onEvent({ type: "chat.error", message: payload?.message || "Chat unavailable" });
    }
  };
  ws.onerror = () => onEvent({ type: "chat.error", message: "Chat connection failed" });
  return {
    sendMessage: (body: string, audience = "all") =>
      send("chat.send", audience === "team" ? { body, audience } : { body }),
    sendEmote: (emote: ChatEmote, audience = "all") =>
      send("chat.emote", audience === "team" ? { emote, audience } : { emote }),
    close: () => ws.close(),
  };
}
