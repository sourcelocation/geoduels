import * as React from "react";

import { cn } from "../../lib/cn";

type Tone = "neutral" | "success" | "warning" | "danger" | "info";

const toneClass: Record<Tone, string> = {
  neutral: "bg-slate-800 text-slate-200",
  success: "bg-emerald-400/15 text-emerald-200",
  warning: "bg-amber-400/15 text-amber-200",
  danger: "bg-red-500/15 text-red-200",
  info: "bg-blue-400/15 text-blue-200",
};

export function StatusPill({
  tone = "neutral",
  className,
  ...props
}: React.HTMLAttributes<HTMLSpanElement> & { tone?: Tone }) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2.5 py-1 text-[10px] font-black uppercase tracking-[0.1em]",
        toneClass[tone],
        className,
      )}
      {...props}
    />
  );
}
