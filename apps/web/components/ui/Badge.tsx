import * as React from "react";

import { cn } from "../../lib/cn";

export type BadgeTone = "neutral" | "success" | "warning" | "danger" | "info";
export type BadgeSize = "sm" | "md";
export type BadgeEmphasis = "subtle" | "strong";

const toneClass: Record<BadgeTone, string> = {
  neutral: "bg-surface-fill text-content-secondary",
  success: "text-status-success",
  warning: "bg-status-warning/15 text-status-warning",
  danger: "bg-status-danger/15 text-status-danger",
  info: "bg-status-info/15 text-status-info",
};

export function Badge({
  tone = "neutral",
  size = "sm",
  emphasis = "subtle",
  className,
  ...props
}: React.HTMLAttributes<HTMLSpanElement> & { tone?: BadgeTone; size?: BadgeSize; emphasis?: BadgeEmphasis }) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full font-strong uppercase",
        size === "sm" ? "px-2.5 py-1 text-caption" : "px-3 py-1.5 text-label",
        toneClass[tone],
        tone === "success" && (emphasis === "strong" ? "bg-status-success/25" : "bg-status-success/15"),
        className,
      )}
      {...props}
    />
  );
}

export function CounterBadge({
  count,
  max = 99,
  label = "unread notifications",
  className,
}: {
  count: number;
  max?: number;
  label?: string;
  className?: string;
}) {
  if (count <= 0) return null;
  const visibleCount = count > max ? `${max}+` : String(count);
  return (
    <span
      aria-label={`${count} ${label}`}
      className={cn(
        "inline-flex min-w-5 items-center justify-center rounded-full bg-action-danger px-1.5 py-0.5 text-caption font-strong leading-collapsed text-content-on-danger",
        className,
      )}
    >
      {visibleCount}
    </span>
  );
}
