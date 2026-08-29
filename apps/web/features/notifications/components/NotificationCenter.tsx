import { Bell } from "lucide-react";
import { useState } from "react";
import { Notice } from "../../../components/ui/patterns";
import { EmptyState } from "../../../components/ui/EmptyState";
import { CounterBadge } from "../../../components/ui/Badge";
import { ButtonLink } from "../../../components/ui/button";
import { AppChromeIconButton } from "../../../components/ui/compositions";
import { Popover } from "../../../components/ui/Popover";
import { ScrollArea } from "../../../components/ui/ScrollArea";
import { useAuthState } from "../../auth/components/AuthProvider";
import { NotificationGroup, NotificationItem, notificationDetails, notificationEntriesFromSummary } from "./NotificationItem";
import { useNotificationActions } from "../hooks/useNotificationActions";

export function NotificationCenter() {
  const auth = useAuthState();
  if (!auth.isRegistered) return null;
  return <ConnectedNotificationCenter session={auth} />;
}

function ConnectedNotificationCenter({
  session,
}: {
  session: ReturnType<typeof useAuthState>;
}) {
  const [open, setOpen] = useState(false);
  const action = useNotificationActions(session.accessToken);
  const notifications = session.bootstrap?.activity.notifications || [];
  const data = {
    incomingRequests: [],
    outgoingRequests: [],
    partyInvitations: [],
    notifications,
    unreadCount: notifications.filter((notification) => !notification.readAt).length,
  };
  const entries = notificationEntriesFromSummary(data);
  return (
    <Popover open={open} onOpenChange={setOpen} className="w-[min(92vw,25rem)] p-0" content={
      <div>
        <div className="flex items-center justify-between border-b border-border-default px-4 py-3">
          <h2 className="text-heading-sm font-strong text-content-primary">Notifications</h2>
          <ButtonLink href="/notifications" variant="ghost" size="sm" onClick={() => setOpen(false)}>View all</ButtonLink>
        </div>
        <ScrollArea className="max-h-[70vh] p-3">
          <div className="grid gap-4">
            {action.isError ? <Notice tone="danger">Could not update the request. Try again.</Notice> : null}
            {entries.length ? (
              <NotificationGroup title="Recent activity">
                {entries.map((entry) => (
                  <NotificationItem key={notificationDetailsForKey(entry)} compact entry={entry} onOpen={() => setOpen(false)} onAction={(nextAction) => action.mutate(nextAction)} actionPending={action.isPending} />
                ))}
              </NotificationGroup>
            ) : null}
            {!data.notifications.length ? (
              <EmptyState title="You’re all caught up" message="New requests and activity will appear here." />
            ) : null}
          </div>
        </ScrollArea>
      </div>
    }>
      <AppChromeIconButton aria-label="Notifications" aria-expanded={open} className="relative">
        <Bell size={18} />
        <CounterBadge count={data?.unreadCount || 0} className="pointer-events-none absolute -right-2 -top-1" />
      </AppChromeIconButton>
    </Popover>
  );
}

function notificationDetailsForKey(entry: Parameters<typeof NotificationItem>[0]["entry"]) {
  if (entry.kind === "friend_request") return `friend:${entry.request.id}`;
  if (entry.kind === "party_invitation") return `party:${entry.invitation.id}`;
  return `notification:${entry.notification.id}`;
}

export function notificationTitle(type: string) {
  return notificationDetails({ id: 0, type, payload: {}, createdAt: "" }).title;
}
