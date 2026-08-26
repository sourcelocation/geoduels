import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createContext, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useAuthState } from "../../auth/components/AuthProvider";
import { apiFetch, apiPath } from "../../../lib/http";
import { getRuntimeConfig } from "../../../lib/runtime-config";
import type { MaintenanceStatus } from "../../matchmaking/lib/queue-client";

export type SocialSession = {
  userId: string;
  accessToken: string;
  isGuest: boolean;
};

type GlobalRealtimeState = {
  onlinePlayers?: number;
  maintenance: MaintenanceStatus | null;
  connectionState: "idle" | "connecting" | "open" | "reconnecting";
};
const GlobalRealtimeContext = createContext<GlobalRealtimeState>({
  maintenance: null,
  connectionState: "idle",
});

export function useGlobalRealtime() {
  return useContext(GlobalRealtimeContext);
}

export function SocialRealtimeProvider({ children }: { children: ReactNode }) {
  const config = getRuntimeConfig();
  const queryClient = useQueryClient();
  const lastSequence = useRef(0);
  const [globalState, setGlobalState] = useState<GlobalRealtimeState>({
    maintenance: null,
    connectionState: "idle",
  });
  const auth = useAuthState();
  const value = useMemo<SocialSession | null>(() => (
    auth.userId && auth.accessToken ? {
      userId: auth.userId,
      accessToken: auth.accessToken,
      isGuest: auth.isGuest,
    } : null
  ), [auth.accessToken, auth.isGuest, auth.userId]);
  const publicStatus = useQuery({
    queryKey: ["global-status"],
    enabled: auth.status === "anonymous",
    queryFn: async () => {
      const response = await apiFetch(config, "/v1/status");
      if (!response.ok) throw new Error("Status unavailable");
      return response.json() as Promise<{
        onlinePlayers: number;
        maintenance: MaintenanceStatus;
      }>;
    },
    refetchInterval: 60_000,
    staleTime: 30_000,
  });

  useEffect(() => {
    if (!publicStatus.data) return;
    setGlobalState({
      onlinePlayers: publicStatus.data.onlinePlayers,
      maintenance: normalizeGlobalMaintenance(publicStatus.data.maintenance),
      connectionState: "idle",
    });
  }, [publicStatus.data]);

  useEffect(() => {
    if (!value || typeof window === "undefined") return;
    let socket: WebSocket | null = null;
    let reconnectTimer = 0;
    let stopped = false;
    let reconnectDelay = 1000;

    const connect = () => {
      setGlobalState((current) => ({ ...current, connectionState: current.connectionState === "open" ? "reconnecting" : "connecting" }));
      const httpURL = new URL(apiPath(config, "/v1/me/events/ws"), window.location.origin);
      httpURL.protocol = httpURL.protocol === "https:" ? "wss:" : "ws:";
      httpURL.searchParams.set("access_token", value.accessToken);
      httpURL.searchParams.set("after", String(lastSequence.current));
      socket = new WebSocket(httpURL);
      socket.onopen = () => {
        reconnectDelay = 1000;
        setGlobalState((current) => ({ ...current, connectionState: "open" }));
        socket?.send(JSON.stringify({
          type: "presence",
          status: document.visibilityState === "visible" ? "online" : "away",
        }));
      };
      socket.onmessage = (event) => {
        try {
          const message = JSON.parse(event.data);
          if (message.type === "global_status.changed") {
            setGlobalState({
              onlinePlayers: message.data?.onlinePlayers,
              maintenance: normalizeGlobalMaintenance(message.data?.maintenance),
              connectionState: "open",
            });
          }
          if (typeof message.sequence === "number") lastSequence.current = message.sequence;
          if (message.type === "resync_required" || message.type?.startsWith("friend") || message.type?.startsWith("party_") || message.type === "presence.changed") {
            void queryClient.invalidateQueries({ queryKey: ["social"] });
          }
          if (message.type === "notification.created" || message.type?.includes("invitation")) {
            void queryClient.invalidateQueries({ queryKey: ["social", "summary"] });
            void queryClient.invalidateQueries({ queryKey: ["notifications"] });
          }
        } catch {
          void queryClient.invalidateQueries({ queryKey: ["social"] });
        }
      };
      socket.onclose = () => {
        if (stopped) return;
        setGlobalState((current) => ({ ...current, connectionState: "reconnecting" }));
        reconnectTimer = window.setTimeout(connect, reconnectDelay);
        reconnectDelay = Math.min(30_000, reconnectDelay * 2);
      };
    };
    const visibility = () => {
      if (socket?.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({
          type: "presence",
          status: document.visibilityState === "visible" ? "online" : "away",
        }));
      }
      if (document.visibilityState === "visible") {
        void queryClient.invalidateQueries({ queryKey: ["social"] });
      }
    };
    document.addEventListener("visibilitychange", visibility);
    connect();
    return () => {
      stopped = true;
      window.clearTimeout(reconnectTimer);
      document.removeEventListener("visibilitychange", visibility);
      socket?.close();
    };
  }, [config, queryClient, value]);

  return (
    <GlobalRealtimeContext.Provider value={globalState}>
      {children}
    </GlobalRealtimeContext.Provider>
  );
}

function normalizeGlobalMaintenance(value?: Partial<MaintenanceStatus> | null): MaintenanceStatus | null {
  if (!value) return null;
  const status: MaintenanceStatus = {
    phase: value.phase === "warning" || value.phase === "active" ? value.phase : "normal",
    startsAt: value.startsAt || "",
    endsAt: value.endsAt || "",
    queuePaused: !!value.queuePaused,
    playPaused: !!value.playPaused,
    message: value.message || "",
  };
  return status.phase === "normal" && !status.queuePaused && !status.playPaused && !status.message && !status.startsAt && !status.endsAt
    ? null
    : status;
}
