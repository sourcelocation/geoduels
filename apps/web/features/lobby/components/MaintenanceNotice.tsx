import { X } from "lucide-react";
import { motion } from "framer-motion";
import AppModalShell from "../../../components/ui/AppModalShell";
import { IconButton } from "../../../components/ui/button";
import { LobbyNotice } from "./lobby-primitives";
import { Spinner } from "../../../components/ui/Spinner";

export function MaintenanceBanner({
  message,
  countdown,
  onDismiss,
}: { message: string; countdown: string; onDismiss?: () => void }) {
  const trailing = onDismiss ? (
    <div className="flex items-center gap-2">
      <span>{countdown || "Soon"}</span>
      <IconButton
        onClick={onDismiss}
        className="h-8 min-h-8 w-8"
        aria-label="Hide maintenance alert"
      >
        <X size={16} />
      </IconButton>
    </div>
  ) : (
    countdown || "Soon"
  );

  return (
    <motion.div
      initial={{ opacity: 0, y: -14 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -10 }}
      transition={{ duration: 0.22, ease: "easeOut" }}
      className="mb-4"
    >
      <LobbyNotice title="Maintenance" tone="warning" trailing={trailing}>
        {message || "Queueing has been paused."}
      </LobbyNotice>
    </motion.div>
  );
}

export function MaintenanceOverlay({
  message,
  eta,
  onDismiss,
}: { message: string; eta: string; onDismiss?: () => void }) {
  return (
    <AppModalShell
      title="Maintenance Break"
      placement="center"
      showHeader={false}
      zIndexClassName="z-modal-critical"
      maxWidthClassName="max-w-[560px]"
    >
      <div className="relative flex flex-col items-center text-center">
        {onDismiss ? (
          <IconButton
            onClick={onDismiss}
            className="absolute right-0 top-0 h-9 min-h-9 w-9"
            aria-label="Hide maintenance alert"
          >
            <X size={17} />
          </IconButton>
        ) : null}
        <div className="mb-5 flex h-16 w-16 items-center justify-center rounded-full border border-status-warning/30 bg-status-warning/10">
          <Spinner size="lg" label="Maintenance in progress" color="current" className="text-status-warning" />
        </div>
        <p className="text-label font-strong text-status-warning">Maintenance Break</p>
        <h2 className="mt-3 text-display-md font-strong tracking-heading text-content-primary">We&apos;ll Be Back Shortly</h2>
        <p className="mt-3 max-w-[42ch] text-body leading-prose text-content-primary">
          {message || "GeoDuels is temporarily offline while we finish a scheduled upgrade."}
        </p>
        <div className="mt-6 rounded-xl border border-border-default bg-surface-grouped px-5 py-4">
          <p className="text-label font-strong text-content-secondary">
            Approximate Time
          </p>
          <p className="mt-2 text-heading-sm font-strong text-content-primary">{eta || "A few minutes"}</p>
        </div>
      </div>
    </AppModalShell>
  );
}
