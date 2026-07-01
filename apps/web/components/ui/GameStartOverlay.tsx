import { motion } from "framer-motion";
import DuelOverlayBackground from "./DuelOverlayBackground";
import MatchSideProfile from "./MatchSideProfile";
import IntroCountdownText from "./IntroCountdownText";
import type { MatchSidesView } from "./ParticipantIdentity";

type Props = {
  roundNumber: number;
  modeName: string;
  mapName?: string;
  countdownSec: number;
  sides: MatchSidesView;
  isFreeForAll?: boolean;
};

export default function GameStartOverlay({
  roundNumber,
  modeName,
  mapName,
  countdownSec,
  sides,
  isFreeForAll = false,
}: Props) {
  const content = (
    <div className="absolute inset-0 flex flex-col items-center justify-center">
      {/* Header Info */}
      <div className="absolute top-12 flex flex-col items-center">
        <img
          src="/icon.v1.png"
          alt="Trophy"
          className="h-16 w-16 md:h-20 md:w-20 drop-shadow-lg mb-2"
        />
        <h1 className="text-3xl md:text-5xl font-hud text-white drop-shadow-md mb-1">
          Round {roundNumber}
        </h1>
        <h2 className="text-xl md:text-2xl font-hud text-white/90 drop-shadow-md">
          {modeName}
        </h2>
        {mapName && (
          <h3 className="text-sm md:text-base font-hud italic text-white/30 uppercase tracking-widest mt-1">
            {mapName}
          </h3>
        )}
      </div>

      <div className="w-full max-w-7xl px-4 md:px-12 flex justify-center items-center mt-12">
        {/* Left Player */}
        {!isFreeForAll && (
          <div className="flex-1 flex justify-start">
            <MatchSideProfile
              side={sides.self}
            />
          </div>
        )}

        {/* Countdown */}
        <div className={isFreeForAll ? "flex items-center justify-center" : "flex-shrink-0 flex items-center justify-center mx-8"}>
          <IntroCountdownText countdownSec={countdownSec} />
        </div>

        {/* Right Player */}
        {!isFreeForAll && (
          <div className="flex-1 flex justify-end">
            <MatchSideProfile
              side={sides.opponent}
              opponent
            />
          </div>
        )}
      </div>
    </div>
  );

  return (
    <motion.div
      key="game-start-overlay"
      initial={{ opacity: 1 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0, y: 48 }}
      transition={{ duration: 0.3, ease: "easeOut" }}
      className="absolute inset-0 z-[100] pointer-events-none overflow-hidden"
    >
      <DuelOverlayBackground variant={isFreeForAll ? "points" : "duel"}>
        {content}
      </DuelOverlayBackground>
    </motion.div>
  );
}
