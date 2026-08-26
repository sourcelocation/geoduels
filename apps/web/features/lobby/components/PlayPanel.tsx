import type React from "react";
import { forwardRef } from "react";
import { CalendarDays, Play } from "lucide-react";
import { motion } from "framer-motion";
import { Button } from "../../../components/ui/button";
import { AppPanel } from "../../../components/ui/compositions";
import { HorizontalScroller } from "../../../components/ui/HorizontalScroller";
import { CenteredSpinner } from "../../../components/ui/Spinner";
import { Eyebrow, Heading, SectionTitle } from "../../../components/ui/typography";
import type { CustomMap } from "../../maps/lib/maps-client";
import { mapThumbnailURL } from "../../maps/lib/map-thumbnails";
import { MapCard } from "./maps/MapPanels";

const panelMotion = {
  initial: { opacity: 0, y: 16, scale: 0.97 },
  animate: { opacity: 1, y: 0, scale: 1 },
  exit: { opacity: 0, y: 10, scale: 0.97 },
  transition: { duration: 0.22, ease: [0.16, 1, 0.3, 1] as const },
};

type PlayPanelProps = {
  isSingleplayerLoading: boolean;
  queueError: string;
  onDuelsPlay: () => void;
  onSingleplayerPlay: () => void;
  duelDisabled: boolean;
  singleplayerDisabled: boolean;
  queuePaused: boolean;
  playPaused: boolean;
  maintenanceIsActive: boolean;
  primaryButtonLabel: string;
  trendingMaps: CustomMap[];
  trendingMapsLoading: boolean;
  changelogCard: React.ReactNode;
  donateCard: React.ReactNode;
  socialCard: React.ReactNode;
};

type PlayModeActionButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  tone: "duel" | "singleplayer";
  loading?: boolean;
};

const playModeActionButtonClass =
  "w-full min-h-[54px] rounded-2xl px-4 py-0 text-body leading-collapsed transition-transform hover:scale-[1.01] active:scale-[0.98]";

export function PlayModeActionButton({
  tone,
  loading = false,
  children,
  className = "",
  ...props
}: PlayModeActionButtonProps) {
  return (
    <Button
      {...props}
      variant={tone === "duel" ? "primary" : "blue"}
      className={`${playModeActionButtonClass} ${className}`}
      loading={loading}
      icon={<Play fill="currentColor" size={20} />}
    >
      {children}
    </Button>
  );
}

export const PlayPanel = forwardRef<HTMLDivElement, PlayPanelProps>(function PlayPanel({
  isSingleplayerLoading,
  queueError,
  onDuelsPlay,
  onSingleplayerPlay,
  duelDisabled,
  singleplayerDisabled,
  queuePaused,
  playPaused,
  maintenanceIsActive,
  primaryButtonLabel,
  trendingMaps,
  trendingMapsLoading,
  changelogCard,
  donateCard,
  socialCard,
}, ref) {
  return (
    <motion.div
      ref={ref}
      key="play"
      {...panelMotion}
      className="flex w-full max-w-6xl flex-col gap-8 pointer-events-auto sm:gap-10"
    >
      <section>
        <Heading as="h2" variant="heading-md" className="mb-3">Play</Heading>
        <div className="grid w-full gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <QueueModeCard
            queueError={queueError}
            onDuelsPlay={onDuelsPlay}
            duelDisabled={duelDisabled}
            queuePaused={queuePaused}
            playPaused={playPaused}
            maintenanceIsActive={maintenanceIsActive}
            primaryButtonLabel={primaryButtonLabel}
          />
          <SingleplayerModeCard
            isSingleplayerLoading={isSingleplayerLoading}
            singleplayerDisabled={singleplayerDisabled}
            playPaused={playPaused}
            maintenanceIsActive={maintenanceIsActive}
            onSingleplayerPlay={onSingleplayerPlay}
          />
          <DailyQuizModeCard />
        </div>
      </section>

      <HorizontalScroller label="Trending Maps" itemClassName="w-72 sm:w-80" viewAllHref="/maps">
        {trendingMapsLoading ? (
          <AppPanel className="flex h-44 w-72 items-center justify-center rounded-2xl sm:w-80">
            <CenteredSpinner label="Loading trending maps" />
          </AppPanel>
        ) : trendingMaps.length ? (
          trendingMaps.map((map) => (
            <MapCard key={map.id} item={map} mode="link" thumbnailURL={(item) => mapThumbnailURL(item.thumbnailKey, item.thumbnailVariant)} />
          ))
        ) : (
          <AppPanel className="flex h-44 w-72 items-center justify-center rounded-2xl p-5 text-center text-body-sm font-semibold text-content-secondary sm:w-80">
            No trending maps yet.
          </AppPanel>
        )}
      </HorizontalScroller>

      <HorizontalScroller label="GeoDuels" itemClassName="w-80 sm:w-96">
        {changelogCard}
        {socialCard}
        {donateCard}
      </HorizontalScroller>
    </motion.div>
  );
});

function QueueModeCard(props: {
  queueError: string;
  onDuelsPlay: () => void;
  duelDisabled: boolean;
  queuePaused: boolean;
  playPaused: boolean;
  maintenanceIsActive: boolean;
  primaryButtonLabel: string;
}) {
  return (
    <AppPanel className="lobby-feature-card relative flex min-h-[180px] w-full flex-col gap-4 rounded-2xl p-4 transition-colors duration-emphasis sm:p-5">
      <div className="pointer-events-none absolute inset-0 bg-status-success/20 opacity-80 transition-opacity duration-emphasis" />
      <ModeMountains active={false} />
        <ModeHeading eyebrow="Ranked" title="Duel" eyebrowClassName="text-status-success" />

      <div className="relative z-content flex w-full flex-col">
        {props.queueError ? <p className="mb-3 text-center text-body-sm font-semibold text-status-danger">{props.queueError}</p> : null}
        <PlayModeActionButton tone="duel" onClick={props.onDuelsPlay} disabled={props.duelDisabled}>
          {props.queuePaused || props.playPaused || props.maintenanceIsActive ? "Paused" : props.primaryButtonLabel}
        </PlayModeActionButton>
      </div>
    </AppPanel>
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
    <AppPanel className="lobby-feature-card relative flex min-h-[180px] w-full flex-col gap-4 rounded-2xl p-4 transition-colors duration-emphasis sm:p-5" style={{ animationDelay: "-2s" }}>
      <div className="pointer-events-none absolute inset-0 bg-status-info/20 opacity-80 transition-opacity duration-emphasis" />
      <ModeMountains hueRotate active={false} />
      <ModeHeading eyebrow="Casual" title="Singleplayer" eyebrowClassName="text-status-info" />

      <div className="relative z-content w-full">
        <PlayModeActionButton
          tone="singleplayer"
          loading={props.isSingleplayerLoading}
          onClick={props.onSingleplayerPlay}
          disabled={props.singleplayerDisabled}
        >
          {props.isSingleplayerLoading ? "Loading..." : props.playPaused || props.maintenanceIsActive ? "Paused" : "Play"}
        </PlayModeActionButton>
      </div>
    </AppPanel>
  );
}

function DailyQuizModeCard() {
  return (
    <AppPanel className="lobby-feature-card relative flex min-h-[180px] w-full flex-col gap-4 rounded-2xl p-4 sm:p-5">
      <div className="pointer-events-none absolute inset-0 bg-status-warning/20 opacity-80" />
      <div className="relative z-content flex flex-1 flex-col justify-between gap-4">
        <div>
          <Eyebrow className="mb-1 text-status-warning">Daily</Eyebrow>
          <SectionTitle className="text-content-primary drop-shadow-md">Quiz</SectionTitle>
        </div>
        <Button type="button" variant="secondary" className={playModeActionButtonClass} icon={<CalendarDays size={20} />} disabled>
          Very Soon
        </Button>
      </div>
    </AppPanel>
  );
}

function ModeMountains({ active, hueRotate = false }: { active: boolean; hueRotate?: boolean }) {
  return (
    <div className={`absolute inset-x-0 bottom-0 pointer-events-none h-full transition-opacity duration-emphasis ${active ? "opacity-25" : "opacity-30"}`}>
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
    <div className="relative z-content flex flex-col">
      <Eyebrow className={`mb-1 drop-shadow-sm ${eyebrowClassName}`}>
        {eyebrow}
      </Eyebrow>
      <SectionTitle className="text-content-primary drop-shadow-md">
        {title}
      </SectionTitle>
      {subtitle ? <span className="text-body font-medium text-content-primary drop-shadow-sm">{subtitle}</span> : null}
    </div>
  );
}
