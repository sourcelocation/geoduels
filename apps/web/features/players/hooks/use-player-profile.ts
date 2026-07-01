import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { requestSession } from "../../auth/lib/auth-client";
import { getRuntimeConfig } from "../../../lib/runtime-config";
import { requestPlayerMatches, requestPlayerProfile } from "../lib/player-client";
import type { OptionalViewer } from "../types";
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

export function useOptionalViewer() {
  const config = getRuntimeConfig();
  return useQuery({
    queryKey: ["optional-viewer"],
    queryFn: async (): Promise<OptionalViewer | null> => {
      const session = await requestSession(config);
      if (!session?.user?.id || !session.accessToken) return null;
      let profile: PublicPlayerProfile | null = null;
      try {
        if (session.user.display_name) {
          profile = await requestPlayerProfile(config, session.user.display_name);
        }
      } catch {
        // The shell can still show the session identity if the public profile is unavailable.
      }
      return {
        userId: session.user.id,
        accessToken: session.accessToken,
        isGuest: !!session.user.isGuest,
        isAdmin: !!session.user.isAdmin,
        isModerator: !!session.user.isModerator,
        displayName:
          profile?.displayName ||
          session.user.display_name ||
          session.user.email ||
          "Player",
        avatarUrl: profile?.avatarUrl || session.user.avatar_url,
        mmr: profile?.mmr,
        selectedBadge: profile?.selectedBadge,
      };
    },
    refetchOnMount: false,
    staleTime: 60_000,
  });
}
