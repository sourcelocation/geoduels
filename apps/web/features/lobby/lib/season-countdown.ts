import { useEffect, useState } from "react";

export function useSeasonResetCountdown(nextResetAt?: string) {
  const [nowMs, setNowMs] = useState(0);

  useEffect(() => {
    setNowMs(Date.now());
    const timer = window.setInterval(() => setNowMs(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [nextResetAt]);

  if (!nextResetAt || nowMs === 0) return "Season reset in --";
  const resetMs = Date.parse(nextResetAt);
  if (!Number.isFinite(resetMs)) return "Season reset in --";
  return resetMs <= nowMs
    ? "Season reset due"
    : `${formatSeasonResetCountdown(resetMs - nowMs)}`;
}

export function formatSeasonResetCountdown(ms: number) {
  const totalSeconds = Math.max(0, Math.ceil(ms / 1000));
  const days = Math.floor(totalSeconds / 86400);
  const hours = Math.floor((totalSeconds % 86400) / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  if (days > 0) return `${days}d ${hours}h ${minutes}m ${seconds}s`;
  if (hours > 0) return `${hours}h ${minutes}m ${seconds}s`;
  if (minutes > 0) return `${minutes}m ${seconds}s`;
  return `${seconds}s`;
}
