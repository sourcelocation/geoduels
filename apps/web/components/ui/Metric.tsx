import { cn } from "../../lib/cn";

type MetricProps = {
  label: string;
  value: string;
  className?: string;
};

export function Metric({ label, value, className }: MetricProps) {
  return (
    <div className={cn("rounded-md border border-slate-800 bg-slate-900/60 p-3", className)}>
      <p className="text-xs font-bold uppercase tracking-[0.12em] text-slate-500">{label}</p>
      <p className="mt-1 text-lg font-black text-white">{value}</p>
    </div>
  );
}
