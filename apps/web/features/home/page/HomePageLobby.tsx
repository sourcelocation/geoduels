import LobbyScreen, { type LobbyContentRoute } from "../../lobby/components/LobbyScreen";
import type {
  HomeActions,
  HomeAuthView,
  HomeLobbyView,
  HomeViewModel,
} from "../model/types";

type HomePageLobbyProps = {
  contentRoute?: LobbyContentRoute;
  mapId?: string;
  routeLoading?: boolean;
  auth: HomeAuthView;
  lobby: HomeLobbyView;
  meta: HomeViewModel["meta"];
  actions: Pick<
    HomeActions,
    | "joinQueue"
    | "startSingleplayer"
    | "clearSingleplayerError"
    | "cancelQueue"
    | "createParty"
    | "joinParty"
    | "leaveParty"
    | "kickPartyMember"
    | "transferPartyOwner"
    | "startParty"
    | "updatePartySettings"
    | "switchPartyTeam"
    | "loadLeaderboard"
    | "startSupportDonation"
  >;
};

export default function HomePageLobby({
  contentRoute = "play",
  mapId = "",
  routeLoading = false,
  auth,
  lobby,
  meta,
  actions,
}: HomePageLobbyProps) {
  if (lobby.inGame) {
    return null;
  }

  return (
    <LobbyScreen
      contentRoute={contentRoute}
      mapId={mapId}
      routeLoading={routeLoading}
      userId={auth.userId}
      accessToken={auth.accessToken}
      userEmail={auth.userEmail}
      displayName={auth.displayName}
      userAvatar={auth.userAvatar}
      isGuest={auth.isGuest}
      authMigrationRequired={!!auth.authMigrationRequired}
      selectedBadge={auth.selectedBadge}
      connected={lobby.connected}
      mmr={lobby.mmr}
      leaderboard={lobby.leaderboard}
      leaderboardLoading={lobby.leaderboardLoading}
      status={lobby.status}
      queueStartedAt={lobby.queueStartedAt}
      joinQueue={actions.joinQueue}
      startSingleplayer={actions.startSingleplayer}
      clearSingleplayerError={actions.clearSingleplayerError}
      cancelQueue={actions.cancelQueue}
      party={lobby.party}
      createParty={actions.createParty}
      joinParty={actions.joinParty}
      leaveParty={actions.leaveParty}
      kickPartyMember={actions.kickPartyMember}
      transferPartyOwner={actions.transferPartyOwner}
      startParty={actions.startParty}
      updatePartySettings={actions.updatePartySettings}
      switchPartyTeam={actions.switchPartyTeam}
      queueError={lobby.queueError}
      singleplayerError={lobby.singleplayerError}
      onlinePlayers={lobby.onlinePlayers}
      maintenance={lobby.maintenance}
      appVersion={meta.appVersion}
      isAdmin={auth.isAdmin}
      isModerator={auth.isModerator}
      changelogEyebrow={lobby.changelogEyebrow}
      changelogTitle={lobby.changelogTitle}
      changelogMarkdown={lobby.changelogMarkdown}
      changelogSlug={lobby.changelogSlug}
      changelogUpdatedAt={lobby.changelogUpdatedAt}
      onBrowseLeaderboard={actions.loadLeaderboard}
      authLoading={auth.authLoading}
      authError={auth.authError}
      nicknameSaving={auth.nicknameSaving}
      onSupportDonation={actions.startSupportDonation}
    />
  );
}
