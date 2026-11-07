import { useSyncExternalStore } from "react";
import { Button } from "../../../components/ui/button";
import { AppPanel } from "../../../components/ui/compositions";
import { Eyebrow } from "../../../components/ui/typography";
import { cn } from "../../../lib/cn";
import { useAuthState } from "../../auth/components/AuthProvider";
import { inboxStore } from "../inbox-store";

/** What the server has told this person. It holds no actions: anything to answer is in chat. */
export function InboxCard() {
  const inbox = useSyncExternalStore(inboxStore.subscribe, inboxStore.getState, inboxStore.getState);
  const { accessToken } = useAuthState();
  return (
    <AppPanel className="flex h-full min-w-0 flex-col rounded-2xl p-5 sm:p-6">
      <header className="flex items-center justify-between gap-3">
        <Eyebrow>Inbox{inbox.unread ? ` · ${inbox.unread} new` : ""}</Eyebrow>
        {inbox.unread ? (
          <Button type="button" variant="ghost" size="sm" onClick={() => void inboxStore.markAllRead(accessToken)}>
            Mark all read
          </Button>
        ) : null}
      </header>
      {inbox.messages.length ? (
        <ul className="scrollbar-hidden mt-3 flex max-h-64 flex-col gap-1 overflow-y-auto">
          {inbox.messages.map((message) => (
            <li key={message.id}>
              <Button
                type="button"
                variant="ghost"
                disabled={!!message.readAt}
                onClick={() => void inboxStore.markRead(accessToken, message.id)}
                className={cn(
                  "h-auto w-full items-start justify-start gap-3 whitespace-normal rounded-lg px-2 py-2 text-left disabled:opacity-100",
                  !message.readAt && "bg-surface-fill hover:bg-surface-raised",
                )}
              >
                {message.imageUrl ? <img src={message.imageUrl} alt="" className="h-9 w-9 flex-shrink-0 rounded-lg object-contain" /> : null}
                <span className="min-w-0">
                  <span className="block truncate text-body-sm font-strong text-content-primary">{message.title}</span>
                  {message.body ? <span className="block text-label text-content-secondary">{message.body}</span> : null}
                  <span className="block text-caption text-content-secondary">{new Date(message.createdAt).toLocaleDateString()}</span>
                </span>
              </Button>
            </li>
          ))}
        </ul>
      ) : (
        <p className="mt-3 flex-1 text-body-sm font-semibold text-content-secondary">Nothing here yet.</p>
      )}
    </AppPanel>
  );
}
