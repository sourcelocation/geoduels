import * as React from "react";

import { cn } from "../../lib/cn";

type ButtonVariant = "primary" | "secondary" | "ghost" | "danger" | "icon";
type ButtonSize = "sm" | "md" | "lg" | "icon";

const variantClass: Record<ButtonVariant, string> = {
  primary:
    "border-transparent bg-accentPrimary text-white shadow-[0_10px_24px_rgba(42,209,143,0.28)] hover:bg-accentPrimaryDeep",
  secondary:
    "border-white/10 bg-white/[0.08] text-white hover:bg-white/[0.12]",
  ghost:
    "border-transparent bg-transparent text-slate-300 hover:bg-white/[0.08] hover:text-white",
  danger:
    "border-red-400/30 bg-red-500/15 text-red-100 hover:bg-red-500/25",
  icon:
    "border-white/10 bg-white/[0.06] text-white/75 hover:bg-white/[0.12] hover:text-white",
};

const sizeClass: Record<ButtonSize, string> = {
  sm: "min-h-9 rounded-md px-3 text-xs",
  md: "min-h-10 rounded-md px-3 text-sm",
  lg: "min-h-12 rounded-[14px] px-5 text-sm",
  icon: "h-10 min-h-10 w-10 rounded-full p-0",
};

export function circularIconButtonClassName(className?: string) {
  return cn(
    "inline-flex h-10 min-h-10 w-10 items-center justify-center rounded-full border border-white/10 bg-white/[0.06] p-0 text-white/75 transition hover:bg-white/[0.12] hover:text-white disabled:cursor-not-allowed disabled:opacity-50",
    className,
  );
}

export type ButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
  size?: ButtonSize;
  icon?: React.ReactNode;
};

export function Button({
  className,
  variant = "secondary",
  size = variant === "icon" ? "icon" : "md",
  icon,
  children,
  ...props
}: ButtonProps) {
  return (
    <button
      className={cn(
        "inline-flex items-center justify-center gap-2 border font-bold transition disabled:cursor-not-allowed disabled:opacity-50",
        variantClass[variant],
        sizeClass[size],
        className,
      )}
      {...props}
    >
      {icon}
      {children}
    </button>
  );
}
