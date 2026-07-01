import type { MapScope } from "../../maps/lib/maps-client";
import { getTeamPresentation } from "../../../lib/team-presentation";
import { formatRelativeTime } from "../../../components/ui/RelativeTime";
import {
  APP_NAV_ITEMS,
  appNavRouteStorageKey,
  isAppNavRoute,
  type AppNavRoute,
} from "../../app-shell/navigation";

export type LobbyContentRoute =
  | AppNavRoute
  | "map-details"
  | "map-upload"
  | "party";

export const CLOCK_OPTIONS = [
  { value: "infinite", label: "Infinite" },
  { value: "30", label: "30s" },
  { value: "45", label: "45s" },
  { value: "60", label: "60s" },
  { value: "90", label: "90s" },
  { value: "120", label: "120s" },
] as const;

export const PRESSURE_OPTIONS = [
  { value: "none", label: "None" },
  { value: "15", label: "15s" },
] as const;

export const NAV_ITEMS = APP_NAV_ITEMS;
export const lobbyRouteStorageKey = appNavRouteStorageKey;

export function lobbyTeamLabel(teamId?: string) {
  return getTeamPresentation(teamId).name;
}

export function lobbyTeamTextClass(teamId?: string) {
  return getTeamPresentation(teamId).textClassName;
}

export function lobbyTeamPillClass(teamId?: string, active = false) {
  const team = getTeamPresentation(teamId);
  return active ? team.activeClassName : team.inactiveClassName;
}

export function isMapScope(value: unknown): value is MapScope {
  return value === "official" || value === "community" || value === "favorites" || value === "mine";
}

export function isLobbyNavRoute(value: string): value is LobbyContentRoute {
  return isAppNavRoute(value);
}

export function parseTime(value?: string) {
  if (!value) return null;
  const ms = Date.parse(value);
  return Number.isFinite(ms) ? ms : null;
}

export function formatRelativeDuration(ms: number) {
  const totalSeconds = Math.max(0, Math.ceil(ms / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) return `${hours}h ${minutes}m`;
  if (minutes > 0) {
    return seconds > 0 ? `${minutes}m ${seconds}s` : `${minutes}m`;
  }
  return `${seconds}s`;
}

export function formatApproximateTime(ms: number) {
  const totalMinutes = Math.max(1, Math.ceil(ms / 60000));
  if (totalMinutes >= 60) {
    const hours = Math.floor(totalMinutes / 60);
    const minutes = totalMinutes % 60;
    return minutes === 0
      ? `about ${hours} hour${hours === 1 ? "" : "s"}`
      : `about ${hours}h ${minutes}m`;
  }
  return `about ${totalMinutes} minute${totalMinutes === 1 ? "" : "s"}`;
}

export function formatQueueElapsed(ms: number) {
  const totalSeconds = ms > 0 ? Math.max(1, Math.ceil(ms / 1000)) : 0;
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
}

export function formatChangelogDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    timeZone: "UTC",
  }).format(date);
}

export function formatCommentAge(value: string) {
  return formatRelativeTime(value);
}

export function commentAvatarFallback(name: string) {
  return (name.trim() || "?").slice(0, 1).toUpperCase();
}

export function commentDeletedLabel(status: string) {
  return status === "visible" ? "" : "(deleted)";
}
