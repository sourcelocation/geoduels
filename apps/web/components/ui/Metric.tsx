import { cn } from "../../lib/cn";

type MetricProps = {
  label: string;
  value: string;
  className?: string;
};

export function Metric({ label, value, className }: MetricProps) {
  return (
    <div className={cn("rounded-md border border-border-default bg-surface-panel p-3", className)}>
      <p className="text-label font-strong uppercase tracking-eyebrow text-content-secondary">{label}</p>
      <p className="mt-1 text-heading-sm font-strong text-content-primary">{value}</p>
    </div>
  );
}
