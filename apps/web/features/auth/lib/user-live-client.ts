import type { RuntimeConfig } from "../../../lib/runtime-config";
import { normalizeWSBase } from "../../../lib/runtime-config";
import type { UserNotification } from "./auth-client";
import type { AppBootstrapPayload } from "./auth-client";

export type LivePresencePatch = {
  userId: string;
  presenceStatus: "online" | "away" | "offline";
  activity?: "in_match" | "in_party" | "";
  lastSeenAt?: string;
};

export type LiveEvent =
  | { type: "hello" }
  | { type: "notification.upsert"; notification: UserNotification }
  | { type: "notification.read"; notificationId: number }
  | { type: "notification.read_all" }
  | { type: "presence.patch"; presence: LivePresencePatch }
  | { type: "invalidate"; resources: string[] }
  | {
      type: "global_status.changed";
      global: AppBootstrapPayload["global"];
    };

export type LiveEventHandler = (event: LiveEvent) => void;

function apiWSBase(config: RuntimeConfig) {
  const http = config.apiURL || (typeof window !== "undefined" ? window.location.origin : "");
  return normalizeWSBase(http).replace(/\/$/, "");
}

export function parseLiveEvent(value: unknown): LiveEvent | null {
  if (!value || typeof value !== "object") return null;
  const record = value as Record<string, unknown>;
  const type = record.type;
  if (type === "hello") return { type };
  if (type === "notification.upsert" && record.notification && typeof record.notification === "object") {
    return { type, notification: record.notification as UserNotification };
  }
  if (type === "notification.read" && typeof record.notificationId === "number") {
    return { type, notificationId: record.notificationId };
  }
  if (type === "notification.read_all") return { type };
  if (type === "presence.patch" && record.presence && typeof record.presence === "object") {
    const presence = record.presence as LivePresencePatch;
    if (!presence.userId || !presence.presenceStatus) return null;
    return { type, presence };
  }
  if (type === "invalidate") {
    const resources = Array.isArray(record.resources) ? record.resources.filter((item): item is string => typeof item === "string") : [];
    return { type, resources };
  }
  if (type === "global_status.changed" && record.global && typeof record.global === "object") {
    return { type, global: record.global as AppBootstrapPayload["global"] };
  }
  return null;
}

export function connectUserLive(
  config: RuntimeConfig,
  accessToken: string,
  onEvent: LiveEventHandler,
  options?: { onReconnect?: () => void },
): () => void {
  let closed = false;
  let socket: WebSocket | null = null;
  let reconnectTimer = 0;
  let attempt = 0;

  const disconnect = () => {
    closed = true;
    window.clearTimeout(reconnectTimer);
    socket?.close();
    socket = null;
  };

  const open = () => {
    if (closed || typeof WebSocket === "undefined") return;
    const url = `${apiWSBase(config)}/v1/me/live?accessToken=${encodeURIComponent(accessToken)}`;
    const next = new WebSocket(url);
    socket = next;
    next.onmessage = (message) => {
      try {
        const event = parseLiveEvent(JSON.parse(String(message.data)));
        if (event) onEvent(event);
      } catch {
        return;
      }
    };
    next.onopen = () => {
      if (attempt > 0) options?.onReconnect?.();
      attempt = 0;
    };
    next.onclose = () => {
      if (closed) return;
      const delay = Math.min(15_000, 1000 * 2 ** attempt);
      attempt += 1;
      reconnectTimer = window.setTimeout(open, delay);
    };
  };

  open();
  return disconnect;
}
