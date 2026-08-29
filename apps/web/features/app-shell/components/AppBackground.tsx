import { useEffect, useState } from "react";

import { cn } from "../../../lib/cn";

const backgroundImage = "/bg5.v2.webp";
const backgroundPlaceholder = "/bg5.placeholder.v2.webp";
const backgroundOverlay = "var(--gd-background-overlay)";

let backgroundLoaded = false;
let backgroundLoadPromise: Promise<void> | null = null;

function loadBackground() {
  if (backgroundLoaded) return Promise.resolve();
  if (backgroundLoadPromise) return backgroundLoadPromise;

  backgroundLoadPromise = new Promise<void>((resolve) => {
    const image = new Image();
    image.onload = async () => {
      try {
        await image.decode();
      } catch {
        // onload is sufficient when decode is unavailable.
      }
      backgroundLoaded = true;
      resolve();
    };
    image.onerror = () => {
      backgroundLoadPromise = null;
      resolve();
    };
    image.src = backgroundImage;
  });
  return backgroundLoadPromise;
}

/** Shared application backdrop with an optional immediate scene-wide blur. */
export function AppBackground({ blurred = false }: { blurred?: boolean }) {
  const [highQualityReady, setHighQualityReady] = useState(backgroundLoaded);

  useEffect(() => {
    let cancelled = false;
    if (backgroundLoaded) {
      setHighQualityReady(true);
      return;
    }
    const timer = window.setTimeout(() => {
      void loadBackground().then(() => {
        if (!cancelled && backgroundLoaded) setHighQualityReady(true);
      });
    }, 350);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, []);

  const transform = blurred ? "scale(1.12)" : "scale(1.06)";
  const sharedStyle = {
    backgroundPosition: "center top",
    backgroundRepeat: "no-repeat",
    backgroundSize: "cover",
    transform,
    transformOrigin: "center top",
  } as const;

  return (
    <div className="pointer-events-none fixed inset-0 z-base overflow-hidden">
      <div
        className="absolute inset-0"
        style={{
          ...sharedStyle,
          backgroundImage: `${backgroundOverlay}, url('${backgroundPlaceholder}')`,
          filter: blurred ? "var(--gd-background-filter-focus)" : "var(--gd-background-filter)",
        }}
      />
      <div
        className={cn(
          "absolute inset-0 transition-opacity duration-dramatic ease-standard",
          highQualityReady ? "opacity-100" : "opacity-0",
        )}
        style={{
          ...sharedStyle,
          backgroundImage: `${backgroundOverlay}, url('${backgroundImage}')`,
          filter: blurred ? "var(--gd-background-filter-focus)" : undefined,
        }}
      />
    </div>
  );
}
