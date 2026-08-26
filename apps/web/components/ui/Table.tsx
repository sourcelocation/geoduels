import * as React from "react";

import { cn } from "../../lib/cn";

export function TableShell({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("overflow-hidden rounded-md border border-border-default", className)} {...props} />;
}

export function Table({ className, ...props }: React.TableHTMLAttributes<HTMLTableElement>) {
  return <table className={cn("w-full text-left text-body-sm", className)} {...props} />;
}

export function TableHead({ className, ...props }: React.HTMLAttributes<HTMLTableSectionElement>) {
  return <thead className={cn("border-b border-border-default text-label uppercase tracking-eyebrow text-content-secondary", className)} {...props} />;
}
