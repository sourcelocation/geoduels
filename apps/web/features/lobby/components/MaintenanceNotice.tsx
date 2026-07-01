import { Loader2, X } from "lucide-react";
import { motion } from "framer-motion";
import AppModalShell from "../../../components/ui/AppModalShell";
import { LobbyNotice } from "./lobby-primitives";

export function MaintenanceBanner({
  message,
  countdown,
  onDismiss,
}: { message: string; countdown: string; onDismiss?: () => void }) {
  const trailing = onDismiss ? (
    <div className="flex items-center gap-2">
      <span>{countdown || "Soon"}</span>
      <button
        type="button"
        onClick={onDismiss}
        className="inline-flex h-8 w-8 items-center justify-center rounded-full border border-white/15 bg-black/15 text-white/80 transition hover:bg-white/15 hover:text-white"
        aria-label="Hide maintenance alert"
      >
        <X size={16} />
      </button>
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
      zIndexClassName="z-[2100]"
      maxWidthClassName="max-w-[560px]"
      panelClassName="p-7 sm:p-10"
    >
      <div className="relative flex flex-col items-center text-center">
        {onDismiss ? (
          <button
            type="button"
            onClick={onDismiss}
            className="absolute right-0 top-0 inline-flex h-9 w-9 items-center justify-center rounded-full border border-white/15 bg-white/[0.06] text-white/75 transition hover:bg-white/[0.12] hover:text-white"
            aria-label="Hide maintenance alert"
          >
            <X size={17} />
          </button>
        ) : null}
        <div className="mb-5 flex h-16 w-16 items-center justify-center rounded-full border border-[#f4c84c]/30 bg-[#f4c84c]/10">
          <Loader2 size={30} className="animate-spin text-[#f4c84c]" />
        </div>
        <p className="text-[11px] font-black uppercase tracking-[0.22em] text-[#f4d98a]">Maintenance Break</p>
        <h2 className="mt-3 text-[30px] font-black tracking-tight text-white sm:text-[38px]">We&apos;ll Be Back Shortly</h2>
        <p className="mt-3 max-w-[42ch] text-[15px] leading-relaxed text-[#d9e7f5]">
          {message || "GeoDuels is temporarily offline while we finish a scheduled upgrade."}
        </p>
        <div className="mt-6 rounded-[20px] border border-white/10 bg-white/5 px-5 py-4">
          <p className="text-[11px] font-bold uppercase tracking-[0.16em] text-[#a9bfd4]">
            Approximate Time
          </p>
          <p className="mt-2 text-[18px] font-extrabold text-white">{eta || "A few minutes"}</p>
        </div>
      </div>
    </AppModalShell>
  );
}
