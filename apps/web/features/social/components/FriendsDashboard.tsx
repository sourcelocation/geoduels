import { useMutation, useQuery } from "@tanstack/react-query";
import { Copy, Search, UserPlus } from "lucide-react";
import { useEffect, useState } from "react";
import { AsyncState, InsetList, SectionHeader } from "../../../components/ui/patterns";
import { Button } from "../../../components/ui/button";
import { AppPanel } from "../../../components/ui/compositions";
import { LobbyInput } from "../../lobby/components/lobby-primitives";
import { getRuntimeConfig } from "../../../lib/runtime-config";
import { socialClient } from "../lib/social-client";
import type { CompactPlayer } from "../types";
import { CompactPlayerRow } from "./CompactPlayerRow";
import { RelationshipActions } from "./RelationshipActions";

export function FriendsDashboard({
  accessToken,
  isGuest,
  partyId,
  partyCard,
}: {
  accessToken?: string;
  isGuest: boolean;
  partyId?: string;
  partyCard?: React.ReactNode;
}) {
  const config = getRuntimeConfig();
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedQuery(query.trim()), 250);
    return () => window.clearTimeout(timer);
  }, [query]);
  const enabled = !!accessToken && !isGuest;
  const friends = useQuery({
    queryKey: ["social", "friends"],
    enabled,
    queryFn: () => socialClient.friends(config, accessToken!),
    staleTime: 20_000,
  });
  const incoming = useQuery({
    queryKey: ["social", "requests", "incoming"],
    enabled,
    queryFn: () => socialClient.requests(config, accessToken!, "incoming"),
  });
  const outgoing = useQuery({
    queryKey: ["social", "requests", "outgoing"],
    enabled,
    queryFn: () => socialClient.requests(config, accessToken!, "outgoing"),
  });
  const recent = useQuery({
    queryKey: ["social", "recent"],
    enabled,
    queryFn: () => socialClient.recent(config, accessToken!),
  });
  const search = useQuery({
    queryKey: ["social", "search", debouncedQuery],
    enabled: enabled && debouncedQuery.length >= 2,
    queryFn: () => socialClient.search(config, accessToken!, debouncedQuery),
  });
  const code = useMutation({
    mutationFn: () => socialClient.createCode(config, accessToken!),
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
  const allFriends = friends.data?.friends || [];
  const offlineFriends = allFriends.filter((player) => player.presenceStatus !== "online");
  const onlineFriends = allFriends.filter((player) => player.presenceStatus === "online");
  const pendingRequests = [
    ...(incoming.data?.requests || []).map((request) => ({ ...request, direction: "incoming" as const })),
    ...(outgoing.data?.requests || []).map((request) => ({ ...request, direction: "outgoing" as const })),
  ];
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
          <PlayerSection title="Search results" players={searchPlayers} token={accessToken!} partyId={partyId} loading={search.isLoading} />
        ) : null}

        {pendingRequests.length ? <RequestSection requests={pendingRequests} token={accessToken!} partyId={partyId} /> : null}
        {friends.isLoading ? <AsyncState status="loading" message="Loading friends" className="mt-5" /> : null}
        {!friends.isLoading && !allFriends.length ? <AsyncState status="empty" message="No friends (yet!)" className="mt-5" /> : null}
        {offlineFriends.length ? <PlayerSection title="Offline" players={offlineFriends} token={accessToken!} partyId={partyId} /> : null}
        {onlineFriends.length ? <PlayerSection title="Online" players={onlineFriends} token={accessToken!} partyId={partyId} /> : null}
        {(recent.data?.players.length || 0) > 0 ? (
          <PlayerSection
            title="Recently played"
            players={(recent.data?.players || []).slice(0, 3)}
            token={accessToken!}
            partyId={partyId}
          />
        ) : null}
      </AppPanel>
    </div>
  );
}

function PlayerSection({
  title,
  players,
  token,
  partyId,
  loading,
  empty,
}: {
  title: string;
  players: CompactPlayer[];
  token: string;
  partyId?: string;
  loading?: boolean;
  empty?: string;
}) {
  return (
    <section className="mt-5">
      <SectionHeader title={title} className="mb-3" />
      <InsetList>
        {players.map((player) => (
          <CompactPlayerRow
            key={player.userId}
            player={player}
            actions={<RelationshipActions accessToken={token} player={player} partyId={partyId} />}
          />
        ))}
        {!players.length && !loading && empty ? <AsyncState status="empty" message={empty} /> : null}
        {loading ? <AsyncState status="loading" message="Loading players" /> : null}
      </InsetList>
    </section>
  );
}

function RequestSection({
  requests,
  token,
  partyId,
}: {
  requests: Array<{ id: string; player: CompactPlayer; direction: "incoming" | "outgoing" }>;
  token: string;
  partyId?: string;
}) {
  return (
    <section className="mt-5">
      <SectionHeader title="Pending requests" className="mb-3" />
      <InsetList>
        {requests.map((request) => (
          <CompactPlayerRow
            key={request.id}
            player={{ ...request.player, requestId: request.id }}
            meta={<span className="text-label text-content-secondary">{request.direction === "incoming" ? "Incoming" : "Sent"}</span>}
            actions={<RelationshipActions accessToken={token} player={{ ...request.player, requestId: request.id }} partyId={partyId} />}
          />
        ))}
      </InsetList>
    </section>
  );
}
