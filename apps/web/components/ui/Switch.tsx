import * as React from "react";
import { cn } from "../../lib/cn";

export type SwitchProps = Omit<React.ButtonHTMLAttributes<HTMLButtonElement>, "onChange"> & {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
};

export function Switch({ checked, onCheckedChange, className, disabled, ...props }: SwitchProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onCheckedChange(!checked)}
      className={cn(
        "relative inline-flex h-6 w-11 shrink-0 rounded-full border transition focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50",
        checked ? "border-status-success bg-status-success" : "border-border-default bg-surface-inset",
        className,
      )}
      {...props}
    >
      <span aria-hidden="true" className={cn("pointer-events-none inline-block h-5 w-5 rounded-full bg-content-on-action shadow-elev-1 transition", checked ? "translate-x-5" : "translate-x-0")} />
    </button>
  );
}

export function Checkbox({ className, ...props }: React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      type="checkbox"
      className={cn("h-4 w-4 rounded border-border-default bg-surface-inset text-status-success focus:ring-status-success disabled:cursor-not-allowed disabled:opacity-50", className)}
      {...props}
    />
  );
}
