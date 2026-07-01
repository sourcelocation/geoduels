import type { NextRouter } from "next/router";
import {
  Ban,
  Bell,
  ClipboardList,
  FileText,
  Gavel,
  History,
  KeyRound,
  MessageCircle,
  Search,
  UserCog,
  Wrench,
} from "lucide-react";

export const moderationViews = new Set([
  "queue",
  "mine",
  "watchlist",
  "closed",
]);

export const adminNav = [
  {
    title: "Operations",
    items: [
      { href: "/admin/operations/maintenance", label: "Maintenance", icon: Wrench },
      { href: "/admin/operations/seasons", label: "Seasons", icon: History },
      { href: "/admin/operations/notifications", label: "Notifications", icon: Bell },
      { href: "/admin/operations/discord", label: "Discord", icon: MessageCircle },
      { href: "/admin/operations/ip-signup-blocks", label: "IP Signup Blocks", icon: Ban },
      { href: "/admin/content/changelog", label: "Changelog", icon: FileText },
    ],
  },
  {
    title: "Access",
    items: [{ href: "/admin/access/roles", label: "Roles", icon: KeyRound }],
  },
];

export const moderatorNav = [
  {
    title: "Review",
    items: [
      { href: "/moderator/queue", label: "Queue", icon: ClipboardList },
      { href: "/moderator/reviews", label: "Mine", icon: UserCog },
      { href: "/moderator/watchlist", label: "Watchlist", icon: Search },
      { href: "/moderator/closed", label: "Closed", icon: History },
      { href: "/moderator/subjects", label: "Subjects", icon: Search },
    ],
  },
  {
    title: "History",
    items: [{ href: "/moderator/enforcement", label: "Enforcement", icon: Gavel }],
  },
];

export function pathFromRouter(router: NextRouter) {
  const rawPath = router.query.path;
  if (Array.isArray(rawPath) && rawPath.length > 0) return rawPath;
  const tab = router.query.tab;
  if (typeof tab === "string") return [tab];
  return ["operations", "maintenance"];
}

export function moderatorPathFromRouter(router: NextRouter) {
  const rawPath = router.query.path;
  if (Array.isArray(rawPath) && rawPath.length > 0) return rawPath;
  const tab = router.query.tab;
  if (typeof tab === "string") return [tab];
  return ["queue"];
}

export function isModerationReviewSection(section: string) {
  return section === "queue" || section === "reviews" || section === "watchlist" || section === "closed";
}

export function moderationViewForRoute(section: string, leaf: string) {
  if (section === "reviews") return "mine";
  if (section === "watchlist") return "watchlist";
  if (section === "closed") return "closed";
  if (moderationViews.has(leaf)) return leaf;
  return "queue";
}

export function moderationTitleForRoute(section: string, leaf: string) {
  if (section === "reviews") return "My Reviews";
  if (section === "watchlist") return "Watchlist";
  if (section === "closed") return "Closed";
  return "Review Queue";
}
