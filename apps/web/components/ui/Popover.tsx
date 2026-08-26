import {
  FloatingFocusManager,
  FloatingPortal,
  autoUpdate,
  flip,
  offset,
  shift,
  useClick,
  useDismiss,
  useFloating,
  useInteractions,
  useMergeRefs,
  useRole,
  type Placement,
} from "@floating-ui/react";
import * as React from "react";

import { cn } from "../../lib/cn";
import { surfaceClassName } from "./Surface";

export type PopoverProps = {
  children: React.ReactElement;
  content: React.ReactNode;
  open?: boolean;
  defaultOpen?: boolean;
  onOpenChange?: (open: boolean) => void;
  placement?: Placement;
  className?: string;
  modal?: boolean;
};

/** Click-triggered, focus-managed floating surface for compact app controls. */
export function Popover({
  children,
  content,
  open: controlledOpen,
  defaultOpen = false,
  onOpenChange,
  placement = "bottom-end",
  className,
  modal = false,
}: PopoverProps) {
  const [uncontrolledOpen, setUncontrolledOpen] = React.useState(defaultOpen);
  const open = controlledOpen ?? uncontrolledOpen;
  const setOpen = React.useCallback((nextOpen: boolean) => {
    if (controlledOpen === undefined) setUncontrolledOpen(nextOpen);
    onOpenChange?.(nextOpen);
  }, [controlledOpen, onOpenChange]);
  const { refs, floatingStyles, context } = useFloating({
    open,
    onOpenChange: setOpen,
    placement,
    whileElementsMounted: autoUpdate,
    middleware: [offset(8), flip({ padding: 8 }), shift({ padding: 8 })],
  });
  const click = useClick(context);
  const dismiss = useDismiss(context, { outsidePressEvent: "mousedown" });
  const role = useRole(context, { role: "dialog" });
  const { getReferenceProps, getFloatingProps } = useInteractions([click, dismiss, role]);
  React.useEffect(() => {
    if (!open) return undefined;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [open, setOpen]);
  const childRef = (children as React.ReactElement & { ref?: React.Ref<HTMLElement> }).ref;
  const [theme, setTheme] = React.useState<string | undefined>();
  const setReference = React.useCallback((node: HTMLElement | null) => {
    refs.setReference(node);
    setTheme(node?.closest<HTMLElement>("[data-ui-theme]")?.dataset.uiTheme);
  }, [refs]);
  const mergedRef = useMergeRefs([setReference, childRef]);
  const reference = React.cloneElement(children, getReferenceProps({
    ...children.props,
    ref: mergedRef,
    "aria-expanded": open,
  }));

  return (
    <>
      {reference}
      {open ? (
        <FloatingPortal>
          <FloatingFocusManager context={context} modal={modal} returnFocus>
            <div
              ref={refs.setFloating}
              style={floatingStyles}
              data-ui-theme={theme}
              className={surfaceClassName({ material: "solid", level: 3, className: cn("z-tooltip w-max max-w-[min(24rem,calc(100vw-1rem))] p-2 font-body", className) })}
              {...getFloatingProps()}
            >
              {content}
            </div>
          </FloatingFocusManager>
        </FloatingPortal>
      ) : null}
    </>
  );
}
