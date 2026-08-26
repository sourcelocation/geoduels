import type { NextRouter } from "next/router";
import {
  Ban,
  Bell,
  Award,
  FileText,
  Gavel,
  History,
  KeyRound,
  MessageCircle,
  Search,
  Wrench,
} from "lucide-react";

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
    items: [
      { href: "/admin/access/roles", label: "Roles", icon: KeyRound },
      { href: "/admin/access/badges", label: "Badges", icon: Award },
    ],
  },
];

export const moderatorNav = [
  {
    title: "Review",
    items: [
      { href: "/moderator/subjects", label: "Subjects", icon: Search },
      { href: "/moderator/signals", label: "Signals", icon: Gavel },
    ],
  },
  {
    title: "History",
    items: [{ href: "/moderator/log", label: "Moderation Log", icon: History }],
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
	return ["subjects"];
}
