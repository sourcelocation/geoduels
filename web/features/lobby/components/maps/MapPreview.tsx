import type { ReactNode } from "react";
import Link from "next/link";
import { Button } from "../../../../components/ui/button";
import { cn } from "../../../../lib/cn";

/** Shared edge-to-edge map artwork for navigation, selection, and settings. */
export function MapPreview({ name, imageURL, href, onClick, disabled, selected, actionLabel, children, className }: {
  name: string;
  imageURL: string;
  href?: string;
  onClick?: () => void;
  disabled?: boolean;
  selected?: boolean;
  actionLabel?: string;
  children?: ReactNode;
  className?: string;
}) {
  const classes = cn(
    "group relative block h-44 w-full overflow-hidden rounded-xl border-0 bg-surface-grouped p-0 text-left text-content-primary",
    "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-border-focus",
    className,
  );
  const content = <>
    <img src={imageURL} alt="" className="absolute inset-0 h-full w-full object-cover opacity-90 transition duration-emphasis group-hover:scale-105" />
    <div className="absolute inset-0 bg-gradient-to-t from-scrim via-transparent to-transparent" />
    {children || <div className="absolute inset-x-0 bottom-0 p-4">
      <p className="truncate text-heading-sm font-strong">{name}</p>
      {actionLabel ? <p className="mt-1 text-body-sm font-semibold">{actionLabel}</p> : null}
    </div>}
  </>;
  if (href) return <Link href={href} prefetch={false} className={classes}>{content}</Link>;
  if (onClick) return <Button variant="ghost" onClick={onClick} disabled={disabled} aria-label={actionLabel ? `${actionLabel}: ${name}` : name} aria-pressed={selected} className={classes}>{content}</Button>;
  return <div className={classes}>{content}</div>;
}
