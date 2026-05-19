type DirectionHUDProps = {
  heading: number;
};

const COMPASS_MARKS = Array.from({ length: 72 }, (_, index) => index * 5);
const VISIBLE_DEGREES = 120;
const DIRECTION_LABELS = ['N', 'NE', 'E', 'SE', 'S', 'SW', 'W', 'NW'];

function normalizeHeading(value: number) {
  if (!Number.isFinite(value)) return 0;
  return ((value % 360) + 360) % 360;
}

function normalizeDelta(value: number) {
  return ((value + 540) % 360) - 180;
}

function getMarkLabel(degree: number) {
  switch (degree) {
    case 0:
      return 'N';
    case 45:
      return 'NE';
    case 90:
      return 'E';
    case 135:
      return 'SE';
    case 180:
      return 'S';
    case 225:
      return 'SW';
    case 270:
      return 'W';
    case 315:
      return 'NW';
    default:
      return '';
  }
}

export function formatHeadingLabel(heading: number) {
  const normalized = normalizeHeading(heading);
  const direction = DIRECTION_LABELS[Math.round(normalized / 45) % DIRECTION_LABELS.length];
  return `${direction} ${Math.round(normalized)}°`;
}

export default function DirectionHUD({ heading }: DirectionHUDProps) {
  const normalizedHeading = normalizeHeading(heading);
  const halfVisibleDegrees = VISIBLE_DEGREES / 2;

  return (
    <div className="pointer-events-none absolute inset-x-0 top-3 z-40 flex justify-center px-3 md:top-4">
      <div
        data-testid="direction-hud"
        className="relative h-12 w-[min(calc(100vw-1.5rem),26rem)] overflow-hidden rounded-lg border border-white/15 bg-[#111820]/75 text-white shadow-elev-2 backdrop-blur-hud"
        aria-label={`Facing ${formatHeadingLabel(normalizedHeading)}`}
      >
        <div className="absolute inset-0 bg-[radial-gradient(circle_at_50%_0%,rgba(255,255,255,0.16),transparent_44%)]" />
        <div className="absolute inset-x-0 top-0 h-px bg-white/25" />
        <div className="absolute inset-y-0 left-0 z-10 w-16 bg-gradient-to-r from-[#111820] to-transparent" />
        <div className="absolute inset-y-0 right-0 z-10 w-16 bg-gradient-to-l from-[#111820] to-transparent" />

        {COMPASS_MARKS.map((degree) => {
          const delta = normalizeDelta(degree - normalizedHeading);
          if (Math.abs(delta) > halfVisibleDegrees) return null;

          const left = 50 + (delta / VISIBLE_DEGREES) * 100;
          const label = getMarkLabel(degree);
          const isCardinal = degree % 90 === 0;
          const isIntercardinal = degree % 45 === 0;
          const isNorth = degree === 0;

          return (
            <div
              key={degree}
              className="absolute top-2 flex -translate-x-1/2 flex-col items-center"
              style={{ left: `${left}%` }}
              aria-hidden="true"
            >
              <div
                className={[
                  'w-px rounded-full',
                  isNorth
                    ? 'h-6 bg-[#ff4f45] shadow-[0_0_12px_rgba(255,79,69,0.7)]'
                    : isCardinal
                      ? 'h-5 bg-white/90'
                      : isIntercardinal
                        ? 'h-4 bg-white/60'
                        : 'h-2.5 bg-white/35',
                ].join(' ')}
              />
              {label ? (
                <span
                  className={[
                    'font-hud mt-0.5 text-[10px] font-black leading-none tracking-[0.12em]',
                    isNorth ? 'text-[#ffb1ad]' : 'text-white/80',
                  ].join(' ')}
                >
                  {label}
                </span>
              ) : null}
            </div>
          );
        })}

        <div className="absolute left-1/2 top-0 z-20 h-full w-px -translate-x-1/2 bg-white/80 shadow-[0_0_12px_rgba(255,255,255,0.55)]" />
        <div className="absolute left-1/2 top-0 z-20 -translate-x-1/2 border-x-[6px] border-t-[8px] border-x-transparent border-t-[#ff4f45]" />
        <div
          data-testid="direction-hud-heading"
          className="font-hud absolute bottom-1 left-1/2 z-20 -translate-x-1/2 whitespace-nowrap rounded bg-black/35 px-2 py-0.5 text-[10px] font-black leading-none tracking-[0.12em] text-white/90"
        >
          {formatHeadingLabel(normalizedHeading)}
        </div>
      </div>
    </div>
  );
}
