import { useMutation, useQueryClient } from "@tanstack/react-query";
import { getRuntimeConfig } from "../../../lib/runtime-config";
import { socialClient } from "../../social/lib/social-client";
import type { NotificationAction } from "../components/NotificationItem";

export function useNotificationActions(accessToken?: string) {
  const config = getRuntimeConfig();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (action: NotificationAction) => {
      if (!accessToken) throw new Error("Sign in required");
      if (action.kind === "friend") return socialClient.respondRequest(config, accessToken, action.id, action.value);
      const invitation = await socialClient.respondPartyInvite(config, accessToken, action.id, action.value as "accept" | "decline");
      if (action.value === "accept" && invitation.inviteCode) window.location.assign(`/party/${encodeURIComponent(invitation.inviteCode)}`);
      return invitation;
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: ["social"] });
      void queryClient.invalidateQueries({ queryKey: ["notifications"] });
    },
  });
}
