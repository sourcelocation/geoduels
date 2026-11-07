import { Map, Play, Trophy, Users, type LucideIcon } from "lucide-react";

export type AppNavRoute = "friends" | "play" | "maps" | "top";

export type AppNavItem = {
  label: string;
  route: AppNavRoute;
  href: string;
  icon: LucideIcon;
};

export const APP_NAV_ITEMS: AppNavItem[] = [
  { label: "Play", route: "play", href: "/", icon: Play },
  { label: "Maps", route: "maps", href: "/maps", icon: Map },
  { label: "Friends", route: "friends", href: "/friends", icon: Users },
  { label: "Top", route: "top", href: "/top", icon: Trophy },
];

/** Which section a page belongs to, if any. */
export function navRouteOf(pathname: string): AppNavRoute | null {
  if (pathname === "/") return "play";
  if (pathname === "/top") return "top";
  if (pathname === "/maps" || pathname.startsWith("/maps/")) return "maps";
  if (pathname === "/friends" || pathname.startsWith("/friend/") || pathname.startsWith("/party/")) return "friends";
  return null;
}

export const appNavRouteStorageKey = "geoduels.lobbyRoute";

export function isAppNavRoute(value: string): value is AppNavRoute {
  return APP_NAV_ITEMS.some((item) => item.route === value);
}
