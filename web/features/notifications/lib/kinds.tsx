import type { ReactNode } from "react";
import { ShieldAlert, ShieldCheck, UserCheck, UserPlus, Users } from "lucide-react";
import type { Toast } from "../../../components/ui/AppNotice";
import type { UserNotification } from "../../auth/lib/auth-client";
import type { RequestAction } from "../../social/hooks/useRequestActions";

/** A modal waits for the person; its content is the kind's own. */
export type NotificationModal = {
  title: string;
  eyebrow: string;
  tone: "success" | "danger";
  content: ReactNode;
  confirm: string;
  /** Closing it reads the account again, which the notification changed. */
  refreshesAccount?: boolean;
};

export type Presentation =
  | { as: "toast"; toast: Toast }
  | { as: "modal"; modal: NotificationModal }
  | { as: "none" };

/** What a toast's action may do, supplied by whoever shows it. */
export type NotificationActions = {
  answer: (action: RequestAction) => void;
  open: (href: string) => void;
};

/** Toasts are news; older news is marked seen without being shown. */
const NEWS_MS = 7 * 24 * 60 * 60 * 1000;

/**
 * How each kind of notification is shown: a modal for what waits for the person,
 * a toast for the rest, or nothing when another part of the app shows it.
 */
export function presentNotification(notification: UserNotification, actions: NotificationActions): Presentation {
  const { payload } = notification;
  const actor = notification.actorDisplayName;
  const toast = (value: Toast): Presentation =>
    Date.now() - Date.parse(notification.createdAt) > NEWS_MS ? { as: "none" } : { as: "toast", toast: value };

  switch (notification.type) {
    case "friend_request_received": {
      const requestId = payload.requestId;
      return toast({
        icon: <UserPlus size={16} />,
        title: actor ? `${actor} sent you a friend request` : "New friend request",
        action: requestId ? { label: "Accept", onClick: () => actions.answer({ kind: "friend", requestId, response: "accept" }) } : undefined,
        durationMs: 10_000,
      });
    }
    case "party_invitation_received": {
      const invitationId = payload.invitationId;
      return toast({
        icon: <Users size={16} />,
        title: actor ? `${actor} invited you to a party` : "New party invitation",
        action: invitationId ? { label: "Join", onClick: () => actions.answer({ kind: "party", invitationId, response: "accept" }) } : undefined,
        durationMs: 15_000,
      });
    }
    case "friendship_accepted":
      return toast({
        icon: <UserCheck size={16} />,
        title: actor ? `${actor} accepted your friend request` : "Friend request accepted",
        action: actor ? { label: "Profile", onClick: () => actions.open(`/players/${encodeURIComponent(actor)}`) } : undefined,
      });
    case "badge_unlocked": {
      const badge = payload.badge;
      if (!badge) return { as: "none" };
      return {
        as: "modal",
        modal: {
          title: "New badge unlocked",
          eyebrow: "New badge unlocked!",
          tone: "success",
          confirm: "Claim",
          refreshesAccount: true,
          content: (
            <div className="mt-5 flex flex-col items-center text-center">
              <img src={badge.imageUrl} alt="" className="h-24 w-24 object-contain drop-shadow-lg" />
              <h2 className="mt-4 text-heading-md font-strong">{badge.label}</h2>
              {badge.description ? <p className="mt-3 text-body-sm text-content-secondary">{badge.description}</p> : null}
            </div>
          ),
        },
      };
    }
    case "mmr_refund":
      return {
        as: "modal",
        modal: {
          title: "Rating refunded",
          eyebrow: "Rating refunded",
          tone: "success",
          confirm: "Got it",
          refreshesAccount: true,
          content: (
            <>
              <h2 className="mt-2 text-heading-md font-strong">+{payload.refundDelta || 0} MMR</h2>
              <p className="mt-3 text-body-sm text-content-secondary">
                A player you lost to was banned for cheating. Your rating has been recalculated from your current MMR and refunded.
              </p>
            </>
          ),
        },
      };
    case "account_banned":
      return {
        as: "modal",
        modal: {
          title: "Account suspended",
          eyebrow: "Account suspended",
          tone: "danger",
          confirm: "Got it",
          content: (
            <p className="mt-3 text-body-sm text-content-secondary">
              {payload.reason ? `Reason: ${payload.reason}` : "Your account access has been restricted."}
            </p>
          ),
        },
      };
    case "account_unbanned":
      return toast({ icon: <ShieldCheck size={16} />, title: "Account restriction removed", body: payload.reason || "Everything works again." });
    case "reported_player_banned":
      return toast({
        icon: <ShieldAlert size={16} />,
        title: "Report action taken",
        body: "A player you reported was suspended after review. Thanks for keeping GeoDuels fair.",
      });
    case "moderation_warning":
      // The warning itself waits to be acknowledged in WarningPrompt.
      return payload.withdrawn ? toast({ icon: <ShieldCheck size={16} />, title: "A warning was withdrawn" }) : { as: "none" };
    default:
      return { as: "none" };
  }
}
