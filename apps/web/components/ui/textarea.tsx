import * as React from "react";

import { cn } from "../../lib/cn";

type FieldVariant = "operational" | "game";

const variantClass: Record<FieldVariant, string> = {
  operational:
    "border-slate-700 bg-slate-950 text-slate-100 placeholder:text-slate-500 focus:border-emerald-400",
  game:
    "border-white/10 bg-[#101a20]/90 text-white placeholder:text-white/35 focus:border-accentPrimary/60",
};

export type TextareaProps = React.TextareaHTMLAttributes<HTMLTextAreaElement> & {
  variant?: FieldVariant;
};

export function Textarea({ className, variant = "operational", ...props }: TextareaProps) {
  return (
    <textarea
      className={cn(
        "rounded-md border px-3 py-2 text-sm outline-none transition disabled:cursor-not-allowed disabled:opacity-50",
        variantClass[variant],
        className,
      )}
      {...props}
    />
  );
}
