import { motion } from "framer-motion";
import { formatDamageMultiplierLabel } from "../../lib/damage-multiplier";
import {
  ParticipantIdentityCard,
  type MatchSideView,
  type MatchSidesView,
} from "./ParticipantIdentity";

type PlayerCardProps = {
  position: "left" | "right";
  side: MatchSideView;
  opponent?: boolean;
};

function PlayerCard({
  position,
  side,
  opponent,
}: PlayerCardProps) {
  return (
    <motion.div
      initial={{ opacity: 0, x: position === "left" ? -20 : 20 }}
      animate={{ opacity: 1, x: 0 }}
      transition={{ type: "spring", stiffness: 300, damping: 25, delay: 0.1 }}
      className={`w-full rounded-xl border border-border-default bg-surface-grouped p-6 text-content-primary shadow-elev-3 md:w-72 ${position === "right" ? "md:text-right" : "md:text-left"}`}
    >
      <ParticipantIdentityCard participant={side.participant} opponent={opponent} />
    </motion.div>
  );
}

type Props = {
  sides: MatchSidesView;
  countdownLeft: number;
  damageMultiplier: number;
};

export default function PrematchVersusOverlay({
  sides,
  countdownLeft,
  damageMultiplier,
}: Props) {
  const damageMultiplierLabel = formatDamageMultiplierLabel(damageMultiplier);

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0, y: 40 }}
      transition={{ duration: 0.4, ease: "easeOut" }}
      className="absolute inset-0 z-game grid place-content-center justify-items-center gap-10 bg-surface-page px-3 pointer-events-none"
    >
      <div className="flex w-full max-w-5xl flex-col items-center gap-6 md:flex-row md:justify-center md:gap-12">
        <PlayerCard
          position="left"
          side={sides.self}
        />

        <motion.div
          initial={{ scale: 0.8, opacity: 0 }}
          animate={{ scale: 1, opacity: 1 }}
          transition={{ type: "spring", stiffness: 300, damping: 20 }}
          className="relative grid place-items-center rounded-full border border-status-success/30 bg-surface-raised p-8 text-content-primary shadow-elev-3"
        >
          <div className="absolute inset-0 rounded-full bg-status-success/15 animate-pulse" />
          <span className="mb-1 text-label font-strong uppercase tracking-display-wide text-content-secondary">
            VS
          </span>
          <motion.span
            key={countdownLeft}
            initial={{ scale: 1.5, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            transition={{ type: "spring", stiffness: 400, damping: 25 }}
            className="font-display text-display-lg font-strong leading-collapsed drop-shadow-md"
          >
            {countdownLeft}
          </motion.span>
        </motion.div>

        <PlayerCard
          position="right"
          side={sides.opponent}
          opponent
        />
      </div>

      {damageMultiplierLabel && (
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{
            delay: 0.2,
            type: "spring",
            stiffness: 300,
            damping: 25,
          }}
          className="flex flex-col items-center"
        >
          <span className="text-heading-sm font-strong uppercase tracking-display-wide text-content-secondary">
            Damage Multiplier
          </span>
          <span className="mt-1 font-display text-display-md font-strong text-content-primary drop-shadow-md">
            {damageMultiplierLabel}
          </span>
        </motion.div>
      )}
    </motion.div>
  );
}
