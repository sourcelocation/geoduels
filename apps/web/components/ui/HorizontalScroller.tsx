import * as React from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";

import { cn } from "../../lib/cn";
import { ButtonLink, IconButton } from "./button";

export function HorizontalScroller({
  label,
  children,
  className,
  itemClassName,
  viewAllHref,
}: {
  label: string;
  children: React.ReactNode;
  className?: string;
  itemClassName?: string;
  viewAllHref?: string;
}) {
  const viewportRef = React.useRef<HTMLDivElement>(null);
  const [canScrollBack, setCanScrollBack] = React.useState(false);
  const [canScrollForward, setCanScrollForward] = React.useState(false);

  const updateControls = React.useCallback(() => {
    const viewport = viewportRef.current;
    if (!viewport) return;
    setCanScrollBack(viewport.scrollLeft > 1);
    setCanScrollForward(viewport.scrollLeft + viewport.clientWidth < viewport.scrollWidth - 1);
  }, []);

  React.useEffect(() => {
    updateControls();
    const viewport = viewportRef.current;
    if (!viewport) return;
    const observer = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(updateControls);
    observer?.observe(viewport);
    window.addEventListener("resize", updateControls);
    return () => {
      observer?.disconnect();
      window.removeEventListener("resize", updateControls);
    };
  }, [children, updateControls]);

  const move = (direction: -1 | 1) => {
    const viewport = viewportRef.current;
    if (!viewport) return;
    const reducedMotion = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
    viewport.scrollBy({
      left: direction * Math.max(viewport.clientWidth * 0.82, 240),
      behavior: reducedMotion ? "auto" : "smooth",
    });
  };

  return (
    <div className={cn("min-w-0", className)}>
      <div className="mb-3 flex items-center justify-between gap-4">
        <h2 className="text-heading-md font-strong leading-heading text-content-primary">{label}</h2>
        <div className="flex shrink-0 items-center gap-2" aria-label={`${label} navigation`}>
          {viewAllHref ? (
            <ButtonLink href={viewAllHref} variant="ghost" size="sm">
              View all
            </ButtonLink>
          ) : null}
          <IconButton aria-label={`Previous ${label}`} size="icon-md" disabled={!canScrollBack} onClick={() => move(-1)}>
            <ChevronLeft size={17} />
          </IconButton>
          <IconButton aria-label={`Next ${label}`} size="icon-md" disabled={!canScrollForward} onClick={() => move(1)}>
            <ChevronRight size={17} />
          </IconButton>
        </div>
      </div>
      <div
        ref={viewportRef}
        role="region"
        aria-label={label}
        tabIndex={0}
        onScroll={updateControls}
        className="scrollbar-hidden overflow-x-auto overscroll-x-contain scroll-smooth focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-border-focus/60"
      >
        <div className="flex w-max min-w-full snap-x snap-proximity gap-4 pb-1">
          {React.Children.map(children, (child) => (
            <div className={cn("snap-start", itemClassName)}>{child}</div>
          ))}
        </div>
      </div>
    </div>
  );
}
