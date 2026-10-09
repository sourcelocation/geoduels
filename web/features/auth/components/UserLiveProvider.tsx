import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { useRuntimeConfig } from "../../../lib/runtime-config-context";
import { getAuthGateway } from "../auth-gateway";
import type { UserNotification } from "../lib/auth-client";
import { unseenNotificationsKey } from "../../notifications/lib/unseen";
import { connectUserLive, type LiveEvent } from "../lib/user-live-client";
import type { FriendsPage } from "../../social/lib/social-client";
import { useAuthState } from "./AuthProvider";

const socialNotifications = new Set(["friend_request_received", "friendship_accepted", "party_invitation_received"]);

export function UserLiveProvider({ children }: { children: React.ReactNode }) {
  const auth = useAuthState();
  const queryClient = useQueryClient();
  const config = useRuntimeConfig();

  useEffect(() => {
    if (!auth.canUseSocial || !auth.accessToken || !auth.userId) return;
    const gateway = getAuthGateway(config);
    const unseenKey = unseenNotificationsKey(auth.userId);
    const updateUnseen = (update: (items: UserNotification[]) => UserNotification[]) =>
      queryClient.setQueryData<UserNotification[]>(unseenKey, (items) => (items ? update(items) : items));
    const apply = (event: LiveEvent) => {
      if (event.type === "notification.upsert") {
        const { notification } = event;
        updateUnseen((items) => [...items.filter((item) => item.id !== notification.id), notification]);
        if (socialNotifications.has(notification.type)) void queryClient.invalidateQueries({ queryKey: ["social"] });
        if (notification.type === "moderation_warning") void queryClient.invalidateQueries({ queryKey: ["moderation-warnings"] });
      }
      // Another device showed it.
      if (event.type === "notification.read") updateUnseen((items) => items.filter((item) => item.id !== event.notificationId));
      if (event.type === "notification.read_all") updateUnseen(() => []);
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
    };
    // Whatever happened while the connection was down is read again.
    const refill = () => {
      void queryClient.invalidateQueries({ queryKey: unseenKey });
      void queryClient.invalidateQueries({ queryKey: ["social"] });
    };
    return connectUserLive(config, auth.accessToken, apply, { onReconnect: refill });
  }, [auth.accessToken, auth.canUseSocial, auth.userId, config, queryClient]);

  return children;
}
