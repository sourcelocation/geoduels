import * as React from "react";

import { cn } from "../../lib/cn";

export type FieldVariant = "operational" | "game";

const variantClass: Record<FieldVariant, string> = {
  operational:
    "border-border-default bg-surface-grouped text-content-primary placeholder:text-content-secondary/60 focus:border-border-focus focus:bg-surface-fill",
  game:
    "border-border-default bg-surface-grouped text-content-primary placeholder:text-content-secondary/60 focus:border-border-focus focus:bg-surface-fill",
};

export type InputProps = React.InputHTMLAttributes<HTMLInputElement> & {
  variant?: FieldVariant;
};

export const Input = React.forwardRef<HTMLInputElement, InputProps>(function Input(
  { className, variant = "operational", ...props },
  ref,
) {
  return (
    <input
      ref={ref}
      className={cn(
        "min-h-11 rounded-md border px-3.5 text-body-sm outline-none transition focus-visible:ring-2 focus-visible:ring-border-focus/40 disabled:cursor-not-allowed disabled:opacity-50",
        variantClass[variant],
        className,
      )}
      {...props}
    />
  );
});
