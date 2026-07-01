import type React from "react";
import { Metric as UIMetric } from "../../../components/ui/Metric";
import { Surface } from "../../../components/ui/Surface";

export function AdminPanel(props: { children: React.ReactNode; className?: string }) {
  return (
    <Surface variant="operational" className={props.className}>
      {props.children}
    </Surface>
  );
}

export function AdminMetric(props: { label: string; value: string }) {
  return <UIMetric label={props.label} value={props.value} />;
}

export function AdminDetailRow(props: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-3 border-b border-slate-900 pb-2 last:border-0 last:pb-0">
      <span className="shrink-0 text-slate-500">{props.label}</span>
      <span className="break-all text-right font-semibold text-slate-200">{props.value}</span>
    </div>
  );
}
