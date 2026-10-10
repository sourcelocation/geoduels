import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { X } from "lucide-react";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { Button, IconButton } from "./button";
import { Surface } from "./Surface";

const AUTO_HIDE_MS = 4000;
const VISIBLE_TOASTS = 3;

/** A brief message at the top of the screen; it hides by itself. */
export type Toast = {
  title: ReactNode;
  body?: ReactNode;
  icon?: ReactNode;
  action?: { label: string; onClick: () => void };
  durationMs?: number;
};

type QueuedToast = Toast & { id: number; key?: string };

type AppNoticeContextValue = {
  show: (message: string) => void;
  toast: (toast: Toast) => void;
};

const AppNoticeContext = createContext<AppNoticeContextValue>({
  show: () => undefined,
  toast: () => undefined,
});

/** Toasts, a few at a time; the rest wait their turn rather than expiring unseen. */
export function AppNoticeProvider({ children }: { children: ReactNode }) {
  const [queue, setQueue] = useState<QueuedToast[]>([]);
  const nextId = useRef(0);

  const push = useCallback((toast: Toast, key?: string) => {
    nextId.current += 1;
    const queued = { ...toast, id: nextId.current, key };
    setQueue((items) => [...items.filter((item) => !key || item.key !== key), queued]);
  }, []);

  // The same message shown again restarts it instead of stacking a copy.
  const show = useCallback((message: string) => {
    const text = message.trim();
    if (text) push({ title: text }, text);
  }, [push]);

  const toast = useCallback((input: Toast) => push(input), [push]);

  const dismiss = useCallback((id: number) => {
    setQueue((items) => items.filter((item) => item.id !== id));
  }, []);

  const value = useMemo(() => ({ show, toast }), [show, toast]);

  return (
    <AppNoticeContext.Provider value={value}>
      {children}
      <div className="pointer-events-none fixed inset-x-0 top-3 z-popover flex flex-col items-center gap-2 px-4">
        <AnimatePresence initial={false}>
          {queue.slice(0, VISIBLE_TOASTS).map((item) => (
            <ToastView key={item.id} toast={item} onDismiss={dismiss} />
          ))}
        </AnimatePresence>
      </div>
    </AppNoticeContext.Provider>
  );
}

function ToastView({ toast, onDismiss }: { toast: QueuedToast; onDismiss: (id: number) => void }) {
  const reducedMotion = useReducedMotion();
  const { id, durationMs = AUTO_HIDE_MS } = toast;

  useEffect(() => {
    const timer = window.setTimeout(() => onDismiss(id), durationMs);
    return () => window.clearTimeout(timer);
  }, [durationMs, id, onDismiss]);

  return (
    <motion.div
      layout={!reducedMotion}
      initial={reducedMotion ? { opacity: 0 } : { opacity: 0, y: -12 }}
      animate={{ opacity: 1, y: 0 }}
      exit={reducedMotion ? { opacity: 0 } : { opacity: 0, y: -8 }}
      transition={{ duration: reducedMotion ? 0.12 : 0.22, ease: [0.16, 1, 0.3, 1] }}
      className="pointer-events-auto w-full max-w-md"
    >
      {/* Opaque: toasts float over the header and the game. */}
      <Surface material="solid" level={2} role="status" className="flex items-start gap-3 rounded-lg p-4">
        {toast.icon ? <span className="mt-0.5 shrink-0 text-content-secondary">{toast.icon}</span> : null}
        <div className="min-w-0 flex-1">
          <p className="text-body-sm font-strong text-content-primary">{toast.title}</p>
          {toast.body ? <p className="mt-1 text-body-sm text-content-secondary">{toast.body}</p> : null}
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {toast.action ? (
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => {
                toast.action?.onClick();
                onDismiss(id);
              }}
            >
              {toast.action.label}
            </Button>
          ) : null}
          <IconButton aria-label="Dismiss notice" size="icon-sm" onClick={() => onDismiss(id)}>
            <X size={14} />
          </IconButton>
        </div>
      </Surface>
    </motion.div>
  );
}

export function useAppNotice() {
  return useContext(AppNoticeContext);
}

/** Show an overlay notice whenever `message` becomes a non-empty string. */
export function useShowAppNoticeOnValue(message: string | null | undefined) {
  const { show } = useAppNotice();
  useEffect(() => {
    const text = message?.trim();
    if (text) show(text);
  }, [message, show]);
}

/** Show an overlay notice when `active` is true. Pass `key` to re-fire the same copy. */
export function useShowAppNoticeWhen(active: boolean, message: string, key?: number) {
  const { show } = useAppNotice();
  useEffect(() => {
    if (active) show(message);
  }, [active, key, message, show]);
}
