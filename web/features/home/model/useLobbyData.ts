import { leaderboardKey, normalizeLeaderboardSummary, requestLeaderboard } from "../../lobby/lib/leaderboard";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import type {
  SessionController,
  SessionState,
} from "../../auth/controllers/session-controller";
import {
  requestLobbyChangelog,
} from "../../auth/lib/auth-client";
import { authGateway } from "../../auth/auth-gateway";

type Params = {
  sessionController: SessionController;
  auth: SessionState;
  enabled: boolean;
};

export function useLobbyData({
  sessionController,
  auth,
  enabled,
}: Params) {
  const [leaderboardEnabled, setLeaderboardEnabled] = useState(false);
  const globalState = authGateway.getBootstrapPayload()?.global;

  const leaderboardQuery = useQuery({
    queryKey: leaderboardKey(auth.userId),
    enabled: enabled && leaderboardEnabled,
    queryFn: async () => {
      let accessToken: string | undefined;
      if (auth.userId && auth.accessToken && !auth.nicknameRequired) {
        const session = await sessionController.ensureFreshSession();
        if (session) {
          accessToken = session.accessToken;
        }
      }
      return requestLeaderboard(accessToken);
    },
    refetchOnMount: false,
    staleTime: 60_000,
  });

  const changelogQuery = useQuery({
    queryKey: ["lobby-changelog"],
    queryFn: async () => requestLobbyChangelog(),
    enabled,
    refetchOnMount: false,
    staleTime: 5 * 60_000,
  });

  return {
    onlinePlayers:
      typeof globalState?.onlinePlayers === "number"
        ? globalState.onlinePlayers
        : null,
    maintenance: globalState ? {
      phase: globalState.maintenance.phase,
      startsAt: globalState.maintenance.startsAt || "",
      endsAt: globalState.maintenance.endsAt || "",
      queuePaused: !!globalState.maintenance.queuePaused,
      playPaused: !!globalState.maintenance.playPaused,
      message: globalState.maintenance.message || "",
    } : null,
    leaderboard: normalizeLeaderboardSummary(leaderboardQuery.data),
    leaderboardLoading:
      leaderboardQuery.isLoading || leaderboardQuery.isFetching,
    changelogEyebrow:
      typeof changelogQuery.data?.eyebrow === "string"
        ? changelogQuery.data.eyebrow
        : "Latest News",
    changelogTitle:
      typeof changelogQuery.data?.title === "string"
        ? changelogQuery.data.title
        : "GeoDuels v1.1",
    changelogMarkdown:
      typeof changelogQuery.data?.markdown === "string"
        ? changelogQuery.data.markdown
        : "",
    changelogSlug:
      typeof changelogQuery.data?.slug === "string"
        ? changelogQuery.data.slug
        : "",
    changelogUpdatedAt:
      typeof changelogQuery.data?.updatedAt === "string"
        ? changelogQuery.data.updatedAt
        : "",
    loadLeaderboard: () => setLeaderboardEnabled(true),
  };
}
