import React from "react";
import type { PlayerBadgeInfo } from "../../players/components/PlayerBadge";
import type { LeaderboardSummary } from "../../auth/controllers/session-controller";
import type { PartyTeamId, PartyMode } from "../lib/party-client";
import type {
  MaintenanceStatus,
  MatchConfig,
  MatchReturnTarget,
  QueueVariant,
} from "../../matchmaking/lib/queue-client";
import type { LobbyContentRoute } from "../lib/lobby-ui";
import { PlayPanel } from "./PlayPanel";
import { PartyPanel } from "./PartyPanel";
import { LeaderboardPanel } from "./LeaderboardPanel";
import {
  DonateCard,
  InvitePartyCard,
  LegalFooter,
  NewsPanel,
  PartyErrorNotice,
  SocialLinksCard,
} from "./LobbyShellPieces";
import {
  MaintenanceBanner,
  MaintenanceOverlay,
} from "./MaintenanceNotice";
import { MapRouteSurface } from "./maps/MapRouteSurfaces";
import { FriendsDashboard } from "../../social/components/FriendsDashboard";
import { LobbyScreenView } from "./LobbyScreenView";
import { LobbyScreenModals } from "./LobbyScreenModals";
import { useLobbyScreenState, type LobbyPartyView } from "../hooks/useLobbyScreenState";
import { useMapList } from "../../maps/lib/map-hooks";
import { getRuntimeConfig } from "../../../lib/runtime-config";
import { selectFeaturedOfficialMaps } from "../lib/featured-maps";

export type { LobbyContentRoute } from "../lib/lobby-ui";

type PartyView = LobbyPartyView;

type Props = {
  contentRoute?: LobbyContentRoute;
  mapId?: string;
  routeLoading?: boolean;
  userId: string;
  accessToken?: string;
  userEmail: string;
  displayName: string;
  userAvatar?: string;
  isGuest: boolean;
  authMigrationRequired?: boolean;
  selectedBadge?: PlayerBadgeInfo | null;
  connected: boolean;
  mmr: number;
  leaderboard: LeaderboardSummary | null;
  leaderboardLoading: boolean;
  status: string;
  queueStartedAt: number | null;
  joinQueue: (queues?: QueueVariant[]) => void;
  startSingleplayer: (config?: MatchConfig, returnTarget?: MatchReturnTarget) => Promise<string>;
  clearSingleplayerError: () => void;
  cancelQueue: () => void;
  party?: PartyView;
  createParty?: (mode?: PartyMode, config?: MatchConfig) => Promise<boolean>;
  joinParty?: (inviteCode?: string) => Promise<boolean>;
  leaveParty?: () => Promise<void>;
  kickPartyMember?: (userId: string) => Promise<void>;
  transferPartyOwner?: (userId: string) => Promise<void>;
  startParty?: () => Promise<void>;
  updatePartySettings?: (
    config: MatchConfig,
    mode?: PartyMode,
  ) => Promise<void>;
  switchPartyTeam?: (teamId: PartyTeamId) => Promise<void>;
  queueError: string;
  singleplayerError: string;
  onlinePlayers: number;
  maintenance: MaintenanceStatus | null;
  appVersion: string;
  isAdmin: boolean;
  isModerator?: boolean;
  changelogEyebrow: string;
  changelogTitle: string;
  changelogMarkdown: string;
  changelogSlug: string;
  changelogUpdatedAt: string;
  onBrowseLeaderboard: () => void;
  authLoading: boolean;
  authError: string;
  nicknameSaving: boolean;
  onSupportDonation?: () => Promise<void>;
};

const defaultParty: PartyView = {
  status: "idle",
  snapshot: null,
  inviteCode: "",
  isOwner: false,
  busy: false,
  error: "",
};

export default function LobbyScreen({
  contentRoute = "play",
  mapId = "",
  routeLoading = false,
  userId,
  accessToken = "",
  userEmail,
  displayName,
  userAvatar,
  isGuest,
  authMigrationRequired = false,
  selectedBadge = null,
  connected,
  mmr,
  leaderboard,
  leaderboardLoading,
  status,
  queueStartedAt,
  joinQueue,
  startSingleplayer,
  clearSingleplayerError,
  party = defaultParty,
  createParty = async () => false,
  joinParty = async () => false,
  leaveParty = async () => {},
  kickPartyMember = async () => {},
  transferPartyOwner = async () => {},
  startParty = async () => {},
  updatePartySettings = async () => {},
  switchPartyTeam = async () => {},
  queueError,
  singleplayerError,
  onBrowseLeaderboard,
  authLoading,
  authError,
  nicknameSaving,
  onSupportDonation = async () => {},
  maintenance,
  onlinePlayers,
  appVersion,
  isAdmin,
  isModerator = false,
  changelogEyebrow,
  changelogTitle,
  changelogMarkdown,
  changelogSlug,
  changelogUpdatedAt,
}: Props) {
  const {
    openModal, setOpenModal, extensionStatus, extensionAvailable, duel, setDuel,
    singleplayer, setSingleplayer, inviteCopied, inviteCodeInput, setInviteCodeInput,
    mapPickerOpen, setMapPickerOpen, currentNavRoute, partyPanelState, hasActiveParty,
    isQueueing, isSingleplayerLoading, primaryButtonLabel,
    userAvatarFallback, maintenanceIsWarning,
    maintenanceIsActive, queuePaused, playPaused, maintenanceMessage,
    maintenanceAlertKey, warningCountdown, activeEta, authBusy, duelDisabled, singleplayerDisabled,
    onDuelsPlay, startDuelQueue, startSingleplayerFromModal, openSingleplayerModal,
    dismissedMaintenanceAlertKey, setDismissedMaintenanceAlertKey,
  } = useLobbyScreenState({
    contentRoute, onBrowseLeaderboard, maintenance, status, userId, isGuest, party,
    updatePartySettings, queueStartedAt, connected, queueError, userEmail, displayName,
    authLoading, authMigrationRequired, nicknameSaving,
    joinQueue, startSingleplayer, clearSingleplayerError,
  });
  const canUploadCustomMaps = !!accessToken && !isGuest;
  const trendingMapsQuery = useMapList(
    getRuntimeConfig(),
    accessToken,
    userId,
    "official",
    "trending",
    "",
    { enabled: contentRoute === "play" },
  );
  const featuredOfficialMaps = React.useMemo(
    () => selectFeaturedOfficialMaps(trendingMapsQuery.data || [], 10),
    [trendingMapsQuery.data],
  );

  const newsPanel = (
    <NewsPanel
      changelogEyebrow={changelogEyebrow}
      changelogMarkdown={changelogMarkdown}
      changelogSlug={changelogSlug}
      changelogTitle={changelogTitle}
      changelogUpdatedAt={changelogUpdatedAt}
    />
  );
  const donateCard = <DonateCard onSupportDonation={onSupportDonation} />;
  const socialCard = <SocialLinksCard />;
  const partyConfig = partyPanelState.config;
  const savePartyConfig = partyPanelState.saveConfig;

  const mapPickerFlow =
    hasActiveParty && party.isOwner && party.snapshot?.state === "open";
  const mapRouteSurface =
    contentRoute === "maps" ||
    contentRoute === "map-details" ||
    contentRoute === "map-upload" ? (
      <MapRouteSurface
        accessToken={accessToken}
        canUploadCustomMaps={canUploadCustomMaps}
        contentRoute={contentRoute}
        createParty={createParty}
        displayName={displayName}
        isAdmin={isAdmin}
        isModerator={isModerator}
        mapId={mapId}
        mapPickerFlow={!!mapPickerFlow}
        partyActive={hasActiveParty}
        savePartyConfig={savePartyConfig}
        singleplayerDisabled={singleplayerDisabled}
        playMapSingleplayer={(map) =>
          openSingleplayerModal({ mapId: map.id, mapName: map.displayName })
        }
        userAvatar={userAvatar}
        userAvatarFallback={userAvatarFallback}
        userEmail={userEmail}
        userId={userId}
      />
    ) : null;

  const partyPanel = hasActiveParty ? (
    <PartyPanel
      inviteCopied={inviteCopied}
      kickPartyMember={kickPartyMember}
      leaveParty={leaveParty}
      party={party}
      setMapPickerOpen={setMapPickerOpen}
      startParty={startParty}
      state={partyPanelState}
      switchPartyTeam={switchPartyTeam}
      transferPartyOwner={transferPartyOwner}
      userId={userId}
    />
  ) : null;

  const maintenanceAlertDismissed =
    isAdmin && dismissedMaintenanceAlertKey === maintenanceAlertKey;
  const dismissMaintenanceAlert = isAdmin
    ? () => setDismissedMaintenanceAlertKey(maintenanceAlertKey)
    : undefined;
  const showMaintenanceBanner =
    maintenanceIsWarning && !maintenanceAlertDismissed;
  const maintenanceBanner = showMaintenanceBanner ? (
    <MaintenanceBanner
      message={maintenanceMessage}
      countdown={warningCountdown}
      onDismiss={dismissMaintenanceAlert}
    />
  ) : null;

  const maintenanceOverlay =
    maintenanceIsActive && !maintenanceAlertDismissed ? (
      <MaintenanceOverlay
        message={maintenanceMessage}
        eta={activeEta}
        onDismiss={dismissMaintenanceAlert}
      />
    ) : null;

  const legalCard = <LegalFooter appVersion={appVersion} />;

  const leaderboardPanel = (
    <LeaderboardPanel
      leaderboard={leaderboard}
      leaderboardLoading={leaderboardLoading}
      mmr={mmr}
      userId={userId}
    />
  );

  const invitePartyCard = (
    <InvitePartyCard
      disabled={
        party.busy ||
        isQueueing ||
        hasActiveParty ||
        authBusy ||
        playPaused ||
        maintenanceIsActive
      }
      onClick={() => setOpenModal("invite")}
    />
  );

  const partyErrorNotice =
    openModal !== "invite" ? <PartyErrorNotice message={party.error} /> : null;
  const showPartyPanel = hasActiveParty && contentRoute === "party";
  const playPanel = (
    <PlayPanel
      isSingleplayerLoading={isSingleplayerLoading}
      queueError={queueError}
      onDuelsPlay={onDuelsPlay}
      onSingleplayerPlay={() => openSingleplayerModal()}
      duelDisabled={duelDisabled}
      singleplayerDisabled={singleplayerDisabled}
      queuePaused={queuePaused}
      playPaused={playPaused}
      maintenanceIsActive={maintenanceIsActive}
      primaryButtonLabel={primaryButtonLabel}
      trendingMaps={featuredOfficialMaps}
      trendingMapsLoading={trendingMapsQuery.isLoading}
      changelogCard={newsPanel}
      donateCard={donateCard}
      socialCard={socialCard}
    />
  );

  const friendsDashboard = (
    <FriendsDashboard
      accessToken={accessToken}
      isGuest={isGuest}
      partyId={party.snapshot?.id}
      partyCard={invitePartyCard}
    />
  );

  const modalNodes = (
    <LobbyScreenModals
      openModal={openModal}
      setOpenModal={setOpenModal}
      inviteCodeInput={inviteCodeInput}
      setInviteCodeInput={setInviteCodeInput}
      partyBusy={party.busy}
      authLoading={authBusy}
      maintenanceIsActive={maintenanceIsActive}
      playPaused={playPaused}
      partyError={party.error}
      authError={authError}
      createParty={createParty}
      joinParty={joinParty}
      extensionAvailable={extensionAvailable}
      extensionStatus={extensionStatus}
      duel={duel}
      setDuel={setDuel}
      singleplayer={singleplayer}
      setSingleplayer={setSingleplayer}
      duelDisabled={duelDisabled}
      singleplayerDisabled={singleplayerDisabled}
      startDuelQueue={startDuelQueue}
      startSingleplayerFromModal={startSingleplayerFromModal}
      singleplayerError={singleplayerError}
      clearSingleplayerError={clearSingleplayerError}
      mapPickerOpen={mapPickerOpen}
      accessToken={accessToken}
      canUploadCustomMaps={canUploadCustomMaps}
      partyConfig={partyConfig}
      savePartyConfig={savePartyConfig}
      userId={userId}
      closeMapPicker={() => setMapPickerOpen(false)}
    />
  );

  return (
    <LobbyScreenView
      activeNavRoute={currentNavRoute}
      backgroundBlurred={showPartyPanel}
      contentClassName={showPartyPanel ? "overflow-hidden" : undefined}
      contentRoute={contentRoute}
      routeLoading={routeLoading}
      maintenanceBanner={maintenanceBanner}
      maintenanceOverlay={maintenanceOverlay}
      modalNodes={modalNodes}
      onlinePlayers={onlinePlayers}
      partyErrorNotice={partyErrorNotice}
      showPartyPanel={showPartyPanel}
      partyPanel={partyPanel}
      mapRouteSurface={mapRouteSurface}
      playPanel={playPanel}
      leaderboardPanel={leaderboardPanel}
      friendsDashboard={friendsDashboard}
      legalCard={legalCard}
    />
  );
}
