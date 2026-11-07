import { useEffect, useSyncExternalStore } from "react";
import AppModalShell from "../../../components/ui/AppModalShell";
import { useAppNotice } from "../../../components/ui/AppNotice";
import { Button } from "../../../components/ui/button";
import { useAuthState } from "../../auth/components/AuthProvider";
import { emoteGlyph } from "../../chat/lib/emotes";
import { useItemAction } from "../../chat/hooks/use-item-action";
import { homeRuntime } from "../../home/state/home-runtime";
import { inboxStore } from "../../inbox/inbox-store";
import { liveConnection } from "../../live/live-connection";

/**
 * Tells people what just happened, whatever it was: the server words every message and says what can
 * be done about it. Nothing covers a match or its results; what arrives meanwhile is in the inbox or
 * chat, and a message that must be seen waits as a modal until the player is back.
 */
export function Notifications() {
  const { matchController, directChat } = homeRuntime;
  const { toast } = useAppNotice();
  const runAction = useItemAction();
  const { accessToken } = useAuthState();
  const inbox = useSyncExternalStore(inboxStore.subscribe, inboxStore.getState, inboxStore.getState);
  const match = useSyncExternalStore(matchController.subscribe, matchController.getState.bind(matchController), matchController.getState.bind(matchController));

  useEffect(() => {
    const inMatch = () => !!matchController.getState().snapshot;
    const stopInbox = liveConnection.on("inbox.added", ({ message }) => {
      if (inMatch() || message.modal) return;
      toast({
        title: message.title,
        body: message.body,
        icon: message.imageUrl ? <img src={message.imageUrl} alt="" className="h-8 w-8 object-contain" /> : undefined,
      });
    });
    const stopItems = liveConnection.on("conversation.item", ({ item, notify }) => {
      if (!notify || inMatch() || directChat.getState().conversationId === item.conversationId) return;
      const action = item.actions?.[0];
      toast({
        title: item.senderDisplayName,
        body: item.body || emoteGlyph(item.emote),
        action: action
          ? { label: action.label, onClick: () => void runAction(action) }
          : { label: "Open", onClick: () => directChat.open(item.conversationId) },
      });
    });
    return () => {
      stopInbox();
      stopItems();
    };
  }, [directChat, matchController, runAction, toast]);

  // Oldest first, one at a time. Closing one reads it, on every device.
  const modal = match.snapshot ? undefined : inbox.messages.filter((message) => message.modal && !message.readAt).at(-1);
  if (!modal) return null;
  return (
    <AppModalShell
      key={modal.id}
      title={modal.title}
      placement="center"
      showHeader={false}
      zIndexClassName="z-modal"
      maxWidthClassName="max-w-sm"
      onClose={() => void inboxStore.markRead(accessToken, modal.id)}
    >
      {modal.imageUrl ? <img src={modal.imageUrl} alt="" className="mx-auto mb-3 h-20 w-20 object-contain" /> : null}
      <p className="text-heading-sm font-strong text-content-primary">{modal.title}</p>
      {modal.body ? <p className="mt-2 text-body-sm text-content-secondary">{modal.body}</p> : null}
      <Button type="button" variant="primary" size="lg" onClick={() => void inboxStore.markRead(accessToken, modal.id)} className="mt-5 w-full">
        OK
      </Button>
    </AppModalShell>
  );
}
