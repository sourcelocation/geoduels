import type { LucideIcon } from "lucide-react";
import { cn } from "../../../lib/cn";
import { SectionCard } from "../../../components/ui/compositions";

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
    <SectionCard
      className={cn(
        "grid min-w-0 grid-cols-[42px_minmax(0,1fr)] overflow-hidden rounded-xl sm:grid-cols-[48px_minmax(0,1fr)]",
        className,
      )}
    >
      <div className="flex items-center justify-center bg-surface-fill text-status-success">
        <Icon size={19} aria-hidden="true" />
      </div>
      <div className="min-w-0 px-2.5 py-3 sm:px-3">
        <div className="truncate text-heading-sm font-strong leading-collapsed text-content-primary">
          {value}
        </div>
        <div className="mt-1 text-label font-strong text-content-secondary">
          {label}
        </div>
      </div>
    </SectionCard>
  );
}
