import Link from "next/link";
import * as React from "react";

import { cn } from "../../lib/cn";
import { BodyText, Eyebrow, SectionTitle } from "./typography";

type PageShellVariant = "player" | "operational";

type PageShellProps = {
  variant?: PageShellVariant;
  title?: string;
  eyebrow?: string;
  description?: React.ReactNode;
  backHref?: string;
  backLabel?: string;
  actions?: React.ReactNode;
  maxWidthClassName?: string;
  children: React.ReactNode;
};

export function PageShell({
  variant = "operational",
  title,
  eyebrow,
  description,
  backHref,
  backLabel = "Back to GeoDuels",
  actions,
  maxWidthClassName = "max-w-4xl",
  children,
}: PageShellProps) {
  const operational = variant === "operational";
  return (
    <div
      className={cn(
        "min-h-screen font-sans",
        operational
          ? "bg-[#0d1216] text-[#f4f9ff]"
          : "relative overflow-hidden bg-[#071016] text-[#f4f9ff]",
      )}
    >
      <header className="border-b border-white/10">
        <div className={cn("mx-auto flex w-full items-center justify-between px-6 py-5", maxWidthClassName)}>
          {backHref ? (
            <Link href={backHref} className="text-sm font-bold text-inkMuted transition hover:text-white">
              {backLabel}
            </Link>
          ) : (
            <span />
          )}
          {actions || (
            <img src="/logo.v2.png" alt="GeoDuels" width={120} height={32} className="h-auto w-[110px]" />
          )}
        </div>
      </header>

      <main className={cn("mx-auto w-full px-6 py-10 sm:py-14", maxWidthClassName)}>
        {title || eyebrow || description ? (
          <div className="border-b border-white/10 pb-8">
            {eyebrow ? <Eyebrow>{eyebrow}</Eyebrow> : null}
            {title ? <SectionTitle className={eyebrow ? "mt-2" : ""}>{title}</SectionTitle> : null}
            {description ? <BodyText className="mt-5 max-w-3xl">{description}</BodyText> : null}
          </div>
        ) : null}
        {children}
      </main>
    </div>
  );
}
