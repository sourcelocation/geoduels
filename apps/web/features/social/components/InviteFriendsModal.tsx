import AppModalShell from "../../../components/ui/AppModalShell";
import { AsyncState } from "../../../components/ui/patterns";
import { useFriendsPage } from "../hooks/useFriendsPage";
import type { CompactPlayer } from "../types";
import { InviteToPartyButton } from "./InviteToPartyButton";
import { PlayerListSection } from "./PlayerListSection";

export function InviteFriendsModal({
  accessToken,
  partyId,
  memberUserIds,
  onClose,
}: {
  accessToken: string;
  partyId: string;
  memberUserIds: readonly string[];
  onClose: () => void;
}) {
  const page = useFriendsPage(accessToken, true, partyId);
  const memberIds = new Set(memberUserIds);
  const available = (page.data?.friends || []).filter((player) => !memberIds.has(player.userId));
  const online = available.filter((player) => player.presenceStatus === "online");
  const offline = available.filter((player) => player.presenceStatus !== "online");
  const inviteActions = (player: CompactPlayer) => (
    <InviteToPartyButton
      accessToken={accessToken}
      partyId={partyId}
      userId={player.userId}
      displayName={player.displayName}
      partyInvite={player.partyInvite}
    />
  );

  return (
    <AppModalShell
      title="Invite friends"
      description="Send a party invite to someone on your friends list."
      onClose={onClose}
    >
      {page.isLoading ? <AsyncState status="loading" message="Loading friends" /> : null}
      {!page.isLoading && !available.length ? (
        <AsyncState status="empty" title="No friends to invite" message="Add friends first, or share the party code instead." />
      ) : null}
      {online.length ? <PlayerListSection title="Online" players={online} renderActions={inviteActions} /> : null}
      {offline.length ? <PlayerListSection title="Offline" players={offline} renderActions={inviteActions} /> : null}
    </AppModalShell>
  );
}
