import * as React from "react";

import { cn } from "../../lib/cn";
import { Button, type ButtonProps } from "./button";
import { EmptyState } from "./EmptyState";
import { InfoButton } from "./InfoButton";
import { Separator } from "./Separator";
import { CenteredSpinner } from "./Spinner";
import { Surface, type SurfaceTone } from "./Surface";
import { BodyText, Eyebrow, HelperText, SectionTitle } from "./typography";

export function SectionHeader({
  eyebrow,
  title,
  description,
  actions,
  className,
}: {
  eyebrow?: React.ReactNode;
  title: React.ReactNode;
  description?: React.ReactNode;
  actions?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between", className)}>
      <div className="min-w-0">{eyebrow ? <Eyebrow>{eyebrow}</Eyebrow> : null}<h2 className="mt-1 text-heading-md font-strong text-content-primary sm:text-heading-lg">{title}</h2>{description ? <BodyText className="mt-2 max-w-3xl">{description}</BodyText> : null}</div>
      {actions ? <ActionGroup className="shrink-0">{actions}</ActionGroup> : null}
    </div>
  );
}

export function PageHeader({
  eyebrow,
  title,
  description,
  actions,
  className,
}: React.ComponentProps<typeof SectionHeader>) {
  return (
      <div className={cn("flex flex-col gap-3 border-b border-border-default pb-8 sm:flex-row sm:items-end sm:justify-between", className)}>
      <div className="min-w-0">{eyebrow ? <Eyebrow>{eyebrow}</Eyebrow> : null}<SectionTitle className={eyebrow ? "mt-1" : undefined}>{title}</SectionTitle>{description ? <BodyText className="mt-2 max-w-3xl">{description}</BodyText> : null}</div>
      {actions ? <ActionGroup className="shrink-0">{actions}</ActionGroup> : null}
    </div>
  );
}

export function ActionGroup({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("flex flex-wrap items-center gap-2", className)} {...props} />;
}

export function InsetList({ className, children, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("overflow-hidden rounded-lg border border-border-default bg-surface-inset divide-y divide-border-default", className)} {...props}>{children}</div>;
}

/** Structural row for a player, map, notification, or other compact entity. */
export function EntityRow({
  leading,
  title,
  description,
  meta,
  actions,
  className,
  children,
}: {
  leading?: React.ReactNode;
  title: React.ReactNode;
  description?: React.ReactNode;
  meta?: React.ReactNode;
  actions?: React.ReactNode;
  children?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex min-h-16 items-center gap-3 px-3 py-3 sm:px-4", className)}>
      {leading ? <div className="shrink-0">{leading}</div> : null}
      <div className="min-w-0 flex-1"><div className="truncate text-body-sm font-strong text-content-primary">{title}</div>{description ? <div className="mt-0.5 text-body-sm text-content-secondary">{description}</div> : null}{children}</div>
      {meta ? <div className="shrink-0 text-label font-semibold text-content-secondary">{meta}</div> : null}
      {actions ? <ActionGroup className="shrink-0">{actions}</ActionGroup> : null}
    </div>
  );
}

export function SettingRow({
  title,
  description,
  info,
  control,
  stackControlOnMobile = false,
  className,
}: {
  title: React.ReactNode;
  description?: React.ReactNode;
  info?: React.ReactNode;
  control: React.ReactNode;
  stackControlOnMobile?: boolean;
  className?: string;
}) {
  if (!stackControlOnMobile) {
    return <EntityRow title={<SettingTitle title={title} info={info} />} description={description} actions={control} className={cn("min-h-18", className)} />;
  }

  return (
    <div
      className={cn(
        "flex min-h-18 flex-col items-stretch gap-3 px-3 py-3 sm:flex-row sm:items-center sm:px-4",
        className,
      )}
    >
      <div className="min-w-0 flex-1">
        <SettingTitle title={title} info={info} />
        {description ? <div className="mt-0.5 text-body-sm text-content-secondary">{description}</div> : null}
      </div>
      <div className="flex shrink-0 justify-end">{control}</div>
    </div>
  );
}

function SettingTitle({ title, info }: { title: React.ReactNode; info?: React.ReactNode }) {
  return (
    <div className="flex items-center gap-1.5 text-body-sm font-strong text-content-primary">
      <span>{title}</span>
      {info ? <InfoButton content={info} label={`About ${typeof title === "string" ? title : "this setting"}`} /> : null}
    </div>
  );
}

export function Notice({
  title,
  children,
  tone = "info",
  action,
  className,
}: {
  title?: React.ReactNode;
  children: React.ReactNode;
  tone?: Exclude<SurfaceTone, "default">;
  action?: React.ReactNode;
  className?: string;
}) {
  return (
    <Surface material="inset" tone={tone} level={0} role={tone === "danger" ? "alert" : "status"} className={cn("rounded-lg p-4", className)}>
      <div className="flex flex-wrap items-start justify-between gap-3"><div>{title ? <p className="text-body-sm font-strong text-content-primary">{title}</p> : null}<div className={cn("text-body-sm", title && "mt-1")}>{children}</div></div>{action}</div>
    </Surface>
  );
}

export type AsyncStateProps = {
  status: "loading" | "error" | "empty";
  title?: string;
  message: string;
  onRetry?: () => void;
  retryLabel?: string;
  action?: React.ReactNode;
  className?: string;
};

/** Standard feedback for async lists and panels. Render data separately. */
export function AsyncState({ status, title, message, onRetry, retryLabel = "Try again", action, className }: AsyncStateProps) {
  if (status === "loading") {
    return <CenteredSpinner label={message} className={className} />;
  }
  if (status === "error") {
    return <Notice title={title ?? "Something went wrong"} tone="danger" className={className}>{message}{onRetry ? <Button variant="secondary" size="sm" onClick={onRetry} className="ml-3">{retryLabel}</Button> : action}</Notice>;
  }
  return <EmptyState title={title} message={message} action={action} className={className} />;
}

export function FormHint({ children, className }: { children: React.ReactNode; className?: string }) {
  return <HelperText className={className}>{children}</HelperText>;
}

export function DestructiveAction({ children, ...props }: Omit<ButtonProps, "variant">) {
  return <Button variant="danger" {...props}>{children}</Button>;
}

export { Separator };
