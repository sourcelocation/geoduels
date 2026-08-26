import type { PartyRuntimeState } from "../../lobby/controllers/party-controller";
import type { MatchState } from "../../matchmaking/controllers/match-controller";

export function selectActiveChatConversationId(params: {
  userId: string;
  party: PartyRuntimeState;
  match: MatchState;
}) {
  const partySnapshot = params.party.snapshot;
  if (partySnapshot?.id && params.party.self?.userId === params.userId) {
    return `party:${partySnapshot.id}`;
  }
  if (params.match.sourcePartyId) {
    return `party:${params.match.sourcePartyId}`;
  }
  if (params.match.snapshot?.matchId && params.match.snapshot.mode !== "singleplayer") {
    return `match:${params.match.snapshot.matchId}`;
  }
  return "";
}
