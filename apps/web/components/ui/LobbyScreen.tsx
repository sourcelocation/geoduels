import React, { useState, useEffect } from "react";
import { motion, AnimatePresence } from "framer-motion";
import type { PlayerBadgeInfo } from "./PlayerBadge";
import type { LeaderboardSummary } from "../../features/auth/controllers/session-controller";
import type { PartySnapshot, PartyTeamId, PartyMode } from "../../features/lobby/lib/party-client";
import type { PartyRuntimeStatus } from "../../features/lobby/controllers/party-controller";
import type { MaintenanceStatus, MatchConfig, QueueVariant } from "../../features/matchmaking/lib/queue-client";
import {
  formatApproximateTime,
  formatQueueElapsed,
  formatRelativeDuration,
  parseTime,
  type LobbyContentRoute,
} from "../../features/lobby/lib/lobby-ui";
import { AppShell } from "../../features/app-shell/components/AppShell";
import { AppContentRail } from "../../features/app-shell/components/AppContentRail";
import type { AppNavRoute } from "../../features/app-shell/navigation";
import { PlayPanel } from "../../features/lobby/components/PlayPanel";
import { LobbyTutorialSection } from "../../features/lobby/components/LobbyTutorialSection";
import { DiscordProviderButton, GoogleProviderButton, SignInButton } from "../../features/lobby/components/LobbyAuthButtons";
import { PartyPanel } from "../../features/lobby/components/PartyPanel";
import { LeaderboardPanel } from "../../features/lobby/components/LeaderboardPanel";
import {
  DonateCard,
  InvitePartyCard,
  LegalFooter,
  LobbyUpdatesPanel,
  NewsPanel,
  PartyErrorNotice,
  SocialLinksCard,
} from "../../features/lobby/components/LobbyShellPieces";
import { MaintenanceBanner, MaintenanceOverlay } from "../../features/lobby/components/MaintenanceNotice";
import { MapPickerController, MapRouteSurface } from "../../features/lobby/components/maps/MapRouteSurfaces";
import { InviteModal } from "../../features/lobby/components/modals/InviteModal";
import { SignInModal } from "../../features/lobby/components/modals/SignInModal";
import { usePartyPanelState } from "../../features/lobby/hooks/usePartyPanelState";
import { usePlayPreferences } from "../../features/lobby/hooks/usePlayPreferences";
import { PlayLaunchModal } from "../../features/lobby/components/PlayLaunchModal";
import { useExtensionAvailability } from "../../features/browser-extension/hooks/use-extension-availability";

export type { LobbyContentRoute } from "../../features/lobby/lib/lobby-ui";

type PartyModal = "invite" | "signin" | "duel" | "singleplayer" | null;

type PartyView = {
  status: PartyRuntimeStatus;
  snapshot: PartySnapshot | null;
  inviteCode: string;
  isMember: boolean;
  isOwner: boolean;
  busy: boolean;
  error: string;
};

type Props = {
  contentRoute?: LobbyContentRoute;
  mapId?: string;
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
  startSingleplayer: (config?: MatchConfig) => void | Promise<string>;
  cancelQueue: () => void;
  party?: PartyView;
  createParty?: (mode?: PartyMode, config?: MatchConfig) => Promise<boolean>;
  joinParty?: (inviteCode?: string) => Promise<boolean>;
  leaveParty?: () => Promise<void>;
  kickPartyMember?: (userId: string) => Promise<void>;
  transferPartyOwner?: (userId: string) => Promise<void>;
  startParty?: () => Promise<void>;
  updatePartySettings?: (config: MatchConfig, mode?: PartyMode) => Promise<void>;
  switchPartyTeam?: (teamId: PartyTeamId) => Promise<void>;
  queueError: string;
  onlinePlayers: number;
  maintenance: MaintenanceStatus | null;
  googleClientId: string;
  discordClientId?: string;
  appVersion: string;
  isAdmin: boolean;
  isModerator?: boolean;
  changelogEyebrow: string;
  changelogTitle: string;
  changelogMarkdown: string;
  changelogSlug: string;
  changelogUpdatedAt: string;
  devLogin: () => void;
  onGoogleSignIn: () => void;
  onDiscordSignIn?: () => void;
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
  isMember: false,
  isOwner: false,
  busy: false,
  error: "",
};

const tabPanelMotion = {
  initial: {
    opacity: 0,
    y: 16,
    scale: 0.97,
  },
  animate: {
    opacity: 1,
    y: 0,
    scale: 1,
  },
  exit: {
    opacity: 0,
    y: 10,
    scale: 0.97,
  },
  transition: {
    duration: 0.22,
    ease: [0.16, 1, 0.3, 1] as const,
  },
};

export default function LobbyScreen({
  contentRoute = "play",
  mapId = "",
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
  cancelQueue,
  party = defaultParty,
  createParty = async () => false,
  joinParty = async () => false,
  leaveParty = async () => {},
  kickPartyMember = async () => { },
  transferPartyOwner = async () => { },
  startParty = async () => {},
  updatePartySettings = async () => {},
  switchPartyTeam = async () => {},
  queueError,
  googleClientId,
  discordClientId,
  devLogin,
  onGoogleSignIn,
  onDiscordSignIn = devLogin,
  onBrowseLeaderboard,
  authLoading,
  authError,
  nicknameSaving,
  onSupportDonation = async () => { },
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
  const [openModal, setOpenModal] = useState<PartyModal>(null);
  const extensionStatus = useExtensionAvailability();
  const extensionAvailable = extensionStatus.state === "ready";
  const { duel, setDuel, singleplayer, setSingleplayer } =
    usePlayPreferences(extensionStatus.state === "checking" ? null : extensionAvailable);
  const [inviteCopied, setInviteCopied] = useState(false);
  const [inviteCodeInput, setInviteCodeInput] = useState("");
  const [mapPickerOpen, setMapPickerOpen] = useState(false);
  const [nowMs, setNowMs] = useState(() => Date.now());
  const [dismissedMaintenanceAlertKey, setDismissedMaintenanceAlertKey] = useState("");
  const currentNavRoute: AppNavRoute =
    contentRoute === "map-details" || contentRoute === "map-upload"
      ? "maps"
      : contentRoute === "party"
        ? "play"
        : contentRoute;
  const canInteractWithMaps = !!accessToken && !isGuest;
  const canUploadCustomMaps = canInteractWithMaps;

  useEffect(() => {
    if (contentRoute === "top") {
      onBrowseLeaderboard();
    }
  }, [contentRoute, onBrowseLeaderboard]);

  useEffect(() => {
    if (!maintenance && status !== "queueing") {
      return;
    }
    setNowMs(Date.now());
    const timer = window.setInterval(() => setNowMs(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [maintenance, status]);

  const isQueueing = status === "queueing";
  const isSingleplayerLoading = status === "matched_connecting";
  const canUseRankedQueue = !!userId && !isGuest;
  const queueElapsedLabel = formatQueueElapsed(
    queueStartedAt ? nowMs - queueStartedAt : 0,
  );
  const showConnectionError =
    !connected && queueError.toLowerCase() === "connection error";
  const primaryButtonLabel = showConnectionError ? "Connection Error" : "Play";
  const userAvatarFallback = !userEmail
    ? "?"
    : (displayName || userEmail || "P").slice(0, 1).toUpperCase();
  const duelModeLabel = isQueueing ? "Searching..." : "Ranked";
  const showGoogleButton = !!googleClientId;
  const showDiscordButton = !!discordClientId;
  const maintenanceStartMs = parseTime(maintenance?.startsAt);
  const maintenanceEndMs = parseTime(maintenance?.endsAt);
  const maintenanceIsWarning = maintenance?.phase === "warning";
  const maintenanceIsActive = maintenance?.phase === "active";
  const queuePaused = !!maintenance?.queuePaused;
  const playPaused = !!maintenance?.playPaused;
  const maintenanceMessage = maintenance?.message?.trim() || "";
  const maintenanceAlertKey = maintenance
    ? [maintenance.phase, maintenance.startsAt, maintenance.endsAt, maintenance.message].join("|")
    : "";
  const warningCountdown =
    maintenanceIsWarning && maintenanceStartMs && maintenanceStartMs > nowMs
      ? formatRelativeDuration(maintenanceStartMs - nowMs)
      : "";
  const activeEta =
    maintenanceIsActive && maintenanceEndMs && maintenanceEndMs > nowMs
      ? formatApproximateTime(maintenanceEndMs - nowMs)
      : "";
  const duelDisabled =
    authLoading ||
    authMigrationRequired ||
    nicknameSaving ||
    queuePaused ||
    playPaused ||
    maintenanceIsActive;
  const singleplayerDisabled =
    isQueueing ||
    isSingleplayerLoading ||
    authLoading ||
    authMigrationRequired ||
    nicknameSaving ||
    playPaused ||
    maintenanceIsActive;
  const onDuelsPlay = () => {
    if (!canUseRankedQueue) {
      setOpenModal("signin");
      return;
    }
    setOpenModal("duel");
  };

  const startDuelQueue = () => {
    if (!extensionAvailable && (duel.streetNames !== "shown" || duel.modes.includes("no_move"))) {
      return;
    }
    const queues: QueueVariant[] = [];
    for (const mode of duel.modes) {
      if (duel.streetNames === "hidden" || duel.streetNames === "any") {
        queues.push(`${mode}_hidden` as QueueVariant);
      }
      if (duel.streetNames === "shown" || duel.streetNames === "any") {
        queues.push(mode);
      }
    }
    setOpenModal(null);
    joinQueue(queues);
  };

  const startSingleplayerFromModal = () => {
    if (!extensionAvailable && (singleplayer.streetNames !== "shown" || singleplayer.mode === "no_move")) {
      return;
    }
    setOpenModal(null);
    void startSingleplayer({
      ruleset: singleplayer.mode,
      streetNames: singleplayer.streetNames,
    });
  };

  const discordProviderButton = showDiscordButton ? (
    <DiscordProviderButton authLoading={authLoading} onClick={onDiscordSignIn} />
  ) : null;

  const signInButton =
    showGoogleButton || showDiscordButton ? (
      <SignInButton authLoading={authLoading} onClick={() => setOpenModal("signin")}>
        Sign In
      </SignInButton>
    ) : (
      <SignInButton authLoading={authLoading} onClick={devLogin} rounded="full">
        Dev Login
      </SignInButton>
    );

  const googleProviderButton = showGoogleButton ? (
    <GoogleProviderButton authLoading={authLoading} onClick={onGoogleSignIn} />
  ) : null;

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
  const socialLinksCard = <SocialLinksCard />;
  const partyPanelState = usePartyPanelState({
    party,
    userId,
    updateSettings: updatePartySettings,
    setInviteCopied,
  });
  const partyActive = partyPanelState.active;
  const partyConfig = partyPanelState.config;
  const savePartyConfig = partyPanelState.saveConfig;

  const mapPickerFlow = partyActive && party.isOwner && party.snapshot?.state === "open";
  const mapRouteSurface =
    contentRoute === "maps" || contentRoute === "map-details" || contentRoute === "map-upload" ? (
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
        partyActive={partyActive}
        savePartyConfig={savePartyConfig}
        singleplayerDisabled={singleplayerDisabled}
        startSingleplayer={startSingleplayer}
        userAvatar={userAvatar}
        userAvatarFallback={userAvatarFallback}
        userEmail={userEmail}
        userId={userId}
      />
    ) : null;

  const partyPanel = partyActive ? (
    <PartyPanel
      authError={authError}
      authLoading={authLoading}
      inviteCopied={inviteCopied}
      joinParty={joinParty}
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

  const maintenanceAlertDismissed = isAdmin && dismissedMaintenanceAlertKey === maintenanceAlertKey;
  const dismissMaintenanceAlert = isAdmin
    ? () => setDismissedMaintenanceAlertKey(maintenanceAlertKey)
    : undefined;
  const showMaintenanceBanner = maintenanceIsWarning && !maintenanceAlertDismissed;
  const maintenanceBanner = showMaintenanceBanner ? (
    <MaintenanceBanner
      message={maintenanceMessage}
      countdown={warningCountdown}
      onDismiss={dismissMaintenanceAlert}
    />
  ) : null;

  const maintenanceOverlay = maintenanceIsActive && !maintenanceAlertDismissed ? (
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

  const renderInvitePartyModal = () => (
    <InviteModal
      inviteCodeInput={inviteCodeInput}
      setInviteCodeInput={setInviteCodeInput}
      busy={party.busy}
      authLoading={authLoading}
      maintenanceIsActive={maintenanceIsActive}
      playPaused={playPaused}
      authError={authError}
      createParty={createParty}
      joinParty={joinParty}
      onClose={() => setOpenModal(null)}
    />
  );

  const renderSignInModal = () => (
    <SignInModal
      googleProviderButton={googleProviderButton}
      discordProviderButton={discordProviderButton}
      fallbackButton={signInButton}
      authError={authError}
      onClose={() => setOpenModal(null)}
    />
  );

  const renderPlayLaunchModal = () => {
    if (openModal === "duel") {
      return (
        <PlayLaunchModal
          kind="duel"
          extensionAvailable={extensionAvailable}
          extensionStatus={extensionStatus}
          modes={duel.modes}
          streetNames={duel.streetNames}
          disabled={duelDisabled}
          onModesChange={(modes) => setDuel((current) => ({ ...current, modes }))}
          onStreetNamesChange={(streetNames) =>
            setDuel((current) => ({ ...current, streetNames }))
          }
          onClose={() => setOpenModal(null)}
          onStart={startDuelQueue}
        />
      );
    }
    if (openModal === "singleplayer") {
      return (
        <PlayLaunchModal
          kind="singleplayer"
          extensionAvailable={extensionAvailable}
          extensionStatus={extensionStatus}
          mode={singleplayer.mode}
          streetNames={singleplayer.streetNames}
          disabled={singleplayerDisabled}
          onModeChange={(mode) =>
            setSingleplayer((current) => ({ ...current, mode }))
          }
          onStreetNamesChange={(streetNames) =>
            setSingleplayer((current) => ({ ...current, streetNames }))
          }
          onClose={() => setOpenModal(null)}
          onStart={startSingleplayerFromModal}
        />
      );
    }
    return null;
  };

  const invitePartyCard = (
    <InvitePartyCard
      disabled={authLoading || nicknameSaving || playPaused || maintenanceIsActive}
      onClick={() => setOpenModal("invite")}
    />
  );

  const partyErrorNotice = <PartyErrorNotice message={party.error} />;
  const mapPickerModal = mapPickerOpen ? (
    <MapPickerController
      accessToken={accessToken}
      canUploadCustomMaps={canUploadCustomMaps}
      partyConfig={partyConfig}
      onClose={() => setMapPickerOpen(false)}
      savePartyConfig={savePartyConfig}
      userId={userId}
    />
  ) : null;
  const showPartyPanel = partyActive && contentRoute !== "maps" && contentRoute !== "map-details" && contentRoute !== "map-upload";

  return (
    <>
      <AnimatePresence>{maintenanceOverlay}</AnimatePresence>
      <AnimatePresence>
        {openModal === "invite" && renderInvitePartyModal()}
        {openModal === "signin" && renderSignInModal()}
        {renderPlayLaunchModal()}
        {mapPickerModal}
      </AnimatePresence>
      <AppShell
        activeNavRoute={currentNavRoute}
        isAdmin={isAdmin}
        isModerator={isModerator}
        maintenanceBanner={maintenanceBanner}
        navigationDisabled={isQueueing}
        navigationHidden={showPartyPanel}
        onlinePlayers={onlinePlayers}
        signedOutAction={signInButton}
        viewer={
          userId && userEmail
            ? {
                userId,
                displayName: displayName || userEmail || "Player",
                avatarUrl: userAvatar,
                avatarFallback: userAvatarFallback,
                mmr,
                selectedBadge,
              }
            : null
        }
      >
        <AppContentRail
          as="main"
          size="wide"
          className="relative z-10 flex flex-1 flex-col items-center justify-start pb-28 pt-4 pointer-events-none sm:pb-12 sm:pt-8"
        >
          {partyErrorNotice}

          <div
            className={`flex w-full justify-center ${
              !showPartyPanel && contentRoute === "play"
                ? "md:min-h-[calc(100svh-21.25rem)] md:items-center"
                : ""
            }`}
          >
            <AnimatePresence mode="popLayout">
              {showPartyPanel ? partyPanel : null}

              {!showPartyPanel && contentRoute === "play" && (
                <PlayPanel
                  isQueueing={isQueueing}
                  isSingleplayerLoading={isSingleplayerLoading}
                  queueError={queueError}
                  onDuelsPlay={onDuelsPlay}
                  cancelQueue={cancelQueue}
                  onSingleplayerPlay={() => setOpenModal("singleplayer")}
                  duelDisabled={duelDisabled}
                  singleplayerDisabled={singleplayerDisabled}
                  queuePaused={queuePaused}
                  playPaused={playPaused}
                  maintenanceIsActive={maintenanceIsActive}
                  primaryButtonLabel={primaryButtonLabel}
                  queueElapsedLabel={queueElapsedLabel}
                  duelModeLabel={duelModeLabel}
                  updatesPanel={
                    <LobbyUpdatesPanel
                      newsPanel={newsPanel}
                      donateCard={donateCard}
                      socialLinksCard={socialLinksCard}
                    />
                  }
                />
              )}

              {!showPartyPanel && contentRoute === "top" && (
                <motion.div
                  key="top"
                  {...tabPanelMotion}
                  className="flex w-full justify-center pointer-events-auto"
                >
                  {leaderboardPanel}
                </motion.div>
              )}

              {!showPartyPanel && mapRouteSurface}

              {!showPartyPanel && contentRoute === "friends" && (
                <motion.div
                  key="friends"
                  {...tabPanelMotion}
                  className="flex w-full max-w-[520px] flex-col gap-5 pointer-events-auto"
                >
                  {invitePartyCard}
                </motion.div>
              )}
            </AnimatePresence>
          </div>

          {!showPartyPanel && contentRoute === "play" ? (
            <div className="mt-10 w-full sm:mt-16">
              <LobbyTutorialSection />
              <div className="mt-4 w-full">
                {legalCard}
              </div>
            </div>
          ) : null}
        </AppContentRail>
      </AppShell>
    </>
  );
}
