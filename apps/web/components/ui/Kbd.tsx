import * as React from "react";
import { cn } from "../../lib/cn";

export function Kbd({ className, ...props }: React.HTMLAttributes<HTMLElement>) {
  return (
    <kbd
      className={cn("inline-flex min-h-6 items-center rounded-md border border-border-default bg-surface-inset px-1.5 font-mono text-caption font-strong text-content-primary shadow-elev-1", className)}
      {...props}
    />
  );
}
