import { useEffect, useState } from "react";
import type { PartySnapshot } from "../lib/party-client";
import type { PartyRuntimeStatus } from "../controllers/party-controller";
import type {
  MaintenanceStatus,
  MatchConfig,
  MatchReturnTarget,
  QueueVariant,
} from "../../matchmaking/lib/queue-client";
import {
  formatApproximateTime,
  formatQueueElapsed,
  formatRelativeDuration,
  parseTime,
  type LobbyContentRoute,
} from "../lib/lobby-ui";
import { useExtensionAvailability } from "../../browser-extension/hooks/use-extension-availability";
import { useAuthActions, useAuthState } from "../../auth/components/AuthProvider";
import { usePartyPanelState } from "./usePartyPanelState";
import { usePlayPreferences } from "./usePlayPreferences";
import type { LobbyModal } from "../components/LobbyScreenModals";

export type LobbyPartyView = {
  status: PartyRuntimeStatus;
  snapshot: PartySnapshot | null;
  inviteCode: string;
  isOwner: boolean;
  busy: boolean;
  error: string;
};

type Options = {
  contentRoute: LobbyContentRoute;
  onBrowseLeaderboard: () => void;
  maintenance: MaintenanceStatus | null;
  status: string;
  userId: string;
  isGuest: boolean;
  party: LobbyPartyView;
  updatePartySettings: (config: MatchConfig, mode?: "duel" | "team_duel" | "free_for_all") => Promise<void>;
  queueStartedAt: number | null;
  connected: boolean;
  queueError: string;
  userEmail: string;
  displayName: string;
  authLoading: boolean;
  authMigrationRequired: boolean;
  nicknameSaving: boolean;
  joinQueue: (queues?: QueueVariant[]) => void;
  startSingleplayer: (config?: MatchConfig, returnTarget?: MatchReturnTarget) => Promise<string>;
  clearSingleplayerError: () => void;
};

export function useLobbyScreenState({
  contentRoute,
  onBrowseLeaderboard,
  maintenance,
  status,
  userId,
  isGuest,
  party,
  updatePartySettings,
  queueStartedAt,
  connected,
  queueError,
  userEmail,
  displayName,
  authLoading,
  authMigrationRequired,
  nicknameSaving,
  joinQueue,
  startSingleplayer,
  clearSingleplayerError,
}: Options) {
  const [openModal, setOpenModal] = useState<LobbyModal>(null);
  const { openSignIn, authLoading: authActionLoading } = useAuthActions();
  const authState = useAuthState();
  const extensionStatus = useExtensionAvailability();
  const extensionAvailable = extensionStatus.state === "ready";
  const { duel, setDuel, singleplayer, setSingleplayer } = usePlayPreferences();
  const [inviteCopied, setInviteCopied] = useState(false);
  const [inviteCodeInput, setInviteCodeInput] = useState("");
  const [mapPickerOpen, setMapPickerOpen] = useState(false);
  const [singleplayerMap, setSingleplayerMap] = useState<
    Pick<MatchConfig, "mapId" | "mapName"> | undefined
  >();
  const [nowMs, setNowMs] = useState(() => Date.now());
  const [dismissedMaintenanceAlertKey, setDismissedMaintenanceAlertKey] =
    useState("");
  const currentNavRoute =
    contentRoute === "map-details" || contentRoute === "map-upload"
      ? "maps"
      : contentRoute === "party"
        ? "friends"
        : contentRoute;

  useEffect(() => {
    if (contentRoute === "top") onBrowseLeaderboard();
  }, [contentRoute, onBrowseLeaderboard]);

  useEffect(() => {
    if (!maintenance && status !== "queueing") return;
    setNowMs(Date.now());
    const timer = window.setInterval(() => setNowMs(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [maintenance, status]);

  const partyPanelState = usePartyPanelState({
    party,
    userId,
    updateSettings: updatePartySettings,
    setInviteCopied,
  });
  const hasActiveParty =
    !!party.snapshot &&
    !!partyPanelState.currentMember &&
    (party.status === "ready" || party.status === "reconnecting");
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
  // Session restore is owned by AuthProvider (`bootstrapping`). sessionController
  // `authLoading` and AuthActions `authLoading` are only intentional in-flight ops.
  const authBusy =
    authState.status === "bootstrapping" ||
    authActionLoading ||
    authLoading ||
    nicknameSaving;
  const duelDisabled =
    isQueueing || hasActiveParty || authBusy || authMigrationRequired ||
    queuePaused || playPaused || maintenanceIsActive;
  const singleplayerDisabled =
    isQueueing || hasActiveParty || isSingleplayerLoading || authBusy ||
    authMigrationRequired || playPaused || maintenanceIsActive;
  const onDuelsPlay = () => {
    if (canUseRankedQueue) setOpenModal("duel");
    else openSignIn();
  };
  const startDuelQueue = () => {
    if (!extensionAvailable && duel.queues.includes("no_move_hidden")) return;
    setOpenModal(null);
    joinQueue(duel.queues);
  };
  const startSingleplayerFromModal = async () => {
    if (
      !extensionAvailable &&
      (singleplayer.streetNames !== "shown" || singleplayer.mode === "no_move")
    ) return;
    const matchId = await startSingleplayer({
      ...singleplayerMap,
      ruleset: singleplayer.mode,
      streetNames: singleplayer.streetNames,
    }, singleplayerMap?.mapId ? { kind: "map", mapId: singleplayerMap.mapId } : { kind: "home" });
    if (matchId) setOpenModal(null);
  };
  const openSingleplayerModal = (
    map?: Pick<MatchConfig, "mapId" | "mapName">,
  ) => {
    clearSingleplayerError();
    setSingleplayerMap(map);
    setOpenModal("singleplayer");
  };

  return {
    openModal,
    setOpenModal,
    extensionStatus,
    extensionAvailable,
    duel,
    setDuel,
    singleplayer,
    setSingleplayer,
    inviteCopied,
    setInviteCopied,
    inviteCodeInput,
    setInviteCodeInput,
    mapPickerOpen,
    setMapPickerOpen,
    currentNavRoute,
    partyPanelState,
    hasActiveParty,
    isQueueing,
    isSingleplayerLoading,
    queueElapsedLabel,
    primaryButtonLabel,
    userAvatarFallback,
    maintenanceIsWarning,
    maintenanceIsActive,
    queuePaused,
    playPaused,
    maintenanceMessage,
    maintenanceAlertKey,
    warningCountdown,
    activeEta,
    authBusy,
    duelDisabled,
    singleplayerDisabled,
    onDuelsPlay,
    startDuelQueue,
    startSingleplayerFromModal,
    openSingleplayerModal,
    dismissedMaintenanceAlertKey,
    setDismissedMaintenanceAlertKey,
  };
}
