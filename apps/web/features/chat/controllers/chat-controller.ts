import type { ChatAudience, ChatEmote, ChatMessage } from "../model/types";
import type { SfxController } from "../../../lib/audio/sfx";
import { ObservableStore } from "../../../lib/observable-store";
import type { RuntimeConfig } from "../../../lib/runtime-config";
import {
  connectChat,
  type ChatConnection,
} from "../lib/chat-client";
import { isChatMuted } from "../lib/chat-preferences";

export type ChatRuntimeState = {
  conversationId: string;
  messages: ChatMessage[];
  error: string;
};

const initialState: ChatRuntimeState = {
  conversationId: "",
  messages: [],
  error: "",
};

export class ChatController extends ObservableStore<ChatRuntimeState> {
  private readonly config: RuntimeConfig;
  private readonly sfxController?: SfxController;
  private state: ChatRuntimeState = initialState;
  private connection: ChatConnection | null = null;
  private connectionKey = "";
  private messagesByConversation = new Map<string, ChatMessage[]>();
  private destroyed = false;

  constructor(params: {
    config: RuntimeConfig;
    sfxController?: SfxController;
  }) {
    super();
    this.config = params.config;
    this.sfxController = params.sfxController;
  }

  getState() {
    return this.state;
  }

  destroy() {
    this.destroyed = true;
    this.closeConnection();
  }

  reset = () => {
    this.closeConnection();
    this.connectionKey = "";
    this.messagesByConversation.clear();
    this.patchState(initialState);
  };

  setConversation = (conversationId: string, accessToken: string) => {
    const nextConversationId = conversationId.trim();
    const nextConnectionKey =
      nextConversationId && accessToken
        ? `${nextConversationId}:${accessToken}`
        : "";

    if (!nextConversationId || !accessToken) {
      this.closeConnection();
      this.connectionKey = "";
      this.patchState({
        conversationId: "",
        messages: [],
        error: "",
      });
      return;
    }

    const cachedMessages =
      this.messagesByConversation.get(nextConversationId) || [];
    const conversationChanged = this.state.conversationId !== nextConversationId;
    if (conversationChanged) {
      this.patchState({
        conversationId: nextConversationId,
        messages: cachedMessages,
        error: "",
      });
    }

    if (this.connectionKey === nextConnectionKey && this.connection) {
      return;
    }

    this.closeConnection();
    this.connectionKey = nextConnectionKey;
    const connection = connectChat(
      this.config,
      nextConversationId,
      accessToken,
      (event) => {
        if (this.connection !== connection) return;
        if (event.type === "chat.history") {
          if (event.conversationId !== nextConversationId) return;
          this.storeMessages(nextConversationId, event.messages);
          return;
        }
        if (event.type === "chat.message") {
          if (
            event.message.conversationId &&
            event.message.conversationId !== nextConversationId
          ) {
            return;
          }
          const current = this.messagesByConversation.get(nextConversationId) || [];
          this.storeMessages(
            nextConversationId,
            [
              ...current.filter((item) => item.id !== event.message.id),
              event.message,
            ].slice(-100),
          );
          if (!isChatMuted()) {
            this.sfxController?.play("chat");
          }
          return;
        }
        if (event.type === "chat.error") {
          this.patchState({ error: event.message });
        }
      },
    );
    this.connection = connection;
  };

  sendMessage = (body: string, audience: ChatAudience = "all") => {
    const trimmed = body.trim();
    if (!trimmed || !this.state.conversationId) return false;
    return this.connection?.sendMessage(trimmed, audience) || false;
  };

  sendEmote = (emote: ChatEmote, audience: ChatAudience = "all") => {
    if (!this.state.conversationId) return false;
    return this.connection?.sendEmote(emote, audience) || false;
  };

  private storeMessages(conversationId: string, messages: ChatMessage[]) {
    const next = messages.slice(-100);
    this.messagesByConversation.set(conversationId, next);
    if (this.state.conversationId === conversationId) {
      this.patchState({ messages: next });
    }
  }

  private closeConnection() {
    this.connection?.close();
    this.connection = null;
  }

  private patchState(patch: Partial<ChatRuntimeState>) {
    this.state = { ...this.state, ...patch };
    if (!this.destroyed) {
      this.emit();
    }
  }
}
