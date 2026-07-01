import type React from "react";
import { forwardRef } from "react";
import { Loader2, Play } from "lucide-react";
import { motion } from "framer-motion";
import { LobbyActionButton, LobbyPanel } from "./lobby-primitives";

const panelMotion = {
  initial: { opacity: 0, y: 16, scale: 0.97 },
  animate: { opacity: 1, y: 0, scale: 1 },
  exit: { opacity: 0, y: 10, scale: 0.97 },
  transition: { duration: 0.22, ease: [0.16, 1, 0.3, 1] as const },
};

type PlayPanelProps = {
  isQueueing: boolean;
  isSingleplayerLoading: boolean;
  queueError: string;
  onDuelsPlay: () => void;
  cancelQueue: () => void;
  onSingleplayerPlay: () => void;
  duelDisabled: boolean;
  singleplayerDisabled: boolean;
  queuePaused: boolean;
  playPaused: boolean;
  maintenanceIsActive: boolean;
  primaryButtonLabel: string;
  queueElapsedLabel: string;
  duelModeLabel: string;
  updatesPanel: React.ReactNode;
};

type PlayModeActionButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  tone: "duel" | "singleplayer";
  loading?: boolean;
};

const playModeActionButtonClass =
  "w-full min-h-[54px] rounded-2xl px-4 py-0 text-base leading-none transition-transform hover:scale-[1.01] active:scale-[0.98]";

export function PlayModeActionButton({
  tone,
  loading = false,
  children,
  className = "",
  ...props
}: PlayModeActionButtonProps) {
  if (tone === "duel") {
    return (
      <LobbyActionButton
        {...props}
        className={`${playModeActionButtonClass} ${className}`}
      >
        {loading ? (
          <Loader2 size={20} className="animate-spin" />
        ) : (
          <Play fill="currentColor" size={20} />
        )}
        {children}
      </LobbyActionButton>
    );
  }

  return (
    <button
      type="button"
      {...props}
      className={`inline-flex items-center justify-center gap-2 border border-transparent bg-[#3b82f6] font-extrabold uppercase tracking-[0.08em] text-white shadow-[0_4px_16px_rgba(59,130,246,0.3)] transition-all duration-200 hover:bg-[#4b8df8] hover:shadow-[0_6px_24px_rgba(59,130,246,0.4)] disabled:cursor-not-allowed disabled:opacity-60 disabled:hover:scale-100 ${playModeActionButtonClass} ${className}`}
    >
      {loading ? (
        <Loader2 size={20} className="animate-spin" />
      ) : (
        <Play fill="currentColor" size={20} />
      )}
      {children}
    </button>
  );
}

export const PlayPanel = forwardRef<HTMLDivElement, PlayPanelProps>(function PlayPanel({
  isQueueing,
  isSingleplayerLoading,
  queueError,
  onDuelsPlay,
  cancelQueue,
  onSingleplayerPlay,
  duelDisabled,
  singleplayerDisabled,
  queuePaused,
  playPaused,
  maintenanceIsActive,
  primaryButtonLabel,
  queueElapsedLabel,
  duelModeLabel,
  updatesPanel,
}, ref) {
  return (
    <motion.div
      ref={ref}
      key="play"
      {...panelMotion}
      className="flex w-full max-w-[860px] flex-col gap-5 pointer-events-auto sm:gap-6"
    >
      <div className="grid w-full gap-5 sm:gap-6 lg:grid-cols-2">
        <QueueModeCard
          isQueueing={isQueueing}
          queueError={queueError}
          onDuelsPlay={onDuelsPlay}
          cancelQueue={cancelQueue}
          duelDisabled={duelDisabled}
          queuePaused={queuePaused}
          playPaused={playPaused}
          maintenanceIsActive={maintenanceIsActive}
          primaryButtonLabel={primaryButtonLabel}
          queueElapsedLabel={queueElapsedLabel}
          duelModeLabel={duelModeLabel}
        />
        <SingleplayerModeCard
          isSingleplayerLoading={isSingleplayerLoading}
          singleplayerDisabled={singleplayerDisabled}
          playPaused={playPaused}
          maintenanceIsActive={maintenanceIsActive}
          onSingleplayerPlay={onSingleplayerPlay}
        />
      </div>

      {updatesPanel}
    </motion.div>
  );
});

function QueueModeCard(props: {
  isQueueing: boolean;
  queueError: string;
  onDuelsPlay: () => void;
  cancelQueue: () => void;
  duelDisabled: boolean;
  queuePaused: boolean;
  playPaused: boolean;
  maintenanceIsActive: boolean;
  primaryButtonLabel: string;
  queueElapsedLabel: string;
  duelModeLabel: string;
}) {
  return (
    <LobbyPanel className="lobby-feature-card relative flex min-h-[165px] w-full flex-col gap-4 p-4 transition-colors duration-500 sm:min-h-[180px] sm:p-5">
      <div className={`absolute inset-0 pointer-events-none transition-opacity duration-500 ${props.isQueueing ? "opacity-95" : "opacity-80"} bg-[linear-gradient(180deg,rgba(72,128,106,0.28)_0%,rgba(22,42,34,0.78)_100%)]`} />
      <ModeMountains active={props.isQueueing} />
      <ModeHeading eyebrow={props.duelModeLabel} title="Duel" eyebrowClassName="text-[#8cb0a1]" />

      <div className="relative z-10 flex w-full flex-col">
        {props.queueError ? <p className="mb-3 text-center text-xs font-semibold text-red-300">{props.queueError}</p> : null}
        {!props.isQueueing ? (
          <PlayModeActionButton tone="duel" onClick={props.onDuelsPlay} disabled={props.duelDisabled}>
            {props.queuePaused || props.playPaused || props.maintenanceIsActive ? "Paused" : props.primaryButtonLabel}
          </PlayModeActionButton>
        ) : (
          <LobbyActionButton onClick={props.cancelQueue} variant="secondary" className="group w-full rounded-2xl py-[14px] text-sm">
            <Loader2 size={18} className="mr-3 animate-spin text-[#2ad18f] transition-colors group-hover:text-[#3deb9e]" />
            <span className="text-accentPrimary">{props.queueElapsedLabel}</span>
          </LobbyActionButton>
        )}
      </div>
    </LobbyPanel>
  );
}

function SingleplayerModeCard(props: {
  isSingleplayerLoading: boolean;
  singleplayerDisabled: boolean;
  playPaused: boolean;
  maintenanceIsActive: boolean;
  onSingleplayerPlay: () => void;
}) {
  return (
    <LobbyPanel className="lobby-feature-card relative flex min-h-[165px] w-full flex-col gap-4 p-4 transition-colors duration-500 sm:min-h-[180px] sm:p-5" style={{ animationDelay: "-2s" }}>
      <div className="absolute inset-0 pointer-events-none bg-[linear-gradient(180deg,rgba(72,106,128,0.28)_0%,rgba(22,34,42,0.85)_100%)] opacity-80 transition-opacity duration-500" />
      <ModeMountains hueRotate active={false} />
      <ModeHeading eyebrow="Casual" title="Singleplayer" eyebrowClassName="text-[#8caab0]" />

      <div className="relative z-10 w-full">
        <PlayModeActionButton
          tone="singleplayer"
          loading={props.isSingleplayerLoading}
          onClick={props.onSingleplayerPlay}
          disabled={props.singleplayerDisabled}
        >
          {props.isSingleplayerLoading ? "Loading..." : props.playPaused || props.maintenanceIsActive ? "Paused" : "Play"}
        </PlayModeActionButton>
      </div>
    </LobbyPanel>
  );
}

function ModeMountains({ active, hueRotate = false }: { active: boolean; hueRotate?: boolean }) {
  return (
    <div className={`absolute inset-x-0 bottom-0 pointer-events-none h-full transition-opacity duration-500 ${active ? "opacity-[0.24]" : "opacity-[0.32]"}`}>
      <img
        src="/mountains.v1.svg"
        alt=""
        aria-hidden="true"
        className={`absolute inset-0 h-full w-full object-cover object-center ${hueRotate ? "opacity-50" : ""}`}
        style={{ objectPosition: "center bottom", filter: hueRotate ? "hue-rotate(190deg)" : undefined }}
      />
    </div>
  );
}

function ModeHeading({
  eyebrow,
  title,
  subtitle,
  eyebrowClassName,
}: {
  eyebrow: string;
  title: string;
  subtitle?: string;
  eyebrowClassName: string;
}) {
  return (
    <div className="relative z-10 flex flex-col">
      <span className={`mb-1 text-xs font-bold uppercase tracking-[0.16em] drop-shadow-sm ${eyebrowClassName}`}>
        {eyebrow}
      </span>
      <h2 className="text-[32px] font-extrabold leading-tight tracking-tight text-white drop-shadow-md sm:text-[38px]">
        {title}
      </h2>
      {subtitle ? <span className="text-[15px] font-medium text-white/90 drop-shadow-sm sm:text-base">{subtitle}</span> : null}
    </div>
  );
}
