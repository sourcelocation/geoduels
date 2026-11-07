import { useMutation, useQuery } from "@tanstack/react-query";
import { Copy, Search, UserPlus } from "lucide-react";
import { useEffect, useState } from "react";
import { AsyncState, InsetList, SectionHeader } from "../../../components/ui/patterns";
import { Button } from "../../../components/ui/button";
import { AppPanel } from "../../../components/ui/compositions";
import { LobbyInput } from "../../lobby/components/lobby-primitives";
import { useFriendsPage } from "../hooks/useFriendsPage";
import { socialClient } from "../lib/social-client";
import { ItemActions } from "../../chat/components/ItemActions";
import { homeRuntime } from "../../home/state/home-runtime";
import type { Friend, PendingItem } from "../../live/events";
import type { CompactPlayer, FriendRequest } from "../types";
import { CompactPlayerRow } from "./CompactPlayerRow";
import { InviteToPartyButton } from "./InviteToPartyButton";
import { PlayerListSection } from "./PlayerListSection";
import { RelationshipActions } from "./RelationshipActions";

export function FriendsDashboard({
  accessToken,
  isGuest,
  partyId,
  memberUserIds,
  partyCard,
}: {
  accessToken?: string;
  isGuest: boolean;
  partyId?: string;
  memberUserIds?: readonly string[];
  partyCard?: React.ReactNode;
}) {
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedQuery(query.trim()), 250);
    return () => window.clearTimeout(timer);
  }, [query]);
  const enabled = !!accessToken && !isGuest;
  const page = useFriendsPage(accessToken, enabled, partyId);
  const memberIds = new Set(memberUserIds || []);
  const friendActions = (player: CompactPlayer) => (
    <>
      <RelationshipActions accessToken={accessToken!} player={player} />
      {partyId && player.relationship === "friends" && !memberIds.has(player.userId) ? (
        <InviteToPartyButton
          accessToken={accessToken!}
          partyId={partyId}
          userId={player.userId}
          displayName={player.displayName}
          partyInvite={player.partyInvite || page.friends.find((friend) => friend.userId === player.userId)?.partyInvite}
        />
      ) : null}
    </>
  );
  const friendRowActions = (friend: Friend) => (
    <>
      <Button type="button" size="sm" variant="secondary" onClick={() => homeRuntime.directChat.open(friend.conversationId)}>
        Message{friend.unread ? ` (${friend.unread})` : ""}
      </Button>
      {friendActions(friend)}
    </>
  );
  const search = useQuery({
    queryKey: ["social", "search", debouncedQuery],
    enabled: enabled && debouncedQuery.length >= 2,
    queryFn: () => socialClient.search(accessToken!, debouncedQuery),
  });
  const code = useMutation({
    mutationFn: () => socialClient.createCode(accessToken!),
    onSuccess: async (value) => {
      const url = `${window.location.origin}/friend/${value.code}`;
      await navigator.clipboard?.writeText(url);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1600);
    },
  });

  if (!enabled) {
    return (
      <div className="w-full max-w-[720px] space-y-5">
        {partyCard}
        <AppPanel className="rounded-2xl p-6">
          <AsyncState status="empty" title="Sign in to build your friends list" message="Your friends will appear here… hopefully, once you have some." />
        </AppPanel>
      </div>
    );
  }

  const searchPlayers = search.data?.players || [];
  const allFriends = page.friends;
  const offlineFriends = allFriends.filter((player) => player.presenceStatus !== "online");
  const onlineFriends = allFriends.filter((player) => player.presenceStatus === "online");
  return (
    <div className="w-full max-w-[820px] space-y-5">
      {partyCard}
      <AppPanel className="rounded-2xl p-5 sm:p-6">
        <SectionHeader eyebrow="Social" title="Friends" description="Find players, see who is around, and get into a party quickly." actions={
          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={() => code.mutate()}
            disabled={code.isPending}
          >
            {copied ? <Copy size={16} /> : <UserPlus size={16} />}
            {copied ? "Link copied" : "Share friend link"}
          </Button>
        } />
        <div className="relative mt-5">
          <Search size={17} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-content-secondary" />
          <LobbyInput
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search players"
            aria-label="Search players"
            className="pl-10"
          />
        </div>
        {debouncedQuery.length >= 2 ? (
          <PlayerListSection
            title="Search results"
            players={searchPlayers}
            loading={search.isLoading}
            renderActions={friendActions}
          />
        ) : null}

        {page.pending.length ? <PendingSection pending={page.pending} /> : null}
        {page.outgoing.length ? <SentSection requests={page.outgoing} token={accessToken!} /> : null}
        {page.isLoading ? <AsyncState status="loading" message="Loading friends" className="mt-5" /> : null}
        {!page.isLoading && !allFriends.length ? <AsyncState status="empty" message="No friends (yet!)" className="mt-5" /> : null}
        {onlineFriends.length ? <PlayerListSection title="Online" players={onlineFriends} renderActions={friendRowActions} /> : null}
        {offlineFriends.length ? <PlayerListSection title="Offline" players={offlineFriends} renderActions={friendRowActions} /> : null}
        {page.recentPlayers.length ? (
          <PlayerListSection title="Recently played" players={page.recentPlayers.slice(0, 3)} renderActions={friendActions} />
        ) : null}
      </AppPanel>
    </div>
  );
}

function PendingSection({ pending }: { pending: PendingItem[] }) {
  return (
    <section className="mt-5">
      <SectionHeader title="Waiting for you" className="mb-3" />
      <InsetList>
        {pending.map(({ item, player }) => (
          <CompactPlayerRow
            key={item.id}
            player={player}
            meta={<span className="text-label text-content-secondary">{item.body}</span>}
            actions={<ItemActions actions={item.actions ?? []} />}
          />
        ))}
      </InsetList>
    </section>
  );
}

function SentSection({ requests, token }: { requests: FriendRequest[]; token: string }) {
  return (
    <section className="mt-5">
      <SectionHeader title="Sent requests" className="mb-3" />
      <InsetList>
        {requests.map((request) => (
          <CompactPlayerRow
            key={request.id}
            player={request.player}
            actions={<RelationshipActions accessToken={token} player={{ ...request.player, requestId: request.id }} />}
          />
        ))}
      </InsetList>
    </section>
  );
}
