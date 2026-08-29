import { useQuery } from "@tanstack/react-query";
import { getRuntimeConfig } from "../../../lib/runtime-config";
import { socialClient } from "../lib/social-client";

export function useFriendsPage(accessToken?: string, enabled = true, partyId?: string) {
  const config = getRuntimeConfig();
  const scopedPartyId = partyId?.trim() || "";
  return useQuery({
    queryKey: ["social", "friends-page", scopedPartyId],
    enabled: !!accessToken && enabled,
    queryFn: () => socialClient.friendsPage(config, accessToken!, scopedPartyId || undefined),
    staleTime: 20_000,
  });
}
