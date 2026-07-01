import type { ElementType, ReactNode } from "react";
import { cn } from "../../../lib/cn";

export type AppContentRailSize = "compact" | "standard" | "wide";

const railWidth = {
  compact: "max-w-[520px]",
  standard: "max-w-[1120px]",
  wide: "max-w-[1220px]",
};

export function AppContentRail({
  as: Component = "div",
  children,
  className,
  size = "standard",
}: {
  as?: ElementType;
  children: ReactNode;
  className?: string;
  size?: AppContentRailSize;
}) {
  return (
    <Component
      className={cn(
        "mx-auto w-full px-4 sm:px-6 lg:px-8",
        railWidth[size],
        className,
      )}
    >
      {children}
    </Component>
  );
}
