import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/router";
import { useRuntimeConfig } from "../../../lib/runtime-config-context";
import { getHomeRuntime } from "../../home/state/home-runtime";
import { socialClient } from "../lib/social-client";

export type RequestAction =
  | { kind: "friend"; requestId: string; response: "accept" | "decline" }
  | { kind: "party"; invitationId: string; response: "accept" | "decline" };

/** Answers friend requests and party invitations; accepting an invitation also joins the party. */
export function useRequestActions(accessToken?: string) {
  const config = useRuntimeConfig();
  const queryClient = useQueryClient();
  const router = useRouter();
  return useMutation({
    mutationFn: async (action: RequestAction) => {
      if (!accessToken) throw new Error("Sign in required");
      if (action.kind === "friend") {
        await socialClient.respondRequest(config, accessToken, action.requestId, action.response);
        return;
      }
      const invitation = await socialClient.respondPartyInvite(config, accessToken, action.invitationId, action.response);
      if (action.response !== "accept" || !invitation.inviteCode) return;
      const inviteCode = invitation.inviteCode.trim().toUpperCase();
      if (!(await getHomeRuntime(config).partyController.joinParty(inviteCode))) {
        throw new Error("Could not join party");
      }
      const nextPath = `/party/${encodeURIComponent(inviteCode)}`;
      if (router.asPath.split("?")[0] !== nextPath) {
        await router.replace(nextPath, undefined, { shallow: true });
      }
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: ["social"] }),
  });
}
