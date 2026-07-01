import type React from "react";
import Link, { type LinkProps } from "next/link";
import { Button, type ButtonProps } from "../../../components/ui/button";
import { Input, type InputProps } from "../../../components/ui/input";
import { Select, type SelectProps } from "../../../components/ui/select";
import { Surface, type SurfaceVariant } from "../../../components/ui/Surface";
import { Textarea, type TextareaProps } from "../../../components/ui/textarea";
import { cn } from "../../../lib/cn";

type LobbyPanelProps = {
  children: React.ReactNode;
  className?: string;
  interactive?: boolean;
  variant?: SurfaceVariant;
  radius?: "xl" | "2xl" | "3xl";
  density?: "none" | "sm" | "md" | "lg";
  style?: React.CSSProperties;
};

const radiusClass = {
  xl: "rounded-xl",
  "2xl": "rounded-2xl",
  "3xl": "rounded-3xl",
};

const densityClass = {
  none: "p-0",
  sm: "p-3",
  md: "p-4",
  lg: "p-5",
};

export function LobbyPanel({
  children,
  className,
  interactive = false,
  variant = "gameGlass",
  radius = "2xl",
  density,
  style,
}: LobbyPanelProps) {
  return (
    <Surface
      variant={variant}
      interactive={interactive}
      className={cn(radiusClass[radius], density ? densityClass[density] : undefined, className)}
      style={style}
    >
      {children}
    </Surface>
  );
}

export function LobbyActionButton({
  className,
  variant = "primary",
  size = "md",
  emphasis = "loud",
  ...props
}: ButtonProps & { emphasis?: "loud" | "plain" }) {
  return (
    <Button
      variant={variant}
      size={size}
      className={cn(
        emphasis === "loud" ? "font-extrabold uppercase tracking-[0.08em]" : undefined,
        className,
      )}
      {...props}
    />
  );
}

export function LobbyActionLink({
  className,
  variant = "primary",
  size = "md",
  emphasis = "loud",
  children,
  ...props
}: LinkProps &
  Omit<React.AnchorHTMLAttributes<HTMLAnchorElement>, keyof LinkProps> &
  Pick<ButtonProps, "variant" | "size"> & { emphasis?: "loud" | "plain" }) {
  const variantClass = {
    primary: "border-transparent bg-accentPrimary text-white shadow-[0_10px_24px_rgba(42,209,143,0.28)] hover:bg-accentPrimaryDeep",
    secondary: "border-white/10 bg-white/[0.08] text-white hover:bg-white/[0.12]",
    ghost: "border-transparent bg-transparent text-slate-300 hover:bg-white/[0.08] hover:text-white",
    danger: "border-red-400/30 bg-red-500/15 text-red-100 hover:bg-red-500/25",
    icon: "border-white/10 bg-white/[0.06] text-white/75 hover:bg-white/[0.12] hover:text-white",
  }[variant];
  const sizeClass = {
    sm: "min-h-9 rounded-md px-3 text-xs",
    md: "min-h-10 rounded-md px-3 text-sm",
    lg: "min-h-12 rounded-[14px] px-5 text-sm",
    icon: "h-10 min-h-10 w-10 rounded-full p-0",
  }[size];

  return (
    <Link
      className={cn(
        "inline-flex items-center justify-center gap-2 border font-bold transition",
        emphasis === "loud" ? "font-extrabold uppercase tracking-[0.08em]" : undefined,
        variantClass,
        sizeClass,
        className,
      )}
      {...props}
    >
      {children}
    </Link>
  );
}

export function LobbyCardButton({
  children,
  className,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { children: React.ReactNode }) {
  return (
    <button
      type="button"
      className={cn(
        "glass-panel glass-panel-interactive lobby-feature-card rounded-2xl text-left",
        className,
      )}
      {...props}
    >
      {children}
    </button>
  );
}

export function LobbyInput(props: InputProps) {
  return <Input variant="game" {...props} />;
}

export function LobbyTextarea(props: TextareaProps) {
  return <Textarea variant="game" {...props} />;
}

export function LobbySelect(props: SelectProps) {
  return <Select variant="game" {...props} />;
}

export function LobbyMutedBox(props: { children: React.ReactNode; className?: string; density?: keyof typeof densityClass }) {
  return (
    <Surface variant="subtle" className={cn("rounded-xl text-sm font-semibold text-inkMuted", densityClass[props.density ?? "md"], props.className)}>
      {props.children}
    </Surface>
  );
}

export function LobbyInset({
  children,
  className,
  density = "md",
  tone = "subtle",
}: {
  children: React.ReactNode;
  className?: string;
  density?: keyof typeof densityClass;
  tone?: SurfaceVariant;
}) {
  return (
    <Surface variant={tone} className={cn("rounded-xl", densityClass[density], className)}>
      {children}
    </Surface>
  );
}

export function LobbyDangerNotice(props: { children: React.ReactNode; className?: string }) {
  return (
    <Surface variant="danger" className={cn("rounded-xl p-4 text-sm font-semibold", props.className)}>
      {props.children}
    </Surface>
  );
}

export function LobbyPill({
  children,
  tone = "muted",
  className,
}: {
  children: React.ReactNode;
  tone?: "muted" | "success" | "danger" | "warning" | "blue";
  className?: string;
}) {
  const toneClass = {
    muted: "bg-white/[0.06] text-[#a9bfd4]",
    success: "bg-[#2ad18f]/15 text-[#8ff0c2]",
    danger: "bg-red-500/15 text-red-100",
    warning: "bg-[#f4c84c]/15 text-[#fff0ba]",
    blue: "bg-[#2563eb]/15 text-[#bfdbfe]",
  }[tone];
  return (
    <span className={cn("inline-flex items-center rounded-full px-2.5 py-1 text-[10px] font-black uppercase tracking-[0.12em]", toneClass, className)}>
      {children}
    </span>
  );
}

export function LobbyFieldLabel(props: { children: React.ReactNode; htmlFor?: string; className?: string }) {
  return (
    <label htmlFor={props.htmlFor} className={cn("text-[10px] font-black uppercase tracking-[0.14em] text-[#6b8b80]", props.className)}>
      {props.children}
    </label>
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
    <div className={cn("flex rounded-xl border border-white/10 bg-black/20 p-1", className)}>
      {items.map((item) => (
        <button
          key={item.value}
          type="button"
          onClick={() => onChange(item.value)}
          className={cn(
            "rounded-lg px-3 py-2 text-[11px] font-black uppercase tracking-[0.08em] transition",
            value === item.value ? "bg-white text-[#10201a]" : "text-[#a9bfd4] hover:bg-white/[0.08]",
          )}
        >
          {item.label}
        </button>
      ))}
    </div>
  );
}

export function LobbyIconButton({
  className,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      type="button"
      className={cn("inline-flex h-9 w-9 items-center justify-center rounded-full border border-white/10 bg-white/[0.06] text-white/75 transition hover:bg-white/[0.12] hover:text-white disabled:cursor-not-allowed disabled:opacity-50", className)}
      {...props}
    />
  );
}

export function LobbyLoadingState(props: { children: React.ReactNode; className?: string }) {
  return (
    <div className={cn("flex items-center gap-3 text-sm font-semibold text-[#a9bfd4]", props.className)}>
      {props.children}
    </div>
  );
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
  const toneClass = {
    warning:
      "border-[#f3cf68]/40 bg-[linear-gradient(135deg,rgba(242,197,67,0.22),rgba(115,75,0,0.28))] text-[#fff6d8] shadow-[0_12px_40px_rgba(91,63,7,0.24)]",
    danger: "border-red-300/20 bg-red-500/10 text-red-100 shadow-[0_14px_40px_rgba(0,0,0,0.22)]",
    success: "border-[#77f0be]/20 bg-[#22d385]/10 text-[#d8f7e9]",
    muted: "border-white/10 bg-black/20 text-[#d6e4ed]",
  }[tone];
  const titleClass = {
    warning: "text-[#ffe69a]",
    danger: "text-red-200",
    success: "text-[#77f0be]",
    muted: "text-[#a9bfd4]",
  }[tone];
  const trailingClass = {
    warning: "text-[#ffefb5]",
    danger: "text-red-100",
    success: "text-[#baf7dc]",
    muted: "text-[#d6e4ed]",
  }[tone];

  return (
    <div className={cn("rounded-3xl border px-5 py-4 backdrop-blur-sm", toneClass, className)}>
      <div className="flex flex-col gap-1.5 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <p className={cn("text-[11px] font-black uppercase tracking-[0.18em]", titleClass)}>
            {title}
          </p>
          <div className="mt-1 text-[15px] font-semibold text-white">{children}</div>
        </div>
        {trailing ? (
          <div className={cn("text-[15px] font-semibold", trailingClass)}>{trailing}</div>
        ) : null}
      </div>
    </div>
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
        <span className="text-[11px] font-black uppercase tracking-[0.16em] text-[#77f0be]">
          {eyebrow}
        </span>
      ) : null}
      <h2 className="mt-1 text-[30px] font-black text-white">{title}</h2>
      {description ? <p className="mt-2 text-sm text-[#a9bfd4]">{description}</p> : null}
    </div>
  );
}
