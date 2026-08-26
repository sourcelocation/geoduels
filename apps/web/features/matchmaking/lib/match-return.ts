import { toPublicEntityId } from "../../../lib/entity-id";
import type { MatchReturnTarget } from "./queue-client";

export type MatchReturnDestination = {
  href: string;
  label: "Back to party" | "Back to map" | "Back to lobby";
};

/** Resolve a server-provided symbolic target into the web route and label. */
export function getMatchReturnDestination(
  target?: MatchReturnTarget,
): MatchReturnDestination {
  if (target?.kind === "party" && target.partyInviteCode) {
    return {
      href: `/party/${encodeURIComponent(target.partyInviteCode)}`,
      label: "Back to party",
    };
  }
  if (target?.kind === "map") {
    return {
      href: `/maps/${encodeURIComponent(toPublicEntityId(target.mapId))}`,
      label: "Back to map",
    };
  }
  return { href: "/", label: "Back to lobby" };
}
