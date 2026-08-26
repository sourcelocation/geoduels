import type {
  MatchSidesView,
  ParticipantIdentityView,
  PlayerIdentityView,
} from "../components/overlays/participant-types";
import type {
  RatingDeltaPreview,
  Snapshot,
  SnapshotPlayer,
} from "./types";
import { getTeamPresentation } from "../../../lib/team-presentation";

type FallbackSelf = {
  id: string;
  name: string;
  avatarUrl?: string;
  avatarFallback: string;
  isAdmin?: boolean;
  isGuest?: boolean;
  selectedBadge?: SnapshotPlayer["selectedBadge"];
  rating?: number;
};

type CompetitiveDetails = {
  selfRatingDelta?: number;
  opponentRatingDelta?: number;
  selfRatingPreview?: RatingDeltaPreview;
  opponentRatingPreview?: RatingDeltaPreview;
};

export type DerivedMatchSides = {
  sides: MatchSidesView;
  playersById: Record<string, PlayerIdentityView>;
  selfPlayerId: string;
  opponentPlayerId: string;
  selfTeamId?: string;
  opponentTeamId?: string;
};

function avatarFallback(player: SnapshotPlayer | undefined, fallback: string) {
  if (player?.isGuest) return "?";
  return (player?.displayName || player?.userId || fallback)
    .slice(0, 1)
    .toUpperCase();
}

function playerIdentity(
  id: string,
  player: SnapshotPlayer | undefined,
  fallback?: FallbackSelf,
): PlayerIdentityView {
  return {
    kind: "player",
    id,
    name:
      player?.displayName ||
      fallback?.name ||
      player?.userId ||
      fallback?.id ||
      (id === "opponent" ? "Opponent" : id || "Player"),
    avatarUrl: player?.avatarUrl || fallback?.avatarUrl,
    avatarFallback:
      fallback?.avatarFallback || avatarFallback(player, "P"),
    selectedBadge: player?.selectedBadge || fallback?.selectedBadge || null,
    isAdmin: player?.isAdmin ?? fallback?.isAdmin,
    isGuest: player?.isGuest ?? fallback?.isGuest,
    rating: player?.mmr ?? fallback?.rating,
    disconnected: !!player?.disconnected,
  };
}

function sideConnection(members: PlayerIdentityView[]) {
  if (!members.length || members.every((member) => !member.disconnected)) {
    return "connected" as const;
  }
  if (members.every((member) => member.disconnected)) {
    return "disconnected" as const;
  }
  return "degraded" as const;
}

export function deriveMatchSides(params: {
  snapshot: Snapshot | null;
  selfUserId: string;
  fallbackSelf: FallbackSelf;
  competitive?: CompetitiveDetails;
}): DerivedMatchSides {
  const { snapshot, selfUserId, fallbackSelf, competitive = {} } = params;
  const playerEntries = Object.entries(snapshot?.players || {});
  const playersById: Record<string, PlayerIdentityView> = {};
  for (const [id, player] of playerEntries) {
    playersById[id] = playerIdentity(
      id,
      player,
      id === selfUserId ? fallbackSelf : undefined,
    );
  }

  const selfPlayerId =
    (selfUserId && snapshot?.players?.[selfUserId] ? selfUserId : "") ||
    playerEntries[0]?.[0] ||
    fallbackSelf.id ||
    "self";
  if (!playersById[selfPlayerId]) {
    playersById[selfPlayerId] = playerIdentity(
      selfPlayerId,
      undefined,
      fallbackSelf,
    );
  }

  if (snapshot?.mode === "team_duel") {
    const selfTeamId = snapshot.players[selfPlayerId]?.teamId || "a";
    const opponentTeamId =
      Object.keys(snapshot.teams || {}).find((id) => id !== selfTeamId) ||
      (selfTeamId === "a" ? "b" : "a");
    const selfTeam = snapshot.teams?.[selfTeamId];
    const opponentTeam = snapshot.teams?.[opponentTeamId];
    const selfMembers = Object.entries(snapshot.players)
      .filter(([, player]) => player.teamId === selfTeamId)
      .map(([id]) => playersById[id]);
    const opponentMembers = Object.entries(snapshot.players)
      .filter(([, player]) => player.teamId === opponentTeamId)
      .map(([id]) => playersById[id]);
    const selfPresentation = getTeamPresentation(
      selfTeamId,
      selfTeam?.name,
    );
    const opponentPresentation = getTeamPresentation(
      opponentTeamId,
      opponentTeam?.name,
    );
    const selfParticipant: ParticipantIdentityView = {
      kind: "team",
      id: selfTeamId,
      name: selfPresentation.name,
      avatarFallback: selfPresentation.fallback,
      avatarColor: selfPresentation.color,
      members: selfMembers,
    };
    const opponentParticipant: ParticipantIdentityView = {
      kind: "team",
      id: opponentTeamId,
      name: opponentPresentation.name,
      avatarFallback: opponentPresentation.fallback,
      avatarColor: opponentPresentation.color,
      members: opponentMembers,
    };
    return {
      sides: {
        self: {
          id: selfTeamId,
          participant: selfParticipant,
          hp: selfTeam?.hp ?? snapshot.players[selfPlayerId]?.hp ?? 0,
          connection: sideConnection(selfMembers),
        },
        opponent: {
          id: opponentTeamId,
          participant: opponentParticipant,
          hp:
            opponentTeam?.hp ??
            snapshot.players[opponentMembers[0]?.id || ""]?.hp ??
            0,
          connection: sideConnection(opponentMembers),
        },
      },
      playersById,
      selfPlayerId,
      opponentPlayerId: opponentMembers[0]?.id || "",
      selfTeamId,
      opponentTeamId,
    };
  }

  const opponentPlayerId =
    playerEntries.find(([id]) => id !== selfPlayerId)?.[0] || "opponent";
  if (!playersById[opponentPlayerId]) {
    playersById[opponentPlayerId] = playerIdentity(
      opponentPlayerId,
      undefined,
    );
  }
  const selfParticipant = {
    ...playersById[selfPlayerId],
    ratingDelta: competitive.selfRatingDelta,
    ratingPreview: competitive.selfRatingPreview,
  };
  const opponentParticipant = {
    ...playersById[opponentPlayerId],
    ratingDelta: competitive.opponentRatingDelta,
    ratingPreview: competitive.opponentRatingPreview,
  };
  return {
    sides: {
      self: {
        id: selfPlayerId,
        participant: selfParticipant,
        hp: snapshot?.players?.[selfPlayerId]?.hp ?? 0,
        connection: selfParticipant.disconnected
          ? "disconnected"
          : "connected",
      },
      opponent: {
        id: opponentPlayerId,
        participant: opponentParticipant,
        hp: snapshot?.players?.[opponentPlayerId]?.hp ?? 0,
        connection: opponentParticipant.disconnected
          ? "disconnected"
          : "connected",
      },
    },
    playersById,
    selfPlayerId,
    opponentPlayerId,
  };
}

export function withRoundSideResults(
  sides: MatchSidesView,
  round: Snapshot["lastRoundResult"],
): MatchSidesView {
  if (!round) return sides;
  const resolve = (side: MatchSidesView["self"]) => {
    const result =
      side.participant.kind === "team"
        ? round.teams?.[side.id]
        : round.players[side.id];
    return {
      ...side,
      hp: result?.hpAfterRound ?? side.hp,
      score: result?.score,
      distanceKm: result?.distanceKm,
    };
  };
  return {
    self: resolve(sides.self),
    opponent: resolve(sides.opponent),
  };
}
