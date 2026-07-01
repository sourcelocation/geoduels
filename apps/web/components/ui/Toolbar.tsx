import * as React from "react";

import { cn } from "../../lib/cn";

export function Toolbar({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn("flex flex-wrap items-center gap-2 rounded-lg border border-white/10 bg-black/20 p-2", className)}
      {...props}
    />
  );
}
