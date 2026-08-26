import { Check, Gift, ShieldAlert, Trophy, Users, X } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";
import { Badge } from "../../../components/ui/Badge";
import { IconButton } from "../../../components/ui/button";
import { EntityRow, InsetList } from "../../../components/ui/patterns";
import { RelativeTime } from "../../../components/ui/RelativeTime";
import { cn } from "../../../lib/cn";
import type { FriendRequest, PartyInvitation, SocialSummary } from "../../social/types";

export type NotificationRecord = { id: number; type: string; payload: Record<string, unknown>; createdAt: string; readAt?: string };
export type NotificationFeedEntry =
  | { kind: "notification"; notification: NotificationRecord }
  | { kind: "friend_request"; request: FriendRequest }
  | { kind: "party_invitation"; invitation: PartyInvitation };
export type NotificationAction = { kind: "friend" | "party"; id: string; value: "accept" | "decline" | "cancel" };
type NotificationPresentation = { key: string; title: string; body?: string; href: string; icon: ReactNode; createdAt: string; unread: boolean; actions: NotificationAction[] };

function payloadString(payload: Record<string, unknown>, key: string) {
  return typeof payload[key] === "string" ? payload[key] : undefined;
}

export function parseNotificationEntry(entry: NotificationFeedEntry): NotificationPresentation {
  if (entry.kind === "friend_request") {
    const { request } = entry;
    const incoming = request.direction === "incoming";
    return {
      key: `friend-request:${request.id}`,
      title: incoming ? `${request.player.displayName} sent a friend request` : `Friend request sent to ${request.player.displayName}`,
      body: incoming ? "Accept to add them to your friends list." : "Waiting for a response.",
      href: `/players/${encodeURIComponent(request.player.displayName || request.player.userId)}`,
      icon: <Users size={16} />,
      createdAt: request.createdAt,
      unread: incoming,
      actions: incoming
        ? [{ kind: "friend", id: request.id, value: "accept" }, { kind: "friend", id: request.id, value: "decline" }]
        : [{ kind: "friend", id: request.id, value: "cancel" }],
    };
  }
  if (entry.kind === "party_invitation") {
    const { invitation } = entry;
    return {
      key: `party-invitation:${invitation.id}`,
      title: `${invitation.inviter.displayName} invited you to a party`,
      body: `${invitation.mode.replaceAll("_", " ")} · ${invitation.memberCount} players`,
      href: "/notifications",
      icon: <Users size={16} />,
      createdAt: invitation.createdAt,
      unread: true,
      actions: [{ kind: "party", id: invitation.id, value: "accept" }, { kind: "party", id: invitation.id, value: "decline" }],
    };
  }

  const { notification } = entry;
  const matchId = payloadString(notification.payload, "matchId");
  const requestId = payloadString(notification.payload, "requestId");
  const invitationId = payloadString(notification.payload, "invitationId");
  const reason = payloadString(notification.payload, "reason");
  const badge = notification.payload.badge as { label?: string; description?: string } | undefined;
  const base = { key: `notification:${notification.id}`, createdAt: notification.createdAt, unread: !notification.readAt };
  switch (notification.type) {
    case "friend_request_received":
      return { ...base, title: "New friend request", body: "A player wants to add you.", href: "/notifications", icon: <Users size={16} />, actions: requestId ? [{ kind: "friend", id: requestId, value: "accept" }, { kind: "friend", id: requestId, value: "decline" }] : [] };
    case "friendship_accepted":
      return { ...base, title: "Friend request accepted", body: "You're now friends. ", href: "/notifications", icon: <Users size={16} />, actions: [] };
    case "party_invitation_received":
      return { ...base, title: "New party invitation", body: "Join your friends for the next duel.", href: "/notifications", icon: <Users size={16} />, actions: invitationId ? [{ kind: "party", id: invitationId, value: "accept" }, { kind: "party", id: invitationId, value: "decline" }] : [] };
    case "badge_unlocked":
      return { ...base, title: badge?.label ? `${badge.label} unlocked` : "Badge unlocked", body: badge?.description, href: "/notifications", icon: <Trophy size={16} />, actions: [] };
    case "mmr_refund":
      return { ...base, title: "Rating refunded", body: "Your rating was adjusted after a match review.", href: matchId ? `/match/${encodeURIComponent(matchId)}` : "/notifications", icon: <ShieldAlert size={16} />, actions: [] };
    case "account_banned":
      return { ...base, title: "Account suspended", body: reason ? `Reason: ${reason}` : "Your account access has been restricted.", href: "/notifications", icon: <ShieldAlert size={16} />, actions: [] };
    case "account_unbanned":
      return { ...base, title: "Account restriction removed", body: reason || "Your account has been restored.", href: "/notifications", icon: <ShieldAlert size={16} />, actions: [] };
    case "reported_player_banned":
      return { ...base, title: "Report action taken", body: "A player you reported was suspended after review. Thank you for helping keep GeoDuels fair.", href: "/notifications", icon: <ShieldAlert size={16} />, actions: [] };
    default:
      return { ...base, title: "GeoDuels update", href: "/notifications", icon: <Gift size={16} />, actions: [] };
  }
}

export function notificationEntriesFromSummary(summary: SocialSummary): NotificationFeedEntry[] {
  return [
    ...summary.incomingRequests.map((request) => ({ kind: "friend_request" as const, request })),
    ...summary.outgoingRequests.map((request) => ({ kind: "friend_request" as const, request })),
    ...summary.partyInvitations.map((invitation) => ({ kind: "party_invitation" as const, invitation })),
    ...summary.notifications.map((notification) => ({ kind: "notification" as const, notification })),
  ].sort((left, right) => parseNotificationEntry(right).createdAt.localeCompare(parseNotificationEntry(left).createdAt));
}

export function notificationDetails(notification: NotificationRecord) {
  return parseNotificationEntry({ kind: "notification", notification });
}

export function NotificationItem({ entry, onOpen, onAction, actionPending = false, compact = false }: {
  entry: NotificationFeedEntry;
  onOpen?: (entry: NotificationFeedEntry) => void;
  onAction?: (action: NotificationAction) => void;
  actionPending?: boolean;
  compact?: boolean;
}) {
  const item = parseNotificationEntry(entry);
  const actions = onAction && item.actions.length ? (
    <div className="pointer-events-auto flex gap-2">
      {item.actions.map((action) => (
        <IconButton key={action.value} aria-label={notificationActionLabel(action, item.title)} onClick={() => onAction(action)} disabled={actionPending} className="h-9 min-h-9 w-9">
          {action.value === "accept" ? <Check size={15} /> : <X size={15} />}
        </IconButton>
      ))}
    </div>
  ) : null;
  return (
    <div className={cn("relative transition hover:bg-surface-raised", item.unread && "bg-surface-raised")}>
      <Link href={item.href} onClick={() => onOpen?.(entry)} className="absolute inset-0 z-base focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-border-focus" aria-label={`${item.title}${item.unread ? ", unread" : ""}`} />
      <EntityRow
        leading={<span className="grid h-9 w-9 place-items-center rounded-full bg-status-info/15 text-content-primary">{item.icon}</span>}
        title={<span className="flex items-center gap-2">{item.title}{item.unread ? <Badge tone="info">New</Badge> : null}</span>}
        description={item.body}
        meta={<RelativeTime value={item.createdAt} />}
        actions={actions}
        className={cn("pointer-events-none relative z-content", compact && "min-h-14 px-2 py-2")}
      />
    </div>
  );
}

function notificationActionLabel(action: NotificationAction, title: string) {
  if (action.value === "accept") return `${action.kind === "party" ? "Join" : "Accept"}: ${title}`;
  if (action.value === "cancel") return `Cancel: ${title}`;
  return `Decline: ${title}`;
}

export function NotificationGroup({ title, children }: { title: string; children: ReactNode }) {
  return <section className="grid gap-2"><h3 className="px-1 text-label font-strong text-content-secondary">{title}</h3><InsetList>{children}</InsetList></section>;
}
