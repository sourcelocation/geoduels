import { useEffect, useState } from "react";
import LobbyScreen, { type LobbyContentRoute } from "../../../components/ui/LobbyScreen";
import type {
  HomeActions,
  HomeAuthView,
  HomeLobbyView,
  HomeViewModel,
} from "../model/types";

type HomePageLobbyProps = {
  contentRoute?: LobbyContentRoute;
  mapId?: string;
  auth: HomeAuthView;
  lobby: HomeLobbyView;
  meta: HomeViewModel["meta"];
  actions: Pick<
    HomeActions,
    | "joinQueue"
    | "startSingleplayer"
    | "cancelQueue"
    | "createParty"
    | "joinParty"
    | "leaveParty"
    | "kickPartyMember"
    | "transferPartyOwner"
    | "startParty"
    | "updatePartySettings"
    | "switchPartyTeam"
    | "devLogin"
    | "triggerGoogleSignIn"
    | "triggerDiscordSignIn"
    | "loadLeaderboard"
    | "startSupportDonation"
  >;
};

export default function HomePageLobby({
  contentRoute = "play",
  mapId = "",
  auth,
  lobby,
  meta,
  actions,
}: HomePageLobbyProps) {
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  if (lobby.inGame) {
    return null;
  }

  return (
    <LobbyScreen
      contentRoute={contentRoute}
      mapId={mapId}
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
      onlinePlayers={lobby.onlinePlayers}
      maintenance={lobby.maintenance}
      googleClientId={
        mounted && auth.googleSignInEnabled ? auth.googleClientId : ""
      }
      discordClientId={
        mounted && auth.discordSignInEnabled ? auth.discordClientId : ""
      }
      appVersion={meta.appVersion}
      isAdmin={auth.isAdmin}
      isModerator={auth.isModerator}
      changelogEyebrow={lobby.changelogEyebrow}
      changelogTitle={lobby.changelogTitle}
      changelogMarkdown={lobby.changelogMarkdown}
      changelogSlug={lobby.changelogSlug}
      changelogUpdatedAt={lobby.changelogUpdatedAt}
      devLogin={actions.devLogin}
      onGoogleSignIn={actions.triggerGoogleSignIn}
      onDiscordSignIn={actions.triggerDiscordSignIn || actions.triggerGoogleSignIn}
      onBrowseLeaderboard={actions.loadLeaderboard}
      authLoading={auth.authLoading}
      authError={auth.authError}
      nicknameSaving={auth.nicknameSaving}
      onSupportDonation={actions.startSupportDonation}
    />
  );
}
