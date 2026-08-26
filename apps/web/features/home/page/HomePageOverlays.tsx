import EndMatchOverlay from "../../game/components/overlays/EndMatchOverlay";
import RequiredNicknameModal from "../../../components/home/RequiredNicknameModal";
import AppModalShell from "../../../components/ui/AppModalShell";
import GuestVerificationOverlay from "./GuestVerificationOverlay";
import type {
  HomeActions,
  HomeAuthView,
  HomeOverlaysView,
} from "../model/types";
import { useHotkey } from "../../hotkeys/hooks/use-hotkey";
import { Button } from "../../../components/ui/button";

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
    | "dismissNotification"
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
  const activeNotification = overlays.notifications?.[0];
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
      {activeNotification?.type === "mmr_refund" ? (
        <AppModalShell
          title="Rating refunded"
          placement="center"
          showHeader={false}
          zIndexClassName="z-modal"
          maxWidthClassName="max-w-sm"
        >
          <p className="text-label font-strong uppercase text-status-success">
            Rating refunded
          </p>
          <h2 className="mt-2 text-heading-md font-strong">
            +{activeNotification.payload.refundDelta || 0} MMR
          </h2>
          <p className="mt-3 text-body-sm text-content-secondary">
            A player you lost to was banned for cheating. Your rating has been
            recalculated from your current MMR and refunded.
          </p>
          <Button
            type="button"
            variant="primary"
            size="lg"
            onClick={() =>
              void actions.dismissNotification(activeNotification.id)
            }
            className="mt-5 w-full"
          >
            Got it
          </Button>
        </AppModalShell>
      ) : null}
      {activeNotification?.type === "badge_unlocked" &&
      activeNotification.payload.badge ? (
        <AppModalShell
          title="New badge unlocked"
          placement="center"
          showHeader={false}
          zIndexClassName="z-modal"
          maxWidthClassName="max-w-sm"
        >
          <p className="text-label font-strong uppercase text-status-success">
            New badge unlocked!
          </p>
          <div className="mt-5 flex flex-col items-center text-center">
            <img
              src={activeNotification.payload.badge.imageUrl}
              alt=""
              className="h-24 w-24 object-contain drop-shadow-lg"
            />
            <h2 className="mt-4 text-heading-md font-strong">
              {activeNotification.payload.badge.label}
            </h2>
            {activeNotification.payload.badge.description ? (
              <p className="mt-3 text-body-sm text-content-secondary">
                {activeNotification.payload.badge.description}
              </p>
            ) : null}
          </div>
          <Button
            type="button"
            variant="primary"
            size="lg"
            onClick={() =>
              void actions.dismissNotification(activeNotification.id)
            }
            className="mt-5 w-full"
          >
            Claim
          </Button>
        </AppModalShell>
      ) : null}
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
