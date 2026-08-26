import * as React from "react";

import { cn } from "../../lib/cn";

/** Visual roles are independent from document semantics. */
export type TextVariant =
  | "body" | "body-sm" | "label" | "caption"
  | "heading-sm" | "heading-md" | "heading-lg"
  | "display-md" | "display-lg" | "hud-label" | "hud-value" | "hud-countdown" | "hud-countdown-lg";

const variantClass: Record<TextVariant, string> = {
  body: "text-body font-regular leading-body text-content-primary",
  "body-sm": "text-body-sm font-regular leading-body text-content-primary",
  label: "text-label font-strong uppercase text-content-secondary",
  caption: "text-caption font-medium text-content-secondary",
  "heading-sm": "text-heading-sm font-strong leading-heading text-content-primary",
  "heading-md": "text-heading-md font-strong leading-heading text-content-primary",
  "heading-lg": "text-heading-lg font-strong leading-heading text-content-primary",
  "display-md": "font-display text-display-md font-strong leading-heading tracking-display-tight text-content-primary",
  "display-lg": "font-display text-display-lg font-strong leading-heading tracking-display-tight text-content-primary",
  "hud-label": "font-hud text-hud-label font-strong uppercase text-content-primary",
  "hud-value": "font-hud text-hud-value font-strong text-content-primary",
  "hud-countdown": "font-hud text-hud-countdown font-strong leading-collapsed text-content-primary",
  "hud-countdown-lg": "font-hud text-hud-countdown-lg font-strong leading-collapsed text-content-primary",
};

type PolymorphicProps<T extends React.ElementType, Props> = Props & { as?: T } &
  Omit<React.ComponentPropsWithoutRef<T>, keyof Props | "as">;

export type TextProps<T extends React.ElementType = "p"> = PolymorphicProps<T, {
  variant?: TextVariant;
  className?: string;
  children?: React.ReactNode;
}>;

export function Text<T extends React.ElementType = "p">({ as, variant = "body", className, ...props }: TextProps<T>) {
  const Component = (as ?? "p") as React.ElementType;
  return <Component className={cn(variantClass[variant], className)} {...props} />;
}

export type HeadingVariant = Extract<TextVariant, "heading-sm" | "heading-md" | "heading-lg" | "display-md" | "display-lg">;
export type HeadingProps<T extends React.ElementType = "h2"> = PolymorphicProps<T, {
  variant?: HeadingVariant;
  className?: string;
  children?: React.ReactNode;
}>;

export function Heading<T extends React.ElementType = "h2">({ as, variant = "heading-md", className, ...props }: HeadingProps<T>) {
  const Component = (as ?? "h2") as React.ElementType;
  return <Component className={cn(variantClass[variant], className)} {...props} />;
}

// Compatibility atoms; new code should prefer Text / Heading.
export function Eyebrow({ className, ...props }: React.HTMLAttributes<HTMLParagraphElement>) {
  return <Text as="p" variant="label" className={cn("text-status-success", className)} {...props} />;
}

export function SectionTitle({ className, ...props }: React.HTMLAttributes<HTMLHeadingElement>) {
  return <Heading as="h1" variant="display-md" className={cn("sm:text-display-lg", className)} {...props} />;
}

export function CardTitle({ className, ...props }: React.HTMLAttributes<HTMLHeadingElement>) {
  return <Heading as="h2" variant="heading-sm" className={className} {...props} />;
}

export function BodyText({ className, ...props }: React.HTMLAttributes<HTMLParagraphElement>) {
  return <Text as="p" variant="body" className={cn("leading-prose-lg text-content-secondary", className)} {...props} />;
}

export function MutedText({ className, ...props }: React.HTMLAttributes<HTMLParagraphElement>) {
  return <Text as="p" variant="body-sm" className={cn("text-content-secondary", className)} {...props} />;
}

export function HelperText({ className, ...props }: React.HTMLAttributes<HTMLParagraphElement>) {
  return <Text as="p" variant="caption" className={className} {...props} />;
}
