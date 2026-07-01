import type { GeoDuelsExtensionCapabilities } from "../lib/geoduels-extension-protocol";

type Props = {
  capabilities: GeoDuelsExtensionCapabilities;
  heading: number;
};

const MARKERS = [
  { label: "N", degree: 0 },
  { label: "NE", degree: 45 },
  { label: "E", degree: 90 },
  { label: "SE", degree: 135 },
  { label: "S", degree: 180 },
  { label: "SW", degree: 225 },
  { label: "W", degree: 270 },
  { label: "NW", degree: 315 },
];
const PIXELS_PER_DEGREE = 2;
const COMPASS_WIDTH = 260;

export function StreetViewEnhancements({
  capabilities,
  heading,
}: Props) {
  if (!capabilities.heading) return null;
  const centerCycle = Math.floor(heading / 360);
  const cycles = [centerCycle - 1, centerCycle, centerCycle + 1];
  const stripOffset = COMPASS_WIDTH / 2 - heading * PIXELS_PER_DEGREE;

  return (
    <div
      data-testid="extension-compass"
      className="pointer-events-none absolute left-1/2 top-3 z-50 h-9 w-[260px] -translate-x-1/2 overflow-hidden rounded-pill border border-white/15 bg-hudBg text-white shadow-elev-2"
      aria-label={`Compass heading ${Math.round(((heading % 360) + 360) % 360)} degrees`}
    >
      <div
        className="absolute inset-y-0 left-0"
        style={{ transform: `translateX(${stripOffset}px)` }}
      >
        {cycles.flatMap((cycle) =>
          MARKERS.map((marker) => (
            <span
              key={`${cycle}-${marker.label}`}
              className="font-hud absolute top-1/2 -translate-x-1/2 -translate-y-1/2 text-[11px] font-black tracking-[0.08em] text-white/85"
              style={{
                left: (cycle * 360 + marker.degree) * PIXELS_PER_DEGREE,
              }}
            >
              {marker.label}
            </span>
          )),
        )}
      </div>
      <span className="absolute bottom-0 left-1/2 top-0 w-0.5 -translate-x-1/2 bg-emerald-300" />
    </div>
  );
}
