// Fixed for every match and every deployment.
export const MAX_HP = 6_000;
export const ROUND_DURATION_MS = 45_000;
export const SOCKET_HEARTBEAT_INTERVAL_MS = 20_000;
export const SOCKET_STALE_AFTER_MS = 35_000;
export const CONNECTION_ERROR_MESSAGE = "Connection error";
export const GAME_CONNECTION_ERROR_MESSAGE = "Connection lost. Reconnecting...";

// Set when the image is built.
export const APP_VERSION =
  process.env.NEXT_PUBLIC_APP_VERSION || process.env.NEXT_PUBLIC_GIT_SHA?.slice(0, 12) || "dev";
