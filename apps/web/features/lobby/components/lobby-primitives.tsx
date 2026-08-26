import type React from "react";
import { ContentUnavailable, DangerZone, SectionCard } from "../../../components/ui/compositions";
import { Input, type InputProps } from "../../../components/ui/input";
import { Select, type SelectProps } from "../../../components/ui/select";
import { Textarea, type TextareaProps } from "../../../components/ui/textarea";
import { cn } from "../../../lib/cn";
import { CenteredSpinner } from "../../../components/ui/Spinner";
import { Tabs } from "../../../components/ui/Tabs";
import { Notice } from "../../../components/ui/patterns";

const densityClass = {
  none: "p-0",
  sm: "p-3",
  md: "p-4",
  lg: "p-5",
};

export function LobbyInput(props: InputProps) {
  return <Input variant="game" {...props} />;
}

export function LobbyTextarea(props: TextareaProps) {
  return <Textarea variant="game" {...props} />;
}

export function LobbySelect(props: SelectProps) {
  return <Select variant="game" {...props} />;
}

export function LobbyPlaceholder(props: { children: React.ReactNode; className?: string; density?: keyof typeof densityClass }) {
  return (
    <ContentUnavailable className={cn(densityClass[props.density ?? "md"], props.className)}>
      {props.children}
    </ContentUnavailable>
  );
}

export function LobbySection({
  children,
  className,
  density = "md",
}: {
  children: React.ReactNode;
  className?: string;
  density?: keyof typeof densityClass;
}) {
  return (
    <SectionCard className={cn("rounded-lg", densityClass[density], className)}>
      {children}
    </SectionCard>
  );
}

export function LobbyDangerNotice(props: { children: React.ReactNode; className?: string }) {
  return (
    <DangerZone className={cn("rounded-lg p-4 text-body-sm font-semibold", props.className)}>
      {props.children}
    </DangerZone>
  );
}

export function LobbyFieldLabel(props: { children: React.ReactNode; htmlFor?: string; className?: string }) {
  const className = cn("text-label font-strong text-content-secondary", props.className);
  return props.htmlFor ? (
    <label htmlFor={props.htmlFor} className={className}>{props.children}</label>
  ) : (
    <span className={className}>{props.children}</span>
  );
}

export function LobbySegmentedControl<T extends string>({
  items,
  value,
  onChange,
  className,
}: {
  items: Array<{ value: T; label: string }>;
  value: T;
  onChange: (value: T) => void;
  className?: string;
}) {
  return (
    <Tabs appearance="segmented" value={value} onChange={onChange} items={items.map((item) => ({ id: item.value, label: item.label }))} className={className} />
  );
}

export function LobbyLoadingState(props: { children: string; className?: string }) {
  return <CenteredSpinner label={props.children} className={props.className} />;
}

export function LobbyNotice({
  title,
  children,
  trailing,
  tone = "warning",
  className,
}: {
  title: string;
  children: React.ReactNode;
  trailing?: React.ReactNode;
  tone?: "warning" | "danger" | "success" | "muted";
  className?: string;
}) {
  return (
    <Notice title={title} tone={tone === "muted" ? "muted" : tone} action={trailing} className={className}>{children}</Notice>
  );
}

export function LobbySectionHeader({
  eyebrow,
  title,
  description,
  className,
}: {
  eyebrow?: React.ReactNode;
  title: React.ReactNode;
  description?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={className}>
      {eyebrow ? (
        <span className="text-label font-strong text-status-success">
          {eyebrow}
        </span>
      ) : null}
      <h2 className="mt-1 text-display-md font-strong tracking-display-tight text-content-primary">{title}</h2>
      {description ? <p className="mt-2 text-body-sm leading-body text-content-secondary">{description}</p> : null}
    </div>
  );
}
