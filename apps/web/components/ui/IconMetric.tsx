import type { LucideIcon } from "lucide-react";
import { cn } from "../../lib/cn";
import { Surface } from "./Surface";

export function IconMetric({
  icon: Icon,
  label,
  value,
  className,
}: {
  icon: LucideIcon;
  label: string;
  value: string | number;
  className?: string;
}) {
  return (
    <Surface
      variant="subtle"
      className={cn(
        "grid min-w-0 grid-cols-[42px_minmax(0,1fr)] overflow-hidden rounded-xl sm:grid-cols-[48px_minmax(0,1fr)]",
        className,
      )}
    >
      <div className="flex items-center justify-center bg-white/[0.06] text-[#77f0be]">
        <Icon size={19} aria-hidden="true" />
      </div>
      <div className="min-w-0 px-2.5 py-3 sm:px-3">
        <div className="truncate text-xl font-extrabold leading-none text-white">
          {value}
        </div>
        <div className="mt-1 text-[10px] font-bold leading-3 text-[#a9bfd4] sm:text-[11px]">
          {label}
        </div>
      </div>
    </Surface>
  );
}
