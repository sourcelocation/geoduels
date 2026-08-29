import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/router";
import { getRuntimeConfig } from "../../../lib/runtime-config";
import { getHomeRuntime } from "../../home/state/home-runtime";
import { socialClient } from "../../social/lib/social-client";
import type { NotificationAction } from "../components/NotificationItem";
import { getAuthGateway } from "../../auth/auth-gateway";

export function useNotificationActions(
  accessToken?: string,
  joinParty?: (inviteCode: string) => Promise<boolean>,
) {
  const config = getRuntimeConfig();
  const queryClient = useQueryClient();
  const router = useRouter();
  const admitParty =
    joinParty ??
    ((inviteCode: string) => getHomeRuntime(config).partyController.joinParty(inviteCode));
  return useMutation({
    mutationFn: async (action: NotificationAction) => {
      if (!accessToken) throw new Error("Sign in required");
      if (action.kind === "friend") return socialClient.respondRequest(config, accessToken, action.id, action.value);
      const invitation = await socialClient.respondPartyInvite(config, accessToken, action.id, action.value as "accept" | "decline");
      if (action.value === "accept" && invitation.inviteCode) {
        const inviteCode = invitation.inviteCode.trim().toUpperCase();
        const joined = await admitParty(inviteCode);
        if (!joined) throw new Error("Could not join party");
        const nextPath = `/party/${encodeURIComponent(inviteCode)}`;
        if (router.asPath.split("?")[0] !== nextPath) {
          await router.replace(nextPath, undefined, { shallow: true });
        }
      }
      return invitation;
    },
    onSettled: async () => {
      await getAuthGateway(config).bootstrap({ force: true });
      void queryClient.invalidateQueries({ queryKey: ["social"] });
      void queryClient.invalidateQueries({ queryKey: ["notifications"] });
    },
  });
}
