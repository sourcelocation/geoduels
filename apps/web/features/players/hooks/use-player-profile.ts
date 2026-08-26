import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { getRuntimeConfig } from "../../../lib/runtime-config";
import { requestPlayerMatches, requestPlayerProfile } from "../lib/player-client";
import type { PublicPlayerProfile } from "../types";

export function usePlayerProfile(
  playerId: string,
  initialProfile?: PublicPlayerProfile,
  matchFilter: "all" | "ranked" = "all",
) {
  const config = getRuntimeConfig();
  const profileQuery = useQuery({
    queryKey: ["player-profile", playerId],
    enabled: !!playerId,
    queryFn: () => requestPlayerProfile(config, playerId),
    initialData: initialProfile,
    refetchOnMount: false,
    staleTime: 60_000,
  });
  const matchesQuery = useInfiniteQuery({
    queryKey: ["player-matches", playerId, matchFilter],
    enabled: !!playerId,
    initialPageParam: "",
    queryFn: ({ pageParam }) =>
      requestPlayerMatches(config, playerId, 20, pageParam, matchFilter),
    getNextPageParam: (lastPage) => lastPage.nextCursor || undefined,
    refetchOnMount: false,
    staleTime: 30_000,
  });
  return { profileQuery, matchesQuery };
}
