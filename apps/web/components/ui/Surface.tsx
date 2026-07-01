import * as React from "react";

import { cn } from "../../lib/cn";

export type SurfaceVariant =
  | "gameGlass"
  | "gameSolid"
  | "operational"
  | "danger"
  | "subtle";

const variantClass: Record<SurfaceVariant, string> = {
  gameGlass:
    "glass-panel text-[#f4f9ff]",
  gameSolid:
    "border border-white/10 bg-[#101a20]/92 text-[#f4f9ff] shadow-elev-2",
  operational:
    "border border-slate-800 bg-slate-950/80 text-slate-100 shadow-sm",
  danger:
    "border border-red-400/25 bg-red-950/35 text-red-100 shadow-sm",
  subtle:
    "border border-white/10 bg-black/20 text-[#f4f9ff]",
};

type SurfaceProps<T extends React.ElementType = "section"> = {
  as?: T;
  variant?: SurfaceVariant;
  interactive?: boolean;
  className?: string;
  children: React.ReactNode;
} & Omit<React.ComponentPropsWithoutRef<T>, "as" | "className" | "children">;

export function Surface<T extends React.ElementType = "section">({
  as,
  variant = "operational",
  interactive = false,
  className,
  children,
  ...props
}: SurfaceProps<T>) {
  const Component = as || "section";
  return (
    <Component
      className={cn(
        "rounded-lg",
        variantClass[variant],
        interactive && "transition hover:border-white/20 hover:bg-white/[0.08]",
        className,
      )}
      {...props}
    >
      {children}
    </Component>
  );
}
