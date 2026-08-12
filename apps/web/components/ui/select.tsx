import * as React from "react";

import { cn } from "../../lib/cn";

type FieldVariant = "operational" | "game";

const variantClass: Record<FieldVariant, string> = {
  operational: "border-slate-700 bg-slate-950 text-slate-100 focus:border-emerald-400",
  game: "border-white/10 bg-[#101a20]/90 text-white focus:border-accentPrimary/60",
};

export type SelectProps = React.SelectHTMLAttributes<HTMLSelectElement> & {
  variant?: FieldVariant;
};

export function Select({ className, variant = "operational", ...props }: SelectProps) {
  return (
    <select
      className={cn(
        "min-h-10 rounded-md border px-3 text-sm outline-none transition disabled:cursor-not-allowed disabled:opacity-50 [&>option]:bg-black [&>option]:text-white",
        variantClass[variant],
        className,
      )}
      {...props}
    />
  );
}
