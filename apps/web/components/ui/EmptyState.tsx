import type React from "react";

import { cn } from "../../lib/cn";
import { SectionCard } from "./compositions";
import { MutedText } from "./typography";

type EmptyStateProps = {
  title?: string;
  message: string;
  action?: React.ReactNode;
  className?: string;
};

export function EmptyState({ title, message, action, className }: EmptyStateProps) {
  return (
    <SectionCard className={cn("border-dashed p-6 text-center", className)}>
      {title ? <h3 className="text-body font-strong text-content-primary">{title}</h3> : null}
      <MutedText className={title ? "mt-2" : ""}>{message}</MutedText>
      {action ? <div className="mt-4">{action}</div> : null}
    </SectionCard>
  );
}
