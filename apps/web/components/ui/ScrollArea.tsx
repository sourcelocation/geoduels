import * as React from "react";
import { cn } from "../../lib/cn";

/** Native scrolling with a consistent, keyboard-accessible focus treatment. */
export function ScrollArea({ className, tabIndex = 0, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div tabIndex={tabIndex} className={cn("overflow-auto overscroll-contain focus-visible:outline-none", className)} {...props} />;
}
