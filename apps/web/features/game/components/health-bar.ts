import type { CSSProperties } from "react";

export type HealthTone = "healthy" | "warning" | "danger";

export function getHealthTone(percentage: number): HealthTone {
  return percentage > 50 ? "healthy" : percentage > 25 ? "warning" : "danger";
}

export function getHealthFillStyle(
  percentage: number,
  gradientAngle = "90deg",
): CSSProperties {
  const tone = getHealthTone(percentage);
  const token = tone === "healthy" ? "--gd-status-success" : tone === "warning" ? "--gd-status-warning" : "--gd-status-danger";

  return {
    backgroundImage: `linear-gradient(${tone === "healthy" ? gradientAngle : "180deg"}, rgb(var(${token}) / 1) 0%, rgb(var(${token}) / 0.72) 100%)`,
  };
}
