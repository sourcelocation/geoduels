import * as React from "react";
import { cn } from "../../lib/cn";

export function Separator({
  orientation = "horizontal",
  decorative = true,
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement> & { orientation?: "horizontal" | "vertical"; decorative?: boolean }) {
  return (
    <div
      role={decorative ? undefined : "separator"}
      aria-orientation={decorative ? undefined : orientation}
      className={cn("shrink-0 bg-border-default", orientation === "horizontal" ? "h-px w-full" : "h-full w-px self-stretch", className)}
      {...props}
    />
  );
}
