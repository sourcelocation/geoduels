import { Send, X } from "lucide-react";
import { useEffect, useRef, useState, useSyncExternalStore, type FormEvent } from "react";
import { Button } from "../../../components/ui/button";
import { AppPanel } from "../../../components/ui/compositions";
import { Input } from "../../../components/ui/input";
import { useAuthState } from "../../auth/components/AuthProvider";
import { homeRuntime } from "../../home/state/home-runtime";
import { socialStore } from "../../social/social-store";
import { emoteGlyph } from "../lib/emotes";
import { ItemActions } from "./ItemActions";

const MAX_MESSAGE = 180;

/** The conversation open with one other person, on any page. */
export function DirectChat() {
  const { directChat, matchController } = homeRuntime;
  const match = useSyncExternalStore(matchController.subscribe, matchController.getState.bind(matchController), matchController.getState.bind(matchController));
  const chat = useSyncExternalStore(directChat.subscribe, directChat.getState, directChat.getState);
  const social = useSyncExternalStore(socialStore.subscribe, socialStore.getState, socialStore.getState);
  const auth = useAuthState();
  const [body, setBody] = useState("");
  const scrollRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const node = scrollRef.current;
    if (node) node.scrollTop = node.scrollHeight;
  }, [chat.messages]);

  // Nothing covers a match. It's still open when the player is back.
  if (!chat.conversationId || match.snapshot) return null;
  const friend = social.friends.find((player) => player.conversationId === chat.conversationId);
  const pending = social.pending.find((entry) => entry.item.conversationId === chat.conversationId);
  const title = friend?.displayName || pending?.player.displayName || "Chat";

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (directChat.sendMessage(body)) setBody("");
  };

  return (
    <AppPanel className="app-layer-chat fixed inset-x-4 bottom-24 flex flex-col sm:left-auto sm:w-80 md:bottom-4 gap-3 rounded-2xl p-4">
      <header className="flex items-center justify-between gap-3">
        <p className="min-w-0 truncate text-body-sm font-strong text-content-primary">{title}</p>
        <Button type="button" variant="ghost" size="icon" aria-label="Close chat" onClick={directChat.close}>
          <X size={15} strokeWidth={2.5} />
        </Button>
      </header>
      <div ref={scrollRef} className="scrollbar-hidden flex max-h-72 min-h-24 flex-col gap-2 overflow-y-auto text-body-sm font-semibold">
        {chat.messages.length === 0 ? (
          <p className="py-6 text-center text-content-secondary">No messages yet</p>
        ) : (
          chat.messages.map((item) => (
            <div key={item.id} className={item.senderUserId === auth.userId ? "self-end text-right" : "self-start"}>
              <p className="break-words text-content-primary">{item.body || emoteGlyph(item.emote)}</p>
              {item.actions?.length ? (
                <div className="mt-1 flex flex-wrap gap-2">
                  <ItemActions actions={item.actions} />
                </div>
              ) : null}
            </div>
          ))
        )}
      </div>
      {chat.error ? <p className="text-label text-status-danger">{chat.error}</p> : null}
      <form onSubmit={submit} className="flex gap-2">
        <Input
          value={body}
          onChange={(event) => setBody(event.target.value.slice(0, MAX_MESSAGE))}
          maxLength={MAX_MESSAGE}
          placeholder="Message"
          aria-label="Message"
          className="min-w-0 flex-1"
        />
        <Button type="submit" variant="primary" size="icon" aria-label="Send message" disabled={!body.trim()}>
          <Send size={16} strokeWidth={2.5} />
        </Button>
      </form>
    </AppPanel>
  );
}
