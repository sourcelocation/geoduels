import type { ComponentProps, Dispatch, SetStateAction } from "react";
import type { ExtensionAvailabilityStatus } from "../../browser-extension/hooks/use-extension-availability";
import type {
  GameRuleset,
  MatchConfig,
  QueueVariant,
  StreetNamesVisibility,
} from "../../matchmaking/lib/queue-client";
import { MapPickerController } from "./maps/MapRouteSurfaces";
import { InviteModal } from "./modals/InviteModal";
import { PlayLaunchModal } from "./PlayLaunchModal";

export type LobbyModal = "invite" | "duel" | "singleplayer" | null;

type PartyPreferences = {
  queues: QueueVariant[];
};

type SingleplayerPreferences = {
  mode: GameRuleset;
  streetNames: StreetNamesVisibility;
};

type Props = {
  openModal: LobbyModal;
  setOpenModal: (modal: LobbyModal) => void;
  inviteCodeInput: string;
  setInviteCodeInput: (value: string) => void;
  partyBusy: boolean;
  authLoading: boolean;
  maintenanceIsActive: boolean;
  playPaused: boolean;
  partyError: string;
  authError: string;
  createParty: () => Promise<boolean>;
  joinParty: (inviteCode?: string) => Promise<boolean>;
  extensionAvailable: boolean;
  extensionStatus: ExtensionAvailabilityStatus;
  duel: PartyPreferences;
  setDuel: Dispatch<SetStateAction<PartyPreferences>>;
  singleplayer: SingleplayerPreferences;
  setSingleplayer: Dispatch<SetStateAction<SingleplayerPreferences>>;
  duelDisabled: boolean;
  singleplayerDisabled: boolean;
  startDuelQueue: () => void;
  startSingleplayerFromModal: () => Promise<void>;
  singleplayerError: string;
  clearSingleplayerError: () => void;
  mapPickerOpen: boolean;
  accessToken: string;
  canUploadCustomMaps: boolean;
  partyConfig: ComponentProps<typeof MapPickerController>["partyConfig"];
  savePartyConfig: ComponentProps<typeof MapPickerController>["savePartyConfig"];
  userId: string;
  closeMapPicker: () => void;
};

export function LobbyScreenModals({
  openModal,
  setOpenModal,
  inviteCodeInput,
  setInviteCodeInput,
  partyBusy,
  authLoading,
  maintenanceIsActive,
  playPaused,
  partyError,
  authError,
  createParty,
  joinParty,
  extensionAvailable,
  extensionStatus,
  duel,
  setDuel,
  singleplayer,
  setSingleplayer,
  duelDisabled,
  singleplayerDisabled,
  startDuelQueue,
  startSingleplayerFromModal,
  singleplayerError,
  clearSingleplayerError,
  mapPickerOpen,
  accessToken,
  canUploadCustomMaps,
  partyConfig,
  savePartyConfig,
  userId,
  closeMapPicker,
}: Props) {
  const inviteModal = (
    <InviteModal
      inviteCodeInput={inviteCodeInput}
      setInviteCodeInput={setInviteCodeInput}
      busy={partyBusy}
      authLoading={authLoading}
      maintenanceIsActive={maintenanceIsActive}
      playPaused={playPaused}
      authError={partyError || authError}
      createParty={createParty}
      joinParty={joinParty}
      onClose={() => setOpenModal(null)}
    />
  );
  const playLaunchModal =
    openModal === "duel" ? (
      <PlayLaunchModal
        kind="duel"
        extensionAvailable={extensionAvailable}
        extensionStatus={extensionStatus}
        queues={duel.queues}
        disabled={duelDisabled}
        onQueuesChange={(queues) => setDuel({ queues })}
        onClose={() => setOpenModal(null)}
        onStart={startDuelQueue}
      />
    ) : openModal === "singleplayer" ? (
      <PlayLaunchModal
        kind="singleplayer"
        extensionAvailable={extensionAvailable}
        extensionStatus={extensionStatus}
        mode={singleplayer.mode}
        streetNames={singleplayer.streetNames}
        disabled={singleplayerDisabled}
        error={singleplayerError}
        onModeChange={(mode) => {
          clearSingleplayerError();
          setSingleplayer((current) => ({ ...current, mode }));
        }}
        onStreetNamesChange={(streetNames) =>
          {
            clearSingleplayerError();
            setSingleplayer((current) => ({ ...current, streetNames }));
          }
        }
        onClose={() => {
          clearSingleplayerError();
          setOpenModal(null);
        }}
        onStart={startSingleplayerFromModal}
      />
    ) : null;
  const mapPickerModal = mapPickerOpen ? (
    <MapPickerController
      accessToken={accessToken}
      canUploadCustomMaps={canUploadCustomMaps}
      partyConfig={partyConfig}
      onClose={closeMapPicker}
      savePartyConfig={savePartyConfig}
      userId={userId}
    />
  ) : null;

  return (
    <>
      {openModal === "invite" ? inviteModal : null}
      {playLaunchModal}
      {mapPickerModal}
    </>
  );
}
