import * as React from "react";
import { ChevronDown } from "lucide-react";
import Link, { type LinkProps } from "next/link";
import { cn } from "../../lib/cn";
import { Surface } from "./Surface";

type BoxProps = React.HTMLAttributes<HTMLElement> & { children: React.ReactNode; as?: "section" | "article" | "div" };

/** Primary content container. Feature code chooses the job, never a material. */
export function AppPanel({ as = "section", className, children, ...props }: BoxProps) {
  return <Surface as={as} material="translucent" level={0} className={cn("rounded-xl", className)} {...props}>{children}</Surface>;
}

/** Primary application navigation chrome. */
export function AppNavigationSurface({ as = "nav", className, children, ...props }: Omit<BoxProps, "as"> & { as?: "nav" | "div" }) {
  return <Surface as={as} material="translucent" level={1} className={cn("rounded-xl", className)} {...props}>{children}</Surface>;
}

export const AppCardButton = React.forwardRef<HTMLButtonElement, React.ButtonHTMLAttributes<HTMLButtonElement>>(function AppCardButton(
  { className, children, type = "button", ...props },
  ref,
) {
  return <button ref={ref} type={type} className={cn("translucent-surface translucent-surface-interactive rounded-xl text-left active:scale-[0.995]", className)} {...props}>{children}</button>;
});

const appChromeControlClassName =
  "pointer-events-auto inline-flex items-center justify-center border-border-default text-content-secondary transition duration-fast hover:border-border-strong hover:text-content-primary focus-visible:outline-none active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-50 disabled:active:scale-100";

/** Standalone circular action displayed directly over the application backdrop. */
export const AppChromeIconButton = React.forwardRef<HTMLButtonElement, React.ButtonHTMLAttributes<HTMLButtonElement>>(function AppChromeIconButton(
  { className, children, type = "button", ...props },
  ref,
) {
  return <button ref={ref} type={type} className={cn("translucent-surface shadow-elev-1", appChromeControlClassName, "h-10 min-h-10 w-10 !rounded-full overflow-visible p-0", className)} {...props}>{children}</button>;
});

/** Standalone circular navigation action displayed over the application backdrop. */
export const AppChromeIconLink = React.forwardRef<
  HTMLAnchorElement,
  LinkProps & Omit<React.AnchorHTMLAttributes<HTMLAnchorElement>, keyof LinkProps>
>(function AppChromeIconLink({ className, children, ...props }, ref) {
  return (
    <Link
      ref={ref}
      className={cn("translucent-surface shadow-elev-1", appChromeControlClassName, "h-10 min-h-10 w-10 !rounded-full overflow-visible p-0", className)}
      {...props}
    >
      {children}
    </Link>
  );
});

/** Standalone text action displayed directly over the application backdrop. */
export function AppChromeButton({ className, children, type = "button", ...props }: React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return <button type={type} className={cn("translucent-surface shadow-elev-1", appChromeControlClassName, "min-h-10 rounded-full px-4 text-label font-strong", className)} {...props}>{children}</button>;
}

/** A visually quiet section nested in a panel or modal. */
export function SectionCard({ as = "section", className, children, ...props }: BoxProps) {
  return <Surface as={as} material="grouped" level={0} className={cn("rounded-lg", className)} {...props}>{children}</Surface>;
}

/** Stable, non-decorative container for admin and document content. */
export function DocumentPanel({ as = "section", className, children, ...props }: BoxProps) {
  return <Surface as={as} material="operational" level={1} className={cn("rounded-xl", className)} {...props}>{children}</Surface>;
}

export function SettingsGroup({ className, children, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("overflow-hidden rounded-lg border border-border-default bg-surface-grouped divide-y divide-border-default", className)} {...props}>{children}</div>;
}

/** Quiet explanatory or unavailable content; intentionally lighter than an inset well. */
export function ContentUnavailable({ className, children, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("rounded-lg border border-dashed border-border-default bg-surface-grouped p-6 text-center text-body-sm font-semibold text-content-secondary", className)} {...props}>{children}</div>;
}

export function DangerZone({ className, children, ...props }: React.HTMLAttributes<HTMLElement>) {
  return <Surface as="div" material="inset" tone="danger" level={0} className={cn("rounded-lg p-4", className)} {...props}>{children}</Surface>;
}

export function DangerZoneDisclosure({
  title,
  description,
  children,
  className,
}: {
  title: React.ReactNode;
  description: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}) {
  const [expanded, setExpanded] = React.useState(false);
  const contentId = React.useId();

  return (
    <DangerZone
      className={cn(
        "p-0 transition-colors",
        !expanded && "border-border-default bg-surface-grouped text-content-primary",
        className,
      )}
    >
      <button
        type="button"
        className={cn(
          "flex w-full items-center gap-3 rounded-lg p-4 text-left transition focus-visible:outline-none",
          expanded ? "hover:bg-status-danger/10" : "hover:bg-surface-fill",
        )}
        aria-expanded={expanded}
        aria-controls={contentId}
        onClick={() => setExpanded((value) => !value)}
      >
        <span className="min-w-0 flex-1">
          <span className="block text-body-sm font-strong text-content-primary">{title}</span>
          <span className="mt-1 block text-label leading-label text-content-secondary">{description}</span>
        </span>
        <ChevronDown className={cn("h-4 w-4 shrink-0 text-content-secondary transition-transform", expanded && "rotate-180")} aria-hidden="true" />
      </button>
      {expanded ? <div id={contentId} className="border-t border-status-danger/25 p-4">{children}</div> : null}
    </DangerZone>
  );
}
