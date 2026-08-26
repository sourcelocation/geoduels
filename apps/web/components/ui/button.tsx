import * as React from "react";
import Link, { type LinkProps } from "next/link";

import { cn } from "../../lib/cn";
import { Spinner } from "./Spinner";

export type ButtonVariant = "primary" | "blue" | "secondary" | "ghost" | "danger" | "icon";
export type ButtonSize = "sm" | "md" | "lg" | "icon" | "icon-sm" | "icon-md";

const variantClass: Record<ButtonVariant, string> = {
  primary:
    "border-transparent bg-action-primary font-display text-body-sm font-strong text-content-on-action shadow-elev-1 hover:bg-action-primary-hover active:brightness-95",
  blue:
    "border-transparent bg-brand-blue font-display text-body-sm font-strong text-content-on-action shadow-elev-1 hover:bg-brand-blue-hover active:brightness-95",
  secondary:
    "border-border-default bg-surface-fill text-body-sm font-semibold text-content-primary hover:border-border-strong hover:bg-surface-grouped",
  ghost:
    "border-transparent bg-transparent text-body-sm font-semibold text-content-secondary hover:bg-surface-fill hover:text-content-primary",
  danger:
    "border-status-danger/30 bg-status-danger/10 font-display text-body-sm font-strong text-content-on-danger hover:bg-status-danger/20",
  icon:
    "border-border-default bg-surface-fill text-content-secondary hover:border-border-strong hover:bg-surface-grouped hover:text-content-primary",
};

const sizeClass: Record<ButtonSize, string> = {
  sm: "min-h-9 rounded-md px-3",
  md: "min-h-10 rounded-md px-3",
  lg: "min-h-12 rounded-lg px-5",
  icon: "h-10 min-h-10 w-10 rounded-full p-0",
  "icon-sm": "h-7 min-h-7 w-7 rounded-full p-0",
  "icon-md": "h-8 min-h-8 w-8 rounded-full p-0",
};

export type ButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
  size?: ButtonSize;
  icon?: React.ReactNode;
  loading?: boolean;
  loadingLabel?: string;
};

export function buttonClassName(variant: ButtonVariant = "secondary", size: ButtonSize = variant === "icon" ? "icon" : "md", className?: string) {
  return cn(
    "inline-flex items-center justify-center gap-2 border transition duration-fast focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-border-focus/60 active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-50 disabled:active:scale-100",
    variantClass[variant],
    sizeClass[size],
    className,
  );
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(function Button({
  className,
  variant = "secondary",
  size = variant === "icon" ? "icon" : "md",
  icon,
  loading = false,
  loadingLabel = "Loading",
    children,
    disabled,
    type = "button",
    ...props
}, ref) {
  return (
    <button
      ref={ref}
      type={type}
      className={buttonClassName(variant, size, className)}
      {...props}
      aria-busy={loading || undefined}
      disabled={disabled || loading}
    >
      {loading ? <Spinner label={loadingLabel} size="sm" color="current" /> : icon}
      {children}
    </button>
  );
});

export type IconButtonProps = Omit<ButtonProps, "variant" | "size"> & {
  "aria-label": string;
  size?: Extract<ButtonSize, "sm" | "md" | "lg" | "icon" | "icon-sm" | "icon-md">;
};

/** Use for a button whose visible content is only an icon. */
export const IconButton = React.forwardRef<HTMLButtonElement, IconButtonProps>(function IconButton({ size = "icon", ...props }, ref) {
  return <Button ref={ref} {...props} variant="icon" size={size === "icon" ? "icon" : size} />;
});

export type ButtonLinkProps = LinkProps &
  Omit<React.AnchorHTMLAttributes<HTMLAnchorElement>, keyof LinkProps> & {
    variant?: ButtonVariant;
    size?: ButtonSize;
    icon?: React.ReactNode;
  };

/** A navigation link with the same visual contract as Button. */
export function ButtonLink({
  variant = "secondary",
  size = variant === "icon" ? "icon" : "md",
  icon,
  className,
  children,
  ...props
}: ButtonLinkProps) {
  return (
    <Link className={buttonClassName(variant, size, className)} {...props}>
      {icon}
      {children}
    </Link>
  );
}
