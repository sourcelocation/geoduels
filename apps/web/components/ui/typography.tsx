import * as React from "react";

import { cn } from "../../lib/cn";

export function Eyebrow({ className, ...props }: React.HTMLAttributes<HTMLParagraphElement>) {
  return (
    <p
      className={cn(
        "text-xs font-bold uppercase tracking-[0.16em] text-accentPrimary",
        className,
      )}
      {...props}
    />
  );
}

export function SectionTitle({ className, ...props }: React.HTMLAttributes<HTMLHeadingElement>) {
  return (
    <h1
      className={cn("text-3xl font-black text-white sm:text-4xl", className)}
      {...props}
    />
  );
}

export function BodyText({ className, ...props }: React.HTMLAttributes<HTMLParagraphElement>) {
  return (
    <p
      className={cn("text-base leading-7 text-[#c5d4e2]", className)}
      {...props}
    />
  );
}

export function MutedText({ className, ...props }: React.HTMLAttributes<HTMLParagraphElement>) {
  return <p className={cn("text-sm text-inkMuted", className)} {...props} />;
}

export function HelperText({ className, ...props }: React.HTMLAttributes<HTMLParagraphElement>) {
  return <p className={cn("text-xs font-semibold text-slate-400", className)} {...props} />;
}
