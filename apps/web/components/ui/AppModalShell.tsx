import { type ReactNode, useCallback, useEffect, useId, useRef, useState } from "react";
import { motion } from "framer-motion";
import { X } from "lucide-react";
import { IconButton } from "./button";
import { registerModalDismissRequest } from "./modal-dismissal";

type AppModalShellProps = {
  title: string;
  description?: ReactNode;
  children: ReactNode;
  onClose?: () => void;
  placement?: "responsive" | "center";
  showHeader?: boolean;
  closeOnBackdrop?: boolean;
  zIndexClassName?: string;
  maxWidthClassName?: string;
  contentClassName?: string;
  role?: "dialog" | "alertdialog";
};

export default function AppModalShell({
  title,
  description,
  children,
  onClose,
  placement = "responsive",
  showHeader = true,
  closeOnBackdrop = Boolean(onClose),
  zIndexClassName = "z-popover",
  maxWidthClassName = "max-w-lg",
  contentClassName = "",
  role = "dialog",
}: AppModalShellProps) {
  const panelRef = useRef<HTMLDivElement>(null);
  const titleId = useId();
  const previousFocus = useRef<HTMLElement | null>(null);
  const onCloseRef = useRef(onClose);
  const closingRef = useRef(false);
  const closePromiseRef = useRef<Promise<void> | null>(null);
  const resolveCloseRef = useRef<(() => void) | null>(null);
  const [closing, setClosing] = useState(false);
  onCloseRef.current = onClose;

  const requestClose = useCallback(() => {
    if (closePromiseRef.current) return closePromiseRef.current;
    closePromiseRef.current = new Promise<void>((resolve) => {
      resolveCloseRef.current = resolve;
    });
    closingRef.current = true;
    setClosing(true);
    return closePromiseRef.current;
  }, []);

  useEffect(() => {
    const unregister = registerModalDismissRequest(requestClose);
    return () => {
      unregister();
      resolveCloseRef.current?.();
    };
  }, [requestClose]);

  useEffect(() => {
    previousFocus.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const panel = panelRef.current;
    const initial = panel?.querySelector<HTMLElement>("[autofocus], button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex='-1'])");
    (initial ?? panel)?.focus();
    const onKeyDown = (e: globalThis.KeyboardEvent) => {
      if (e.key === "Escape" && onCloseRef.current) { e.preventDefault(); requestClose(); }
      if (e.key !== "Tab" || !panel) return;
      const focusable = Array.from(panel.querySelectorAll<HTMLElement>("button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex='-1'])"));
      if (!focusable.length) { e.preventDefault(); panel.focus(); return; }
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
      if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => { window.removeEventListener("keydown", onKeyDown); previousFocus.current?.focus(); };
  }, [requestClose]);

  return (
    <motion.div
      variants={{ open: { opacity: 1 }, closed: { opacity: 0 } }}
      initial="closed"
      animate={closing ? "closed" : "open"}
      exit="closed"
      transition={{ duration: 0.2, ease: "easeOut" }}
      onAnimationComplete={(definition) => {
        if (definition !== "closed" || !closingRef.current) return;
        closingRef.current = false;
        resolveCloseRef.current?.();
        resolveCloseRef.current = null;
        onCloseRef.current?.();
      }}
        className={`fixed inset-0 ${zIndexClassName} flex justify-center bg-scrim backdrop-blur-md ${
        placement === "center"
          ? "items-center p-4"
          : "items-end p-0 sm:items-center sm:p-4"
      }`}
      onClick={closeOnBackdrop ? requestClose : undefined}
      data-architecture-exception="modal-backdrop"
      style={closing ? { pointerEvents: "none" } : undefined}
    >
      <motion.div
        variants={{ open: { opacity: 1, scale: 1 }, closed: { opacity: 0, scale: 0.95 } }}
        initial="closed"
        animate={closing ? "closed" : "open"}
        exit="closed"
        transition={{ type: "spring", stiffness: 350, damping: 30 }}
        ref={panelRef}
        role={role}
        aria-modal="true"
        aria-labelledby={showHeader ? titleId : undefined}
        aria-label={showHeader ? undefined : title}
        tabIndex={-1}
        onClick={(e) => e.stopPropagation()}
        data-architecture-exception="modal-panel"
        className={`translucent-surface relative max-h-[85vh] w-full ${maxWidthClassName} overflow-y-auto rounded-xl p-5 text-content-primary sm:p-6`}
      >
        {showHeader ? (
          <div className="mb-5 flex items-center justify-between gap-4 sm:mb-6">
            <div className="min-w-0">
              <h2 id={titleId} className="text-heading-md font-strong tracking-heading text-content-primary sm:text-heading-lg">{title}</h2>
              {description ? <p className="mt-1 text-body-sm text-content-secondary">{description}</p> : null}
            </div>
            {onClose ? (
              <IconButton
                type="button"
                onClick={requestClose}
                aria-label={`Close ${title}`}
                className="h-8 min-h-8 w-8 shrink-0"
              >
                <X size={18} strokeWidth={2.5} />
              </IconButton>
            ) : null}
          </div>
        ) : null}
        <div className={contentClassName}>{children}</div>
      </motion.div>
    </motion.div>
  );
}
