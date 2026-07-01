import type React from "react";

import { Surface } from "./Surface";
import { MutedText } from "./typography";

type EmptyStateProps = {
  title?: string;
  message: string;
  action?: React.ReactNode;
};

export function EmptyState({ title, message, action }: EmptyStateProps) {
  return (
    <Surface variant="subtle" className="p-6 text-center">
      {title ? <h3 className="font-black text-white">{title}</h3> : null}
      <MutedText className={title ? "mt-2" : ""}>{message}</MutedText>
      {action ? <div className="mt-4">{action}</div> : null}
    </Surface>
  );
}
