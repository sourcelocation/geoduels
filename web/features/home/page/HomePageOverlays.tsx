import EndMatchOverlay from "../../game/components/overlays/EndMatchOverlay";
import RequiredNicknameModal from "../../../components/home/RequiredNicknameModal";
import GuestVerificationOverlay from "./GuestVerificationOverlay";
import type {
  HomeActions,
  HomeAuthView,
  HomeOverlaysView,
} from "../model/types";
import { useHotkey } from "../../hotkeys/hooks/use-hotkey";

type HomePageOverlaysProps = {
  auth: HomeAuthView;
  overlays: HomeOverlaysView;
  maxHP: number;
  actions: Pick<
    HomeActions,
    | "setNicknameInput"
    | "submitRequiredNickname"
    | "leaveGame"
    | "reportPlayer"
    | "startSingleplayer"
    | "submitGuestVerificationToken"
    | "markGuestVerificationExpired"
    | "cancelGuestVerification"
  >;
};

export default function HomePageOverlays({
  auth,
  overlays,
  maxHP,
  actions,
}: HomePageOverlaysProps) {
  const replayConfig = overlays.endMatch.open
    ? overlays.endMatch.matchConfig
    : undefined;
  const canReplaySingleplayer =
    overlays.endMatch.open && overlays.endMatch.mode === "singleplayer";

  useHotkey({
    action: "gameplay.primary",
    scope: "gameplay",
    enabled: canReplaySingleplayer,
    run: () => {
      void actions.startSingleplayer(replayConfig);
    },
  });

  return (
    <>
      <RequiredNicknameModal
        open={overlays.nicknameRequiredOpen}
        nicknameInput={auth.nicknameInput}
        nicknameError={auth.nicknameError}
        nicknameSaving={auth.nicknameSaving}
        onChangeNickname={actions.setNicknameInput}
        onSubmit={() => void actions.submitRequiredNickname()}
      />
      <GuestVerificationOverlay
        verification={overlays.guestVerification}
        onToken={actions.submitGuestVerificationToken}
        onExpired={actions.markGuestVerificationExpired}
        onCancel={actions.cancelGuestVerification}
      />
      {overlays.endMatch.open && (
        <EndMatchOverlay
          onLeaveGame={actions.leaveGame}
          backLabel={overlays.endMatch.backLabel}
          mode={overlays.endMatch.mode}
          outcome={overlays.endMatch.outcome}
          sides={overlays.endMatch.sides}
          selfUserId={overlays.endMatch.selfUserId}
          totalScore={overlays.endMatch.totalScore}
          maxHP={maxHP}
          roundResults={overlays.endMatch.roundResults}
          resultPlayerNames={overlays.endMatch.resultPlayerNames}
          resultPlayerAvatars={overlays.endMatch.resultPlayerAvatars}
          resultPlayerFallbacks={overlays.endMatch.resultPlayerFallbacks}
          resultPlayerBorderColors={overlays.endMatch.resultPlayerBorderColors}
          participantsById={overlays.endMatch.participantsById}
          onReportPlayer={actions.reportPlayer}
          onPlayAgain={
            overlays.endMatch.mode === "singleplayer"
              ? () => actions.startSingleplayer(replayConfig)
              : undefined
          }
          asPage
        />
      )}
    </>
  );
}
