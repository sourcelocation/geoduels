import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { getRuntimeConfig } from "../../../lib/runtime-config";
import { getAuthGateway } from "../auth-gateway";
import { requestUserNotifications } from "../lib/auth-client";
import { connectUserLive, type LiveEvent } from "../lib/user-live-client";
import type { FriendsPage } from "../../social/lib/social-client";
import { useAuthState } from "./AuthProvider";

export function UserLiveProvider({ children }: { children: React.ReactNode }) {
  const auth = useAuthState();
  const queryClient = useQueryClient();
  const config = getRuntimeConfig();

  useEffect(() => {
    if (!auth.canUseSocial || !auth.accessToken) return;
    const gateway = getAuthGateway(config);
    const apply = (event: LiveEvent) => {
      if (event.type === "notification.upsert") gateway.applyNotification(event.notification);
      if (event.type === "notification.read") gateway.applyNotificationRead(event.notificationId);
      if (event.type === "notification.read_all") gateway.applyNotificationReadAll();
      if (event.type === "global_status.changed") gateway.applyGlobal(event.global);
      if (event.type === "presence.patch") {
        queryClient.setQueriesData<FriendsPage>({ queryKey: ["social", "friends-page"] }, (current) => {
          if (!current) return current;
          const patchPlayer = (player: FriendsPage["friends"][number]) =>
            player.userId === event.presence.userId
              ? {
                  ...player,
                  presenceStatus: event.presence.presenceStatus,
                  activity: event.presence.activity || undefined,
                  lastSeenAt: event.presence.lastSeenAt || player.lastSeenAt,
                }
              : player;
          return {
            ...current,
            friends: current.friends.map(patchPlayer),
            recentPlayers: current.recentPlayers.map(patchPlayer),
          };
        });
      }
      if (event.type === "invalidate" && event.resources.includes("friends-page")) {
        void queryClient.invalidateQueries({ queryKey: ["social"] });
      }
      if (event.type === "notification.upsert" || event.type === "notification.read" || event.type === "notification.read_all") {
        void queryClient.invalidateQueries({ queryKey: ["notifications"] });
      }
    };
    const refill = () => {
      void requestUserNotifications(config, auth.accessToken)
        .then((payload) => {
          payload.notifications.forEach((notification) => gateway.applyNotification(notification));
        })
        .catch(() => undefined);
      void queryClient.invalidateQueries({ queryKey: ["social"] });
      void queryClient.invalidateQueries({ queryKey: ["notifications"] });
    };
    return connectUserLive(config, auth.accessToken, apply, { onReconnect: refill });
  }, [auth.accessToken, auth.canUseSocial, config, queryClient]);

  return children;
}
