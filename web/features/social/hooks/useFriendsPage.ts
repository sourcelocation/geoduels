import { useQuery } from "@tanstack/react-query";
import { useSyncExternalStore } from "react";
import { socialClient } from "../lib/social-client";
import { socialStore } from "../social-store";

/** Friends as they are now, told by the server, and what's only read when the page is looked at. */
export function useFriendsPage(accessToken?: string, enabled = true, partyId?: string) {
  const social = useSyncExternalStore(socialStore.subscribe, socialStore.getState, socialStore.getState);
  const scopedPartyId = partyId?.trim() || "";
  const details = useQuery({
    queryKey: ["social", "friends-page", scopedPartyId],
    enabled: !!accessToken && enabled,
    queryFn: () => socialClient.friendsPage(accessToken!, scopedPartyId || undefined),
    staleTime: 60_000,
  });
  const invites = new Map(details.data?.friends.map((friend) => [friend.userId, friend.partyInvite]));
  return {
    friends: social.friends.map((friend) => ({ ...friend, partyInvite: invites.get(friend.userId) })),
    outgoing: social.outgoing,
    pending: social.pending,
    recentPlayers: details.data?.recentPlayers ?? [],
    isLoading: !social.loaded,
  };
}
