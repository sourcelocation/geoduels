import Head from "next/head";
import Link from "next/link";
import { useRouter } from "next/router";
import { useMemo } from "react";
import EndMatchOverlay from "../../components/ui/EndMatchOverlay";
import type { Snapshot } from "../../components/ui/types";
import { requestMatchReport } from "../../features/auth/lib/auth-client";
import { deriveMatchSides } from "../../features/game/model/match-sides";
import HomePageChatDock from "../../features/home/page/HomePageChatDock";
import HomePageGame from "../../features/home/page/HomePageGame";
import HomePageOverlays from "../../features/home/page/HomePageOverlays";
import { useHomeModel } from "../../features/home/model/useHomeModel";
import { useMatchRouteSession } from "../../features/matchmaking/hooks/use-match-route-session";
import { getRuntimeConfig } from "../../lib/runtime-config";
import { getSiteURL } from "../../lib/site";
import { getTeamPresentation } from "../../lib/team-presentation";
import {
  normalizeEntityRouteId,
  toPublicEntityId,
} from "../../lib/entity-id";
import type {
  MatchConfig,
  MatchSessionResponse,
} from "../../features/matchmaking/lib/queue-client";

export function normalizeRouteMatchId(
  raw: string | string[] | undefined,
  asPath: string,
) {
  if (typeof raw === "string") {
    const value = raw.trim();
    return /^\[[^/]+\]$/.test(value) ? "" : normalizeEntityRouteId(value);
  }
  const pathMatch = asPath.match(/^\/match\/([^?#/]+)/);
  if (pathMatch?.[1]) {
    try {
      const value = decodeURIComponent(pathMatch[1]).trim();
      return /^\[[^/]+\]$/.test(value) ? "" : normalizeEntityRouteId(value);
    } catch {
      const value = pathMatch[1].trim();
      return /^\[[^/]+\]$/.test(value) ? "" : normalizeEntityRouteId(value);
    }
  }
  return "";
}

function buildHistoryOverlay(
  snapshot: Snapshot,
  userId: string,
  displayName: string,
  userAvatar: string,
) {
  const playerIds = Object.keys(snapshot.players || {});
  const selfPlayer =
    snapshot.players[userId] || snapshot.players[playerIds[0] || ""];
  const mode = snapshot.mode || "duel";
  const derivedSides = deriveMatchSides({
    snapshot,
    selfUserId: selfPlayer?.userId || userId,
    fallbackSelf: {
      id: userId || "self",
      name: selfPlayer?.displayName || displayName || "You",
      avatarUrl: selfPlayer?.avatarUrl || userAvatar,
      avatarFallback: (
        selfPlayer?.displayName ||
        displayName ||
        "Y"
      ).slice(0, 1).toUpperCase(),
      isAdmin: selfPlayer?.isAdmin,
      isGuest: selfPlayer?.isGuest,
      selectedBadge: selfPlayer?.selectedBadge,
      rating: selfPlayer?.mmr,
    },
  });
  const roundResults =
    snapshot.roundResults && snapshot.roundResults.length > 0
      ? snapshot.roundResults
      : snapshot.lastRoundResult
        ? [snapshot.lastRoundResult]
        : [];
  const resultPlayerNames: Record<string, string | undefined> = {};
  const resultPlayerAvatars: Record<string, string | undefined> = {};
  const resultPlayerFallbacks: Record<string, string | undefined> = {};
  const resultPlayerBorderColors: Record<string, string | undefined> = {};
  Object.entries(snapshot.players || {}).forEach(([id, player]) => {
    resultPlayerNames[id] = player.displayName || player.userId;
    resultPlayerAvatars[id] = player.avatarUrl;
    resultPlayerBorderColors[id] =
      mode === "team_duel"
        ? getTeamPresentation(player.teamId).color
        : undefined;
    resultPlayerFallbacks[id] = (player.displayName || player.userId || "P")
      .slice(0, 1)
      .toUpperCase();
  });
  const selfHP = derivedSides.sides.self.hp ?? 0;
  const oppHP = derivedSides.sides.opponent.hp ?? 0;
  const outcome: "win" | "lose" | "draw" | undefined =
    mode === "singleplayer" || mode === "free_for_all"
      ? undefined
      : selfHP === oppHP
        ? "draw"
        : selfHP > oppHP
          ? "win"
          : "lose";

  return {
    mode,
    outcome,
    sides: derivedSides.sides,
    selfUserId: derivedSides.selfPlayerId,
    totalScore: selfPlayer?.totalScore || 0,
    roundResults,
    resultPlayerNames,
    resultPlayerAvatars,
    resultPlayerFallbacks,
    resultPlayerBorderColors,
    participantsById: derivedSides.playersById,
  };
}

function matchSourcePartyInviteCode(
  response: MatchSessionResponse | null,
): string {
  if (!response) return "";
  if (
    response.status === "live_connectable" ||
    response.status === "history" ||
    response.status === "replaced"
  ) {
    return response.sourcePartyInviteCode || "";
  }
  return "";
}

export default function MatchPage() {
  const router = useRouter();
  const routeMatchId = router.isReady
    ? normalizeRouteMatchId(router.query.id, router.asPath)
    : "";
  const model = useHomeModel({
    routeMatchId: routeMatchId || null,
    routeContext: "match",
  });
  const config = getRuntimeConfig();
  const routeSession = useMatchRouteSession(routeMatchId || null);
  const siteURL = getSiteURL();
  const canonicalURL = routeMatchId
    ? `${siteURL}/match/${encodeURIComponent(toPublicEntityId(routeMatchId))}`
    : `${siteURL}/`;
  const sourcePartyInviteCode =
    model.view.meta.sourcePartyInviteCode ||
    matchSourcePartyInviteCode(routeSession.replacement) ||
    "";
  const backHref = sourcePartyInviteCode
    ? `/party/${encodeURIComponent(sourcePartyInviteCode)}`
    : "/";
  const backLabel = sourcePartyInviteCode
    ? "Back to party"
    : "Back to lobby";
  const handleLeaveToParty = () => {
    model.actions.leaveGame();
    void router.push(backHref);
  };
  const handlePlayAgain = async (matchConfig?: MatchConfig) => {
    const nextMatchId = await model.actions.startSingleplayer(matchConfig);
    if (nextMatchId) {
      void router.push(
        `/match/${encodeURIComponent(toPublicEntityId(nextMatchId))}`,
      );
    }
    return nextMatchId;
  };
  const handleHistoryReport = async (
    reportedUserId: string,
    category = "cheating",
    reason = "",
  ) => {
    if (!model.view.auth.accessToken || !routeMatchId) {
      throw new Error("Report unavailable");
    }
    await requestMatchReport(
      config,
      model.view.auth.accessToken,
      routeMatchId,
      reportedUserId,
      category,
      reason,
    );
  };

  const historyOverlay = useMemo(
    () =>
      routeSession.historySnapshot
        ? buildHistoryOverlay(
            routeSession.historySnapshot,
            model.view.auth.userId,
            model.view.auth.displayName,
            model.view.auth.userAvatar,
          )
        : null,
    [
      routeSession.historySnapshot,
      model.view.auth.displayName,
      model.view.auth.userAvatar,
      model.view.auth.userId,
    ],
  );

  const loadingLabel = useMemo(() => {
    switch (routeSession.status) {
      case "bootstrapping_auth":
        return "Restoring session...";
      case "resolving":
        return "Reconnecting to match...";
      case "awaiting_first_snapshot":
        return "Joining live match...";
      case "replaced":
        return "This match was replaced";
      case "forbidden":
        return "Sign in to view this match";
      case "missing":
        return "Match unavailable";
      default:
        return "Loading match...";
    }
  }, [routeSession.status]);
  const replacementMatchId =
    routeSession.replacement?.status === "replaced"
      ? routeSession.replacement.replacementMatchId
      : "";

  return (
    <>
      <Head>
        <title>GeoDuels | Match</title>
        <meta name="robots" content="noindex,nofollow" />
        <link rel="canonical" href={canonicalURL} />
      </Head>
      <main className="relative min-h-screen overflow-hidden bg-[#08111b] text-ink">
        <HomePageOverlays
          auth={model.view.auth}
          overlays={model.view.overlays}
          actions={{
            ...model.actions,
            leaveGame: handleLeaveToParty,
            startSingleplayer: handlePlayAgain,
          }}
        />
        <HomePageGame
          game={model.view.game}
          maxHP={model.view.meta.maxHP}
          actions={{ ...model.actions, leaveGame: handleLeaveToParty }}
        />
        <HomePageChatDock chat={model.view.chat} actions={model.actions} />
        {!model.view.game.inGame &&
          !model.view.overlays.endMatch.open &&
          historyOverlay && (
            <EndMatchOverlay
              onLeaveGame={handleLeaveToParty}
              backLabel={backLabel}
              mode={historyOverlay.mode}
              outcome={historyOverlay.outcome}
              sides={historyOverlay.sides}
              selfUserId={historyOverlay.selfUserId}
              totalScore={historyOverlay.totalScore}
              roundResults={historyOverlay.roundResults}
              resultPlayerNames={historyOverlay.resultPlayerNames}
              resultPlayerAvatars={historyOverlay.resultPlayerAvatars}
              resultPlayerFallbacks={historyOverlay.resultPlayerFallbacks}
              resultPlayerBorderColors={historyOverlay.resultPlayerBorderColors}
              participantsById={historyOverlay.participantsById}
              onReportPlayer={handleHistoryReport}
              onPlayAgain={
                historyOverlay.mode === "singleplayer"
                  ? () => handlePlayAgain(routeSession.historySnapshot?.config)
                  : undefined
              }
              asPage
            />
          )}
        {!model.view.game.inGame &&
          !model.view.overlays.endMatch.open &&
          !historyOverlay && (
            <div className="flex min-h-screen items-center justify-center p-6">
              <div className="w-full max-w-md rounded-[28px] border border-white/10 bg-white/5 p-8 text-center text-white shadow-[0_24px_60px_rgba(0,0,0,0.38)] backdrop-blur-xl">
                {routeSession.status === "bootstrapping_auth" ||
                routeSession.status === "resolving" ||
                routeSession.status === "awaiting_first_snapshot" ? (
                  <div className="flex flex-col items-center py-6">
                    <div className="h-10 w-10 animate-spin rounded-full border-2 border-white/15 border-t-[#7fb3d8]" />
                  </div>
                ) : (
                  <p className="text-[12px] font-bold uppercase tracking-[0.16em] text-[#7fb3d8]">
                    Match Session
                  </p>
                )}
                <h1 className="mt-3 text-[28px] font-black tracking-tight">
                  {loadingLabel}
                </h1>
                {routeSession.status === "replaced" &&
                routeSession.replacement?.status === "replaced" &&
                routeSession.replacement.replacement ? (
                  <button
                    type="button"
                    onClick={() =>
                      void router.replace(
                        `/match/${encodeURIComponent(toPublicEntityId(replacementMatchId))}`,
                      )
                    }
                    className="mt-6 inline-flex rounded-full border border-[#2ad18f]/40 bg-[#2ad18f]/10 px-5 py-2.5 text-[12px] font-extrabold uppercase tracking-[0.1em] text-white transition hover:bg-[#2ad18f]/20"
                  >
                    Resume Current Match
                  </button>
                ) : null}
                <Link
                  href={backHref}
                  className="mt-4 inline-flex rounded-full border border-white/10 bg-white/10 px-5 py-2.5 text-[12px] font-extrabold uppercase tracking-[0.1em] text-white transition hover:bg-white/15"
                >
                  {backLabel}
                </Link>
              </div>
            </div>
          )}
      </main>
    </>
  );
}
