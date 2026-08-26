import { Info } from "lucide-react";
import { IconButton } from "./button";
import { Tooltip } from "./Tooltip";

export function InfoButton({
  content,
  label = "More information",
}: {
  content: React.ReactNode;
  label?: string;
}) {
  return (
    <Tooltip content={content} side="top" align="start">
      <IconButton
        type="button"
        aria-label={label}
        className="h-7 min-h-7 w-7 rounded-full p-0"
      >
        <Info className="h-3.5 w-3.5" aria-hidden="true" />
      </IconButton>
    </Tooltip>
  );
}
