import { type ReactNode, useEffect } from "react";
import { motion } from "framer-motion";
import { X } from "lucide-react";

type AppModalShellProps = {
  title: string;
  children: ReactNode;
  onClose?: () => void;
  placement?: "responsive" | "center";
  showHeader?: boolean;
  closeOnBackdrop?: boolean;
  zIndexClassName?: string;
  maxWidthClassName?: string;
  panelClassName?: string;
  contentClassName?: string;
};

export default function AppModalShell({
  title,
  children,
  onClose,
  placement = "responsive",
  showHeader = true,
  closeOnBackdrop = Boolean(onClose),
  zIndexClassName = "z-[2200]",
  maxWidthClassName = "max-w-lg",
  panelClassName = "",
  contentClassName = "",
}: AppModalShellProps) {
  useEffect(() => {
    if (!onClose) return undefined;
    const onKeyDown = (e: globalThis.KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.2, ease: "easeOut" }}
      className={`fixed inset-0 ${zIndexClassName} flex justify-center bg-black/60 backdrop-blur-md ${
        placement === "center"
          ? "items-center p-4"
          : "items-end p-0 sm:items-center sm:p-4"
      }`}
      onClick={closeOnBackdrop ? onClose : undefined}
    >
      <motion.div
        initial={{ opacity: 0, scale: 0.95 }}
        animate={{ opacity: 1, scale: 1 }}
        exit={{ opacity: 0, scale: 0.95 }}
        transition={{ type: "spring", stiffness: 350, damping: 30 }}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        onClick={(e) => e.stopPropagation()}
        className={`glass-panel max-h-[85vh] w-full ${maxWidthClassName} overflow-y-auto rounded-t-[26px] p-5 text-[#f4f9ff] sm:rounded-[26px] sm:p-6 ${panelClassName}`}
      >
        {showHeader ? (
          <div className="mb-5 flex items-center justify-between sm:mb-6">
            <h2 className="text-[18px] font-black uppercase tracking-[0.12em] text-white sm:text-[22px]">
              {title}
            </h2>
            {onClose ? (
              <button
                type="button"
                onClick={onClose}
                aria-label={`Close ${title}`}
                className="flex h-8 w-8 items-center justify-center rounded-full bg-white/5 text-white/70 transition hover:bg-white/10 hover:text-white"
              >
                <X size={18} strokeWidth={2.5} />
              </button>
            ) : null}
          </div>
        ) : null}
        <div className={contentClassName}>{children}</div>
      </motion.div>
    </motion.div>
  );
}
