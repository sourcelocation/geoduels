import { motion } from "framer-motion";
import { formatDamageMultiplierLabel } from "./damage-multiplier";
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
      className={`w-full rounded-[24px] border border-white/10 bg-white/5 p-6 text-center shadow-2xl backdrop-blur-md md:w-72 ${position === "right" ? "md:text-right" : "md:text-left"}`}
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
      className="absolute inset-0 z-50 grid place-content-center justify-items-center gap-[40px] bg-[#0d1216] px-3 pointer-events-none"
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
          className="relative grid place-items-center rounded-full border border-white/20 bg-[#162130]/80 p-8 text-white shadow-[0_0_40px_rgba(42,209,143,0.3)] backdrop-blur-md"
        >
          <div className="absolute inset-0 rounded-full bg-[radial-gradient(circle_at_center,rgba(42,209,143,0.15)_0%,transparent_70%)] animate-pulse" />
          <span className="mb-1 text-sm font-extrabold uppercase tracking-[0.2em] text-[#a9bfd4]">
            VS
          </span>
          <motion.span
            key={countdownLeft}
            initial={{ scale: 1.5, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            transition={{ type: "spring", stiffness: 400, damping: 25 }}
            className="text-6xl font-black leading-none drop-shadow-md"
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
          <span className="text-xl font-bold uppercase tracking-[0.2em] text-[#a9bfd4]">
            Damage Multiplier
          </span>
          <span className="mt-1 text-5xl font-black text-white drop-shadow-[0_0_15px_rgba(42,209,143,0.5)]">
            {damageMultiplierLabel}
          </span>
        </motion.div>
      )}
    </motion.div>
  );
}
