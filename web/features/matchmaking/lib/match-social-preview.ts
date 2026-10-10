import { isRatedKind, type Snapshot } from "../../game/model/types";
import type { MatchViewStatus } from "./queue-client";
import { toPublicEntityId } from "../../../lib/entity-id";
import {
  SOCIAL_ICON_IMAGE,
  type SocialPreview,
} from "../../../lib/social-preview";
import { getTeamPresentation } from "../../../lib/team-presentation";

const GENERIC_MATCH: SocialPreview = {
  title: "GeoDuels | Match",
  description: "View this GeoDuels match.",
  canonicalPath: "/",
  robots: "noindex,nofollow",
  ...SOCIAL_ICON_IMAGE,
};

const RULESET_LABELS: Record<string, string> = {
  moving: "Moving",
  no_move: "No Move",
  nmpz: "NMPZ",
};

export function buildMatchSocialPreview(
  snapshot: Snapshot | null,
  status: MatchViewStatus,
  routeId: string,
): SocialPreview {
  const canonicalPath = routeId
    ? `/match/${encodeURIComponent(toPublicEntityId(routeId))}`
    : "/";
  if (status === "starting" || status === "live") {
    return {
      ...GENERIC_MATCH,
      canonicalPath,
      description: "This GeoDuels match is live.",
    };
  }
  if (status !== "ended" || !snapshot) {
    return { ...GENERIC_MATCH, canonicalPath };
  }

  const { title, description } = matchCopy(snapshot);
  return {
    title,
    description,
    canonicalPath,
    robots: "noindex,nofollow",
    ...SOCIAL_ICON_IMAGE,
  };
}

function matchCopy(snapshot: Snapshot) {
  const mapName = snapshot.config?.mapName?.trim() || "";
  const ruleset = snapshot.config?.ruleset
    ? RULESET_LABELS[snapshot.config.ruleset] || ""
    : "";
  const ranked =
    snapshot.mode === "duel" || snapshot.mode === "team_duel"
      ? isRatedKind(snapshot.kind)
        ? "Ranked"
        : "Unranked"
      : "";
  const suffix = [mapName, ruleset, ranked].filter(Boolean);

  if (snapshot.mode === "team_duel") {
    const teams = Object.values(snapshot.teams || {}).sort(
      (a, b) => (b.hp ?? 0) - (a.hp ?? 0),
    );
    if (teams.length >= 2) {
      const a = teamLabel(teams[0].teamId, teams[0].name);
      const b = teamLabel(teams[1].teamId, teams[1].name);
      return {
        title: `${a} vs ${b} | GeoDuels`,
        description: joinParts([
          `${a} ${teams[0].hp ?? 0} HP`,
          `${b} ${teams[1].hp ?? 0} HP`,
          ...suffix,
        ]),
      };
    }
  }

  const players = Object.values(snapshot.players || {}).sort(
    (a, b) =>
      (b.hp ?? 0) - (a.hp ?? 0) || (b.totalScore ?? 0) - (a.totalScore ?? 0),
  );

  if (snapshot.mode === "singleplayer" && players[0]) {
    const score = (players[0].totalScore ?? 0).toLocaleString("en");
    return {
      title: `${players[0].displayName} | GeoDuels`,
      description: joinParts([`${score} points`, ...suffix, "Practice"]),
    };
  }

  if (players.length >= 2 && snapshot.mode !== "free_for_all") {
    const a = players[0];
    const b = players[1];
    return {
      title: `${a.displayName} vs ${b.displayName} | GeoDuels`,
      description: joinParts([
        `${a.displayName} ${a.hp} HP`,
        `${b.displayName} ${b.hp} HP`,
        ...suffix,
      ]),
    };
  }

  if (players.length > 0) {
    const standings = players
      .slice(0, 3)
      .map((player) => `${player.displayName} ${player.hp} HP`);
    return {
      title: `${players[0].displayName} | GeoDuels`,
      description: joinParts([...standings, ...suffix]),
    };
  }

  return {
    title: "GeoDuels | Match",
    description:
      joinParts(["Finished match", ...suffix]) || "Finished GeoDuels match.",
  };
}

function teamLabel(teamId?: string, name?: string) {
  return getTeamPresentation(teamId, name).name;
}

function joinParts(parts: string[]) {
  return parts.filter(Boolean).join(" · ");
}
