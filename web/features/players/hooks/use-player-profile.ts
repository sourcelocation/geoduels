import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { requestPlayerMatches, requestPlayerProfile } from "../lib/player-client";
import type { PublicPlayerProfile } from "../types";

export function usePlayerProfile(
  playerId: string,
  initialProfile?: PublicPlayerProfile,
  matchFilter: "all" | "ranked" = "all",
) {
  const profileQuery = useQuery({
    queryKey: ["player-profile", playerId],
    enabled: !!playerId,
    queryFn: () => requestPlayerProfile(playerId),
    initialData: initialProfile,
    refetchOnMount: false,
    staleTime: 60_000,
  });
  const matchesQuery = useInfiniteQuery({
    queryKey: ["player-matches", playerId, matchFilter],
    enabled: !!playerId,
    initialPageParam: "",
    queryFn: ({ pageParam }) =>
      requestPlayerMatches(playerId, 20, pageParam, matchFilter),
    getNextPageParam: (lastPage) => lastPage.nextCursor || undefined,
    refetchOnMount: false,
    staleTime: 30_000,
  });
  return { profileQuery, matchesQuery };
}
