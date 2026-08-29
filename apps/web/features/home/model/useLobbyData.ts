import { useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import type { RuntimeConfig } from "../../../lib/runtime-config";
import type {
  SessionController,
  SessionState,
} from "../../auth/controllers/session-controller";
import {
  requestLeaderboard,
  requestLobbyChangelog,
} from "../../auth/lib/auth-client";
import { getAuthGateway } from "../../auth/auth-gateway";

type Params = {
  config: RuntimeConfig;
  sessionController: SessionController;
  auth: SessionState;
  enabled: boolean;
};

export function useLobbyData({
  config,
  sessionController,
  auth,
  enabled,
}: Params) {
  const [leaderboardEnabled, setLeaderboardEnabled] = useState(false);
  const globalState = getAuthGateway(config).getBootstrapPayload()?.global;

  const leaderboardQuery = useQuery({
    queryKey: ["leaderboard", auth.userId, auth.accessToken],
    enabled: enabled && leaderboardEnabled,
    queryFn: async () => {
      let accessToken: string | undefined;
      if (auth.userId && auth.accessToken && !auth.nicknameRequired) {
        const session = await sessionController.ensureFreshSession();
        if (session) {
          accessToken = session.accessToken;
        }
      }
      return requestLeaderboard(config, accessToken);
    },
    refetchOnMount: false,
    staleTime: 60_000,
  });

  const changelogQuery = useQuery({
    queryKey: ["lobby-changelog"],
    queryFn: async () => requestLobbyChangelog(config),
    enabled,
    refetchOnMount: false,
    staleTime: 5 * 60_000,
  });

  useEffect(() => {
    if (!leaderboardQuery.data) return;
    sessionController.applyLeaderboardSummary(leaderboardQuery.data);
  }, [leaderboardQuery.data, sessionController]);

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
