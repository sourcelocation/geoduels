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
  requestMe,
} from "../../auth/lib/auth-client";
import { fetchLobbyStatus } from "../../matchmaking/lib/queue-client";

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
  const [onlinePlayersEnabled, setOnlinePlayersEnabled] = useState(false);

  useEffect(() => {
    if (!enabled || typeof document === "undefined") {
      setOnlinePlayersEnabled(false);
      return;
    }
    const syncVisibility = () => {
      setOnlinePlayersEnabled(document.visibilityState === "visible");
    };
    syncVisibility();
    document.addEventListener("visibilitychange", syncVisibility);
    return () =>
      document.removeEventListener("visibilitychange", syncVisibility);
  }, [enabled]);

  const onlinePlayersQuery = useQuery({
    queryKey: ["queue-online"],
    queryFn: async () => fetchLobbyStatus(config),
    enabled: onlinePlayersEnabled,
    refetchInterval: 10_000,
    refetchOnMount: false,
    staleTime: 30_000,
  });

  const profileQuery = useQuery({
    queryKey: ["me", auth.userId, auth.accessToken],
    enabled:
      enabled &&
      !!auth.userId &&
      !!auth.accessToken &&
      !auth.nicknameRequired,
    queryFn: async () => {
      const session = await sessionController.ensureFreshSession();
      if (!session) {
        sessionController.clearAuthSession(
          "Session expired. Please sign in again.",
        );
        throw new Error("Session expired. Please sign in again.");
      }
      let resp = await requestMe(config, session.accessToken);
      if (resp.status === 401 || resp.status === 403) {
        const refreshed = await sessionController.ensureFreshSession(60_000, {
          forceRefresh: true,
        });
        if (!refreshed) {
          sessionController.clearAuthSession(
            "Session expired. Please sign in again.",
          );
          throw new Error("Session expired. Please sign in again.");
        }
        resp = await requestMe(config, refreshed.accessToken);
      }
      if (!resp.ok) {
        throw new Error("Failed to load profile");
      }
      return resp.json();
    },
    refetchOnMount: "always",
    staleTime: 60_000,
  });

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
    if (!profileQuery.data) return;
    sessionController.applyProfileSnapshot(profileQuery.data);
  }, [profileQuery.data, sessionController]);

  useEffect(() => {
    if (!leaderboardQuery.data) return;
    sessionController.applyLeaderboardSummary(leaderboardQuery.data);
  }, [leaderboardQuery.data, sessionController]);

  return {
    onlinePlayers:
      typeof onlinePlayersQuery.data?.onlinePlayers === "number"
        ? onlinePlayersQuery.data.onlinePlayers
        : null,
    maintenance: onlinePlayersQuery.data?.maintenance ?? null,
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
