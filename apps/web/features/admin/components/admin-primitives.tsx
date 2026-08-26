import type React from "react";
import { Metric as UIMetric } from "../../../components/ui/Metric";
import { DocumentPanel } from "../../../components/ui/compositions";

export function AdminPanel(props: { children: React.ReactNode; className?: string }) {
  return (
    <DocumentPanel className={props.className}>
      {props.children}
    </DocumentPanel>
  );
}

export function AdminMetric(props: { label: string; value: string }) {
  return <UIMetric label={props.label} value={props.value} />;
}

export function AdminDetailRow(props: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-3 border-b border-border-default pb-2 last:border-0 last:pb-0">
      <span className="shrink-0 text-body-sm text-content-secondary">{props.label}</span>
      <span className="break-all text-right text-body-sm font-semibold text-content-primary">{props.value}</span>
    </div>
  );
}
