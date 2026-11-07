import type { SfxController } from "../../../lib/audio/sfx";
import { Listeners } from "../../../lib/listeners";
import type { SessionController } from "../../auth/controllers/session-controller";
import { isChatMuted } from "../lib/chat-preferences";
import { markConversationRead, readConversation, sendToConversation } from "../lib/conversation-client";
import type { ChatAudience, ChatEmote, ConversationItem } from "../model/types";

const KEPT_ITEMS = 100;

export type ChatRuntimeState = {
  conversationId: string;
  messages: ConversationItem[];
  error: string;
};

const closed: ChatRuntimeState = { conversationId: "", messages: [], error: "" };

/** One open conversation: read when opened, added to as the server tells it, written to over HTTP. */
export class ChatController {
  private readonly listeners = new Listeners();
  readonly subscribe = this.listeners.subscribe;
  private readonly sessionController: SessionController;
  private readonly sfxController?: SfxController;
  private readonly onRead?: (conversationId: string) => void;
  private state = closed;

  constructor(params: {
    sessionController: SessionController;
    sfxController?: SfxController;
    /** For conversations that track what's been read. */
    onRead?: (conversationId: string) => void;
  }) {
    this.sessionController = params.sessionController;
    this.sfxController = params.sfxController;
    this.onRead = params.onRead;
  }

  getState = () => this.state;

  open = (conversationId: string) => {
    if (conversationId === this.state.conversationId) return;
    this.patch({ ...closed, conversationId });
    void this.load();
  };

  close = () => this.patch(closed);

  /** Reads the open conversation again, after a reconnect may have missed something. */
  reload = () => {
    void this.load();
  };

  receive(item: ConversationItem) {
    if (item.conversationId !== this.state.conversationId) return;
    const known = this.state.messages.some((message) => message.id === item.id);
    this.store(item);
    if (known) return;
    if (!isChatMuted()) this.sfxController?.play("chat");
    void this.markRead();
  }

  remove(conversationId: string, itemId: string) {
    if (conversationId !== this.state.conversationId) return;
    this.patch({ messages: this.state.messages.filter((message) => message.id !== itemId) });
  }

  sendMessage = (body: string, audience: ChatAudience = "all") => {
    const text = body.trim();
    if (!text || !this.state.conversationId) return false;
    void this.send({ kind: "text", body: text, audience });
    return true;
  };

  sendEmote = (emote: ChatEmote, audience: ChatAudience = "all") => {
    if (!this.state.conversationId) return false;
    void this.send({ kind: "emote", emote, audience });
    return true;
  };

  private async send(message: Parameters<typeof sendToConversation>[2]) {
    const { conversationId } = this.state;
    try {
      const token = await this.token();
      this.receive(await sendToConversation(token, conversationId, message));
    } catch (error) {
      if (conversationId === this.state.conversationId) this.patch({ error: errorMessage(error) });
    }
  }

  private async load() {
    const { conversationId } = this.state;
    if (!conversationId) return;
    try {
      const { items } = await readConversation(await this.token(), conversationId);
      if (conversationId !== this.state.conversationId) return;
      this.patch({ messages: items.slice(-KEPT_ITEMS), error: "" });
      void this.markRead();
    } catch (error) {
      if (conversationId === this.state.conversationId) this.patch({ error: errorMessage(error) });
    }
  }

  private async markRead() {
    const { conversationId } = this.state;
    if (!this.onRead || !conversationId) return;
    this.onRead(conversationId);
    await markConversationRead(await this.token(), conversationId).catch(() => undefined);
  }

  private async token() {
    const session = await this.sessionController.ensureFreshSession();
    if (!session) throw new Error("Sign in to chat");
    return session.accessToken;
  }

  private store(item: ConversationItem) {
    const others = this.state.messages.filter((message) => message.id !== item.id);
    this.patch({ messages: [...others, item].slice(-KEPT_ITEMS) });
  }

  private patch(patch: Partial<ChatRuntimeState>) {
    this.state = { ...this.state, ...patch };
    this.listeners.emit();
  }
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : "Chat unavailable";
}
