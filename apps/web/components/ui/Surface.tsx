import * as React from "react";

import { cn } from "../../lib/cn";

export type SurfaceMaterial = "translucent" | "solid" | "grouped" | "inset" | "operational";
export type SurfaceTone = "default" | "muted" | "success" | "warning" | "danger" | "info";
export type SurfaceLevel = 0 | 1 | 2 | 3 | 4;

const materialClass: Record<SurfaceMaterial, string> = {
  // `translucent-surface` remains as the owned material implementation for
  // the game backdrop; semantic classes provide the contract to consumers.
  translucent: "translucent-surface text-content-primary",
  solid: "border border-border-default bg-surface-panel text-content-primary",
  grouped: "border border-border-default bg-surface-grouped text-content-primary",
  inset: "border border-border-default bg-surface-inset text-content-primary",
  operational: "border border-border-default bg-surface-panel text-content-primary",
};

const toneClass: Record<SurfaceTone, string> = {
  default: "",
  muted: "text-content-secondary",
  success: "border-status-success/30 bg-status-success/10 text-content-primary",
  warning: "border-status-warning/35 bg-status-warning/10 text-content-primary",
  danger: "border-status-danger/35 bg-status-danger/10 text-content-primary",
  info: "border-status-info/35 bg-status-info/10 text-content-primary",
};

const levelClass: Record<SurfaceLevel, string> = {
  0: "",
  1: "shadow-elev-1",
  2: "shadow-elev-2",
  3: "shadow-elev-3",
  4: "shadow-elev-4",
};

export function surfaceClassName({
  material = "operational",
  tone = "default",
  level = 1,
  interactive = false,
  className,
}: {
  material?: SurfaceMaterial;
  tone?: SurfaceTone;
  level?: SurfaceLevel;
  interactive?: boolean;
  className?: string;
}) {
  return cn(
    "rounded-xl",
    materialClass[material],
    toneClass[tone],
    levelClass[level],
    interactive && "transition hover:border-border-strong hover:bg-surface-fill focus-visible:border-border-focus focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-border-focus/40",
    className,
  );
}

type SurfaceProps<T extends React.ElementType = "section"> = {
  as?: T;
  material?: SurfaceMaterial;
  tone?: SurfaceTone;
  level?: SurfaceLevel;
  interactive?: boolean;
  className?: string;
  children: React.ReactNode;
} & Omit<React.ComponentPropsWithoutRef<T>, "as" | "className" | "children">;

export function Surface<T extends React.ElementType = "section">({
  as,
  material,
  tone,
  level,
  interactive = false,
  className,
  children,
  ...props
}: SurfaceProps<T>) {
  const Component = as || "section";
  const resolvedMaterial = material ?? "operational";
  const resolvedTone = tone ?? "default";
  const resolvedLevel = level ?? 1;
  return (
    <Component
      className={surfaceClassName({ material: resolvedMaterial, tone: resolvedTone, level: resolvedLevel, interactive, className })}
      {...props}
    >
      {children}
    </Component>
  );
}
