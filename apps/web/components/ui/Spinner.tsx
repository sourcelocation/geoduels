import * as React from "react";
import { cn } from "../../lib/cn";

export function Spinner({
  label = "Loading",
  size = "md",
  color = "positive",
  className,
}: {
  label?: string;
  size?: "sm" | "md" | "lg";
  color?: "positive" | "current";
  className?: string;
}) {
  return (
    <span role="status" aria-label={label} className={cn("inline-flex items-center justify-center", className)}>
      <span
        aria-hidden="true"
        className={cn(
          "animate-spin rounded-full border-2 border-current border-r-transparent",
          color === "positive" ? "text-status-success" : "text-current",
          size === "sm" ? "h-4 w-4" : size === "lg" ? "h-8 w-8" : "h-5 w-5",
        )}
      />
      <span className="sr-only">{label}</span>
    </span>
  );
}

export function CenteredSpinner({
  label = "Loading",
  className,
}: {
  label?: string;
  className?: string;
}) {
  return (
    <div
      className={cn("flex min-h-40 w-full items-center justify-center", className)}
      aria-busy="true"
    >
      <Spinner size="lg" label={label} />
    </div>
  );
}

export function Skeleton({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div aria-hidden="true" className={cn("animate-pulse rounded-md bg-surface-fill", className)} {...props} />;
}
