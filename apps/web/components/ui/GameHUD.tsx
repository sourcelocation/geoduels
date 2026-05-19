import { AnimatePresence, motion } from 'framer-motion';
import { formatDamageMultiplierLabel } from './damage-multiplier';

type Props = {
  mm: string;
  ss: string;
  damageMultiplier: number;
  isRoundTimerRunning?: boolean;
  timerProgressPct: number;
  isTimerCritical: boolean;
  isTimerPulseActive: boolean;
  hideMultiplier?: boolean;
};

export default function GameHUD({
  mm,
  ss,
  damageMultiplier,
  isRoundTimerRunning = true,
  timerProgressPct,
  isTimerCritical,
  isTimerPulseActive,
  hideMultiplier = false,
}: Props) {
  const ringColor = isTimerCritical ? '#ff6d42' : '#2ad18f';
  const progress = Number(Math.max(0, Math.min(100, timerProgressPct)).toFixed(2));
  const width = 120;
  const height = 48;
  const stroke = 4;
  const inset = stroke / 2;
  const radius = (height - stroke) / 2;
  const centerX = width / 2;
  const rightX = width - inset - radius;
  const leftX = inset + radius;
  const topY = inset;
  const bottomY = height - inset;
  const progressPath = `M ${centerX} ${topY} H ${leftX} A ${radius} ${radius} 0 0 0 ${leftX} ${bottomY} H ${rightX} A ${radius} ${radius} 0 0 0 ${rightX} ${topY} H ${centerX}`;
  const multiplierLabel = formatDamageMultiplierLabel(damageMultiplier);

  return (
    <div className="pointer-events-none absolute inset-x-0 top-[91px] z-40 animate-hudSlideIn md:top-[76px]">
      <div className="absolute left-1/2 -translate-x-1/2">
        <div className="flex items-center gap-2.5 md:gap-3">
          <AnimatePresence initial={false}>
            {isRoundTimerRunning && (
              <motion.div
                key="round-timer"
                className="relative shrink-0 overflow-visible"
                initial={{ width: 0, opacity: 0, x: 10 }}
                animate={{ width, opacity: 1, x: 0 }}
                exit={{ width: 0, opacity: 0, x: 10 }}
                transition={{ duration: 0.22, ease: 'easeOut' }}
                style={{ height }}
              >
                <div className="relative shrink-0" style={{ width, height }}>
                  {isTimerPulseActive && (
                    <div
                      data-testid="timer-pulse-glow"
                      className="pointer-events-none absolute inset-0 rounded-pill animate-timerCritical"
                    />
                  )}
                  <div
                    data-testid="timer-pill"
                    className="font-hud relative grid place-items-center rounded-pill shadow-elev-2 backdrop-blur-hud bg-hudBg tracking-[0.08em] text-ink overflow-hidden"
                    style={{ width, height, fontSize: 20 }}
                  >
                    <svg
                      className="pointer-events-none absolute inset-0"
                      viewBox={`0 0 ${width} ${height}`}
                      aria-hidden="true"
                    >
                      <path
                        d={progressPath}
                        fill="none"
                        stroke={ringColor}
                        strokeWidth={stroke}
                        strokeLinecap="round"
                        strokeDasharray={`${progress} 100`}
                        pathLength={100}
                      />
                    </svg>
                    <span className="relative z-10">
                      {mm}:{ss}
                    </span>
                  </div>
                </div>
              </motion.div>
            )}
          </AnimatePresence>

          {!hideMultiplier && multiplierLabel && (
            <div
              data-testid="multiplier-badge"
              className="font-hud relative grid h-[54px] w-[58px] shrink-0 place-items-center text-[20px] tracking-[-0.03em] text-[#dfffee] drop-shadow-[0_8px_16px_rgba(0,0,0,0.28)] md:h-[60px] md:w-[66px] md:text-[20px]"
            >
              <div
                className="absolute inset-0 backdrop-blur-hud bg-hudBg"
                style={{
                  clipPath: 'polygon(50% 0%, 91% 25%, 91% 75%, 50% 100%, 9% 75%, 9% 25%)',
                  boxShadow: 'inset 0 1px 0 rgba(255,255,255,0.14), inset 0 -1px 0 rgba(0,0,0,0.2), 0 12px 24px rgba(0,0,0,0.28)'
                }}
              />
              <div
                className="absolute left-1/2 top-1/2 h-[20px] w-[40px] -translate-x-1/2 -translate-y-1/2 blur-[12px]"
                style={{ background: 'rgba(48, 255, 173, 0.36)' }}
              />
              <span
                className="relative z-10"
                style={{ textShadow: '0 0 12px rgba(80, 255, 191, 0.75), 0 0 24px rgba(80, 255, 191, 0.4)' }}
              >
                {multiplierLabel}
              </span>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
