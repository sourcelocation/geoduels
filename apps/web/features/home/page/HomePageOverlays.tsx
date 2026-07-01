import EndMatchOverlay from "../../../components/ui/EndMatchOverlay";
import RequiredNicknameModal from "../../../components/home/RequiredNicknameModal";
import AppModalShell from "../../../components/ui/AppModalShell";
import GuestVerificationOverlay from "./GuestVerificationOverlay";
import type {
  HomeActions,
  HomeAuthView,
  HomeOverlaysView,
} from "../model/types";

type HomePageOverlaysProps = {
  auth: HomeAuthView;
  overlays: HomeOverlaysView;
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
  actions,
}: HomePageOverlaysProps) {
  const activeNotification = overlays.notifications?.[0];
  const replayConfig = overlays.endMatch.open
    ? overlays.endMatch.matchConfig
    : undefined;
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
          zIndexClassName="z-[1000]"
          maxWidthClassName="max-w-sm"
        >
          <p className="text-xs font-black uppercase tracking-[0.16em] text-[#2ad18f]">
            Rating refunded
          </p>
          <h2 className="mt-2 text-2xl font-black">
            +{activeNotification.payload.refundDelta || 0} MMR
          </h2>
          <p className="mt-3 text-sm leading-6 text-[#b9c9d8]">
            A player you lost to was banned for cheating. Your rating has been
            recalculated from your current MMR and refunded.
          </p>
          <button
            type="button"
            onClick={() =>
              void actions.dismissNotification(activeNotification.id)
            }
            className="mt-5 min-h-11 w-full rounded-xl bg-accentPrimary px-4 text-sm font-black text-white hover:bg-accentPrimaryDeep"
          >
            Got it
          </button>
        </AppModalShell>
      ) : null}
      {activeNotification?.type === "badge_unlocked" &&
      activeNotification.payload.badge ? (
        <AppModalShell
          title="New badge unlocked"
          placement="center"
          showHeader={false}
          zIndexClassName="z-[1000]"
          maxWidthClassName="max-w-sm"
        >
          <p className="text-xs font-black uppercase tracking-[0.16em] text-[#77f0be]">
            New badge unlocked!
          </p>
          <div className="mt-5 flex flex-col items-center text-center">
            <img
              src={activeNotification.payload.badge.imageUrl}
              alt=""
              className="h-24 w-24 object-contain drop-shadow-[0_12px_28px_rgba(0,0,0,0.45)]"
            />
            <h2 className="mt-4 text-2xl font-black">
              {activeNotification.payload.badge.label}
            </h2>
            {activeNotification.payload.badge.description ? (
              <p className="mt-3 text-sm leading-6 text-[#b9c9d8]">
                {activeNotification.payload.badge.description}
              </p>
            ) : null}
          </div>
          <button
            type="button"
            onClick={() =>
              void actions.dismissNotification(activeNotification.id)
            }
            className="mt-5 min-h-11 w-full rounded-xl bg-accentPrimary px-4 text-sm font-black text-white hover:bg-accentPrimaryDeep"
          >
            Claim
          </button>
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
