import {
  FloatingArrow,
  FloatingDelayGroup,
  FloatingPortal,
  arrow,
  autoUpdate,
  flip,
  offset,
  shift,
  size,
  useDelayGroup,
  useDismiss,
  useFloating,
  useFocus,
  useHover,
  useInteractions,
  useMergeRefs,
  useRole,
  type Delay,
  type Placement,
} from "@floating-ui/react";
import React, { useRef, useState } from "react";
import { cn } from "../../lib/cn";

type TooltipSide = "top" | "right" | "bottom" | "left";
type TooltipAlign = "start" | "center" | "end";

export type TooltipProps = {
  children: React.ReactElement;
  content: React.ReactNode;
  side?: TooltipSide;
  align?: TooltipAlign;
  delay?: Delay;
  disabled?: boolean;
  className?: string;
};

export function TooltipProvider({ children }: { children: React.ReactNode }) {
  return (
    <FloatingDelayGroup delay={{ open: 650, close: 100 }} timeoutMs={400}>
      {children}
    </FloatingDelayGroup>
  );
}

function tooltipPlacement(side: TooltipSide, align: TooltipAlign): Placement {
  return align === "center" ? side : `${side}-${align}`;
}

export function Tooltip({
  children,
  content,
  side = "top",
  align = "center",
  delay,
  disabled = false,
  className,
}: TooltipProps) {
  const [open, setOpen] = useState(false);
  const arrowRef = useRef<SVGSVGElement>(null);
  const enabled = !disabled && content !== null && content !== undefined && content !== "";
  const { refs, floatingStyles, context } = useFloating({
    open,
    onOpenChange: setOpen,
    placement: tooltipPlacement(side, align),
    whileElementsMounted: autoUpdate,
    middleware: [
      offset(8),
      flip({ padding: 8 }),
      shift({ padding: 8 }),
      size({
        padding: 8,
        apply({ availableWidth, elements }) {
          elements.floating.style.maxWidth = `${Math.max(0, Math.min(280, availableWidth))}px`;
        },
      }),
      arrow({ element: arrowRef, padding: 5 }),
    ],
  });
  const { delay: groupedDelay } = useDelayGroup(context);
  const hover = useHover(context, {
    enabled,
    delay: delay ?? groupedDelay,
    mouseOnly: true,
    move: false,
  });
  const focus = useFocus(context, { enabled });
  const dismiss = useDismiss(context, { enabled });
  const role = useRole(context, { enabled, role: "tooltip" });
  const { getReferenceProps, getFloatingProps } = useInteractions([hover, focus, dismiss, role]);
  const childRef = (children as React.ReactElement & { ref?: React.Ref<HTMLElement> }).ref;
  const mergedRef = useMergeRefs([refs.setReference, childRef]);
  const reference = React.cloneElement(
    children,
    getReferenceProps({
      ...children.props,
      ref: mergedRef,
    }),
  );

  return (
    <>
      {reference}
      {enabled && open ? (
        <FloatingPortal>
          <div
            ref={refs.setFloating}
            style={floatingStyles}
            className={cn(
              "pointer-events-none z-[2300] break-words rounded-md bg-slate-700 px-2.5 py-1.5 text-center text-xs font-semibold leading-4 text-white shadow-lg",
              className,
            )}
            {...getFloatingProps()}
          >
            {content}
            <FloatingArrow
              ref={arrowRef}
              context={context}
              width={12}
              height={6}
              tipRadius={1}
              className="fill-slate-700"
            />
          </div>
        </FloatingPortal>
      ) : null}
    </>
  );
}
