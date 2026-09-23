import type { GeoDuelsExtensionCapabilities } from "../lib/geoduels-extension-protocol";
import type { HotkeyPreferences } from "../../hotkeys/model/types";

type Props = {
  capabilities: GeoDuelsExtensionCapabilities;
  heading: number;
  style: HotkeyPreferences["compassStyle"];
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
  style,
}: Props) {
  if (!capabilities.heading) return null;
  const centerCycle = Math.floor(heading / 360);
  const cycles = [centerCycle - 1, centerCycle, centerCycle + 1];
  const stripOffset = COMPASS_WIDTH / 2 - heading * PIXELS_PER_DEGREE;

  return <>
    {style !== "traditional" ? (
      <div
        data-testid="extension-compass"
        className="pointer-events-none absolute left-1/2 top-3 z-game h-9 w-[260px] -translate-x-1/2 overflow-hidden rounded-full border border-hud-border bg-hud-surface text-content-primary shadow-elev-2"
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
                className="font-hud absolute top-1/2 -translate-x-1/2 -translate-y-1/2 text-hud-label font-strong text-content-primary"
                style={{ left: (cycle * 360 + marker.degree) * PIXELS_PER_DEGREE }}
              >
                {marker.label}
              </span>
            )),
          )}
        </div>
        <span className="absolute bottom-0 left-1/2 top-0 w-0.5 -translate-x-1/2 bg-action-primary" />
      </div>
    ) : null}
    {style !== "modern" ? (
      <div
        data-testid="traditional-compass"
        className="pointer-events-none absolute bottom-20 left-3 z-game flex h-20 w-20 items-center justify-center rounded-full border border-hud-border bg-surface-page shadow-elev-2 md:left-4"
        role="img"
        aria-label={`Traditional compass heading ${Math.round(((heading % 360) + 360) % 360)} degrees`}
      >
        <svg width="54" height="54" viewBox="0 0 54 54" aria-hidden="true" style={{ transform: `rotate(${-heading}deg)` }}>
          <path d="M27 5 17 27h20Z" className="fill-status-danger" />
          <path d="M27 49 17 27h20Z" className="fill-content-primary" />
        </svg>
      </div>
    ) : null}
  </>;
}
