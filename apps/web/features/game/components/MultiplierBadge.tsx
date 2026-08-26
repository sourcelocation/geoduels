import { formatDamageMultiplierLabel } from "../lib/damage-multiplier";
import styles from "./overlays/GameHud.module.css";

type Props = {
  value: number;
  variant?: "hud" | "inline";
  testId?: string;
  showBaseline?: boolean;
  mirrored?: boolean;
};

export function MultiplierBadge({
  value,
  variant = "inline",
  testId,
  showBaseline = false,
  mirrored = false,
}: Props) {
  const label = showBaseline && value === 1 ? "1x" : formatDamageMultiplierLabel(value);
  if (!label) return null;

  if (variant === "inline") {
    return (
      <div
        data-testid={testId}
        className={`${styles.playerMultiplier} font-hud text-heading-sm font-strong leading-collapsed tracking-display-tight ${mirrored ? styles.playerMultiplierMirrored : ""}`}
      >
        <span>{label}</span>
      </div>
    );
  }

  return (
    <div
      data-testid={testId}
      className={`${styles.hudMultiplier} font-hud relative grid h-[54px] w-[58px] shrink-0 place-items-center text-hud-value tracking-display-tight text-content-primary md:h-[60px] md:w-[66px]`}
    >
      <div
        className={`${styles.hudMultiplierSurface} absolute inset-0 bg-hud-surface`}
      />
      <div className="absolute left-1/2 top-1/2 h-[20px] w-[40px] -translate-x-1/2 -translate-y-1/2 bg-status-success/30 blur-md" />
      <span className="relative z-content text-status-success">
        {label}
      </span>
    </div>
  );
}
