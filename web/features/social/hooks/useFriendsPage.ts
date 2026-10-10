import { useQuery } from "@tanstack/react-query";
import { useRuntimeConfig } from "../../../lib/runtime-config-context";
import { socialClient } from "../lib/social-client";

export function useFriendsPage(accessToken?: string, enabled = true, partyId?: string) {
  const config = useRuntimeConfig();
  const scopedPartyId = partyId?.trim() || "";
  return useQuery({
    queryKey: ["social", "friends-page", scopedPartyId],
    enabled: !!accessToken && enabled,
    queryFn: () => socialClient.friendsPage(config, accessToken!, scopedPartyId || undefined),
    staleTime: 20_000,
    // Friends' online status is read, not pushed: refresh while the list is on screen and the tab
    // is visible, and on focus.
    refetchInterval: 30_000,
  });
}
