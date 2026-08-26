import { AnimatePresence, motion, type Variants } from 'framer-motion';
import { useLayoutEffect, useRef, useState } from 'react';
import { formatDamageMultiplierLabel } from '../../lib/damage-multiplier';
import { RESULT_ANIMATION_CONFIG } from '../../lib/round-result-animation-config';
import type { RoundResultAnimationConfig, RoundResultOverlayProps } from '../../model/types';
import { ResultDistanceBar } from './ResultDistanceBar';

type Side = 'self' | 'opp';

type ScoreTravelRect = {
  startX: number;
  startY: number;
  dx: number;
  dy: number;
  width: number;
  height: number;
};

type MotionEase = 'linear' | 'easeIn' | 'easeOut' | 'easeInOut' | [number, number, number, number];

function parseBezier(value: string): [number, number, number, number] | null {
  const match = value.match(/^cubic-bezier\(([^)]+)\)$/);
  if (!match) return null;
  const parts = match[1]
    .split(',')
    .map((part) => Number(part.trim()))
    .filter((part) => Number.isFinite(part));
  if (parts.length !== 4) return null;
  return [parts[0], parts[1], parts[2], parts[3]];
}

function resolveEase(ease: string | [number, number, number, number]): MotionEase {
  if (Array.isArray(ease)) return ease;
  if (ease === 'linear' || ease === 'easeIn' || ease === 'easeOut' || ease === 'easeInOut') return ease;
  return parseBezier(ease) || [0.25, 0.1, 0.25, 1];
}

function getStaticScoreValue(args: {
  side: Side;
  phase: RoundResultOverlayProps['phase'];
  winner: RoundResultOverlayProps['winner'];
  showCrush: boolean;
  sourceSide: Side;
  targetSide: Side;
  impactReached: boolean;
  damage: number;
  selfScore?: number;
  oppScore?: number;
}) {
  const {
    side,
    phase,
    winner,
    showCrush,
    sourceSide,
    targetSide,
    impactReached,
    damage,
    selfScore,
    oppScore
  } = args;
  if (selfScore === undefined || oppScore === undefined) return '';
  if (phase === 'base') return '';
  if (winner === 'tie' || !showCrush) return side === 'self' ? selfScore : oppScore;
  if (side === sourceSide) return '';
  if (side === targetSide && impactReached) return '';
  return side === 'self' ? selfScore : oppScore;
}

function getScoreSizeClass(side: Side, winner: RoundResultOverlayProps['winner'], sourceSide: Side) {
  if (winner === 'tie') return 'text-display-md';
  return side === sourceSide ? 'text-display-lg' : 'text-display-md';
}

function getScoreTravelDurationMs(animation: RoundResultAnimationConfig = RESULT_ANIMATION_CONFIG) {
  return Math.max(animation.scoreTravel.minDurationMs, animation.scoreTravel.durationMs * animation.scoreTravel.durationScale);
}

function getAppliedDamage(damage: number, damageMultiplier: number) {
  return Math.round(damage * damageMultiplier);
}



export { getScoreSizeClass, getScoreTravelDurationMs, getStaticScoreValue };

export default function RoundResultOverlay({
  roundNumber,
  mapNode,
  phase,
  showScoreReveal,
  winner,
  damage,
  damageMultiplier,
  sides,
  hpPct
}: RoundResultOverlayProps) {
  const animation = RESULT_ANIMATION_CONFIG;
  // derivedState
  const scoreTrackRef = useRef<HTMLDivElement | null>(null);
  const selfScoreTextRef = useRef<HTMLSpanElement | null>(null);
  const oppScoreTextRef = useRef<HTMLSpanElement | null>(null);
  const damageMeasureRef = useRef<HTMLSpanElement | null>(null);
  const [scoreTravel, setScoreTravel] = useState<ScoreTravelRect | null>(null);

  const isSelfWinner = winner === 'self';
  const isOppWinner = winner === 'opp';
  const showScores = showScoreReveal && sides.self.score !== undefined && sides.opponent.score !== undefined;
  const showCrush =
    phase === 'crush' || phase === 'damage_travel' || phase === 'damage_multiplier' || phase === 'hp_apply';
  const sourceSide: Side = isSelfWinner ? 'self' : 'opp';
  const targetSide: Side = isSelfWinner ? 'opp' : 'self';
  const impactReached = phase === 'damage_travel' || phase === 'damage_multiplier' || phase === 'hp_apply';
  const showDamageMultiplier = phase === 'damage_multiplier' || phase === 'hp_apply';
  const damageMultiplierLabel = showDamageMultiplier ? formatDamageMultiplierLabel(damageMultiplier) : null;
  const hasWinnerMotion = showScores && showCrush && winner !== 'tie' && damage > 0;
  const displayedDamage = showDamageMultiplier ? getAppliedDamage(damage, damageMultiplier) : damage;
  const scoreTravelDurationMs = getScoreTravelDurationMs(animation);
  const crushCrossFadeStart = 0.78;

  // motionHelpers
  const scoreVariants: Variants = {
    base: { y: animation.scoreReveal.yFrom, opacity: 0, scale: animation.scoreReveal.scaleFrom },
    scores: {
      y: 0,
      opacity: 1,
      scale: 1,
      transition: {
        type: 'spring',
        stiffness: animation.scoreReveal.springStiffness,
        damping: animation.scoreReveal.springDamping
      }
    }
  };

  const staticScoreVariant = phase === 'base' ? 'base' : 'scores';

  const sourceScoreValue = sourceSide === 'self' ? sides.self.score : sides.opponent.score;
  const selfStaticScoreValue = getStaticScoreValue({
    side: 'self',
    phase,
    winner,
    showCrush,
    sourceSide,
    targetSide,
    impactReached,
    damage: displayedDamage,
    selfScore: sides.self.score,
    oppScore: sides.opponent.score
  });
  const oppStaticScoreValue = getStaticScoreValue({
    side: 'opp',
    phase,
    winner,
    showCrush,
    sourceSide,
    targetSide,
    impactReached,
    damage: displayedDamage,
    selfScore: sides.self.score,
    oppScore: sides.opponent.score
  });

  // layoutMeasurement
  useLayoutEffect(() => {
    if (!hasWinnerMotion) {
      setScoreTravel(null);
      return;
    }
    const track = scoreTrackRef.current;
    const source = sourceSide === 'self' ? selfScoreTextRef.current : oppScoreTextRef.current;
    const target = targetSide === 'self' ? selfScoreTextRef.current : oppScoreTextRef.current;
    if (!track || !source || !target) return;

    const trackRect = track.getBoundingClientRect();
    const sourceRect = source.getBoundingClientRect();
    const targetRect = target.getBoundingClientRect();
    const damageRect = damageMeasureRef.current?.getBoundingClientRect();
    const travelWidth = Math.max(sourceRect.width, targetRect.width, damageRect?.width || 0);
    const travelHeight = Math.max(sourceRect.height, targetRect.height, damageRect?.height || 0);

    const sourceCenterX = sourceRect.left - trackRect.left + sourceRect.width / 2;
    const sourceCenterY = sourceRect.top - trackRect.top + sourceRect.height / 2;
    const targetCenterX = targetRect.left - trackRect.left + targetRect.width / 2;
    const targetCenterY = targetRect.top - trackRect.top + targetRect.height / 2;

    const startX = sourceCenterX - travelWidth / 2;
    const startY = sourceCenterY - travelHeight / 2;
    const endX = targetCenterX - travelWidth / 2;
    const endY = targetCenterY - travelHeight / 2;
    setScoreTravel({
      startX,
      startY,
      dx: endX - startX,
      dy: endY - startY,
      width: travelWidth,
      height: travelHeight
    });
  }, [hasWinnerMotion, sourceSide, targetSide, showScores, phase, damage]);

  // render sections
  return (
    <motion.div
      key={`round-result-overlay-${roundNumber}`}
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0, y: 48 }}
      transition={{ duration: animation.overlayEnter.fadeDuration, ease: 'easeOut' }}
      className="absolute inset-0 z-sticky pointer-events-none overflow-hidden"
    >
      <motion.div
        initial={{ scale: animation.overlayEnter.mapScaleFrom, opacity: 1 }}
        animate={{ scale: 1, opacity: 1 }}
        transition={{
          type: 'spring',
          stiffness: animation.overlayEnter.mapSpringStiffness,
          damping: animation.overlayEnter.mapSpringDamping,
          delay: animation.overlayEnter.mapDelay
        }}
        className="absolute bottom-0 left-0 right-0 top-[148px] z-content pointer-events-auto bg-surface-raised md:top-[176px]"
      >
        {mapNode}
      </motion.div>

      <div className="absolute left-0 right-0 top-0 z-game-controls bg-brand-blue/30 border-b-[4px] border-brand-blue px-3 pb-2 pt-3 md:px-5 md:pt-4">
        <div className="flex items-center justify-center">
          <motion.div
            initial={{ y: animation.roundBadge.yFrom, opacity: 0 }}
            animate={{ y: 0, opacity: 1 }}
            transition={{ duration: animation.roundBadge.duration, ease: resolveEase(animation.roundBadge.ease) }}
            className="inline-flex flex-col items-center justify-center px-1"
          >
            <div className="flex flex-col items-center">
              <div className="mb-1">
                <img src="/icon.v1.png" alt="Trophy" className="h-8 w-8 drop-shadow-lg md:h-10 md:w-10" />
              </div>
              <span className="font-hud text-body whitespace-nowrap italic text-content-primary drop-shadow-md">
                Round {roundNumber}
              </span>
            </div>
          </motion.div>
        </div>

        <div className="mx-auto w-full max-w-5xl px-2">
          <div ref={scoreTrackRef} className="relative h-20">
            <span
              ref={damageMeasureRef}
              aria-hidden
              className="font-hud absolute left-0 top-0 invisible text-display-lg leading-collapsed"
            >
              {damage}
            </span>
            <div className="flex w-full justify-between items-center px-8 md:px-24">
              <div className="flex h-16 flex-1 items-center justify-center">
                <AnimatePresence>
                  {showScores && (
                    <motion.div
                      variants={scoreVariants}
                      initial="base"
                      animate={staticScoreVariant}
                      className={`font-hud flex h-14 min-w-10 items-center justify-center px-2 text-content-primary ${getScoreSizeClass('self', winner, sourceSide)} drop-shadow-md`}
                      style={{ transformOrigin: 'center bottom' }}
                    >
                      <motion.span
                        ref={selfScoreTextRef}
                        animate={
                          hasWinnerMotion && targetSide === 'self' && phase === 'crush'
                            ? { opacity: [1, 1, 0] }
                            : { opacity: 1 }
                        }
                        transition={{
                          duration: scoreTravelDurationMs / 1000,
                          times: [0, crushCrossFadeStart, 1],
                          ease: 'linear'
                        }}
                        className={showCrush && sourceSide === 'self' ? 'invisible leading-collapsed' : 'leading-collapsed'}
                      >
                        {selfStaticScoreValue === '' || (hasWinnerMotion && targetSide === 'self' && phase !== 'crush')
                          ? ''
                          : selfStaticScoreValue}
                      </motion.span>
                    </motion.div>
                  )}
                </AnimatePresence>
              </div>

              <div className="flex h-16 flex-1 items-center justify-center">
                <AnimatePresence>
                  {showScores && (
                    <motion.div
                      variants={scoreVariants}
                      initial="base"
                      animate={staticScoreVariant}
                      className={`font-hud flex h-14 min-w-10 items-center justify-center px-2 text-content-primary ${getScoreSizeClass('opp', winner, sourceSide)} drop-shadow-md`}
                      style={{ transformOrigin: 'center bottom' }}
                    >
                      <motion.span
                        ref={oppScoreTextRef}
                        animate={
                          hasWinnerMotion && targetSide === 'opp' && phase === 'crush'
                            ? { opacity: [1, 1, 0] }
                            : { opacity: 1 }
                        }
                        transition={{
                          duration: scoreTravelDurationMs / 1000,
                          times: [0, crushCrossFadeStart, 1],
                          ease: 'linear'
                        }}
                        className={showCrush && sourceSide === 'opp' ? 'invisible leading-collapsed' : 'leading-collapsed'}
                      >
                        {oppStaticScoreValue === '' || (hasWinnerMotion && targetSide === 'opp' && phase !== 'crush')
                          ? ''
                          : oppStaticScoreValue}
                      </motion.span>
                    </motion.div>
                  )}
                </AnimatePresence>
              </div>
            </div>

            <AnimatePresence>
              {hasWinnerMotion &&
                scoreTravel &&
                (phase === 'crush' ||
                  phase === 'damage_travel' ||
                  phase === 'damage_multiplier' ||
                  phase === 'hp_apply') &&
                sourceScoreValue !== undefined ? (
                <motion.div
                  data-testid="score-travel-token"
                  key={`${sourceSide}-${targetSide}`}
                  initial={{ x: 0, y: 0, opacity: 1 }}
                  animate={
                    phase === 'crush'
                      ? { x: scoreTravel.dx, y: scoreTravel.dy, scale: 1, opacity: 1 }
                      : phase === 'hp_apply'
                        ? {
                          x: scoreTravel.dx,
                          y: scoreTravel.dy + animation.scoreImpact.y,
                          scale: animation.scoreImpact.scale,
                          opacity: 0
                        }
                        : { x: scoreTravel.dx, y: scoreTravel.dy, scale: 1, opacity: 1 }
                  }
                  exit={{
                    opacity: 0,
                    transition: {
                      duration: animation.scoreTravel.exitDurationMs / 1000,
                      ease: resolveEase(animation.scoreTravel.exitEase)
                    }
                  }}
                  transition={
                    phase === 'crush'
                      ? { duration: scoreTravelDurationMs / 1000, ease: resolveEase(animation.scoreTravel.ease) }
                      : phase === 'hp_apply'
                        ? { duration: animation.scoreImpact.durationMs / 1000, ease: resolveEase(animation.scoreImpact.ease) }
                        : { duration: 0 }
                  }
                  style={{
                    left: scoreTravel.startX,
                    top: scoreTravel.startY,
                    width: scoreTravel.width,
                    height: scoreTravel.height
                  }}
                  className="font-hud absolute grid place-items-center text-display-lg leading-collapsed text-content-primary drop-shadow-md"
                >
                  <span className="relative grid place-items-center">
                    {damageMultiplierLabel && (
                      <span
                        data-testid="damage-multiplier-label"
                        className={`font-hud absolute top-1/2 text-heading-md leading-collapsed text-content-primary drop-shadow-md ${targetSide === 'self'
                          ? 'right-full -translate-x-3 -translate-y-1/2'
                          : 'left-full translate-x-3 -translate-y-1/2'
                          }`}
                      >
                        {damageMultiplierLabel}
                      </span>
                    )}
                    <motion.span
                      animate={
                        phase === 'crush'
                          ? { opacity: [1, 1, 0] }
                          : { opacity: 0 }
                      }
                      transition={{
                        duration: scoreTravelDurationMs / 1000,
                        times: [0, crushCrossFadeStart, 1],
                        ease: 'linear'
                      }}
                      className="col-start-1 row-start-1"
                    >
                      {sourceScoreValue}
                    </motion.span>
                    <motion.span
                      animate={
                        phase === 'crush'
                          ? { opacity: [0, 0, 1] }
                          : { opacity: 1 }
                      }
                      transition={{
                        duration: scoreTravelDurationMs / 1000,
                        times: [0, crushCrossFadeStart, 1],
                        ease: 'linear'
                      }}
                      className="col-start-1 row-start-1"
                    >
                      {displayedDamage}
                    </motion.span>
                  </span>
                </motion.div>
              ) : null}
            </AnimatePresence>
          </div>
        </div>

        {/* Distance Bar (Straddling the bottom border) */}
        <div className="absolute bottom-0 left-1/2 -translate-x-1/2 translate-y-1/2 z-game">
          <AnimatePresence>
            {showScores && sides.self.distanceKm !== undefined && sides.opponent.distanceKm !== undefined && (
              <motion.div
                initial={{ y: -30, opacity: 0, scale: 0.95 }}
                animate={{ y: 0, opacity: 1, scale: 1 }}
                transition={{ type: 'spring', stiffness: 500, damping: 30, delay: 0.1 }}
                className="relative"
              >
                <ResultDistanceBar selfDistanceKm={sides.self.distanceKm} oppDistanceKm={sides.opponent.distanceKm} />
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      </div>
    </motion.div >
  );
}
