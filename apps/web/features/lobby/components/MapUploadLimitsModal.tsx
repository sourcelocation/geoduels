import { AlertTriangle, CheckCircle2 } from "lucide-react";
import AppModalShell from "../../../components/ui/AppModalShell";
import type { MapUploadQuota } from "../../maps/lib/maps-client";

type MapUploadLimitsModalProps = {
  blockedReason?: string;
  onClose: () => void;
  quota?: MapUploadQuota;
};

const tiers = [
  { name: "Base", requirement: "Registered account", maps: 10, locations: "200,000", throughput: "300,000" },
  { name: "Trusted", requirement: "25 qualified favorites · account age 14 days", maps: 25, locations: "500,000", throughput: "600,000" },
  { name: "Established", requirement: "100 qualified favorites across 2 maps · account age 30 days", maps: 100, locations: "1,000,000", throughput: "1,000,000" },
];

export function MapUploadLimitsModal({ blockedReason, onClose, quota }: MapUploadLimitsModalProps) {
  return (
    <AppModalShell title="Map Upload Limits" onClose={onClose} placement="center" maxWidthClassName="max-w-2xl">
      <div className="space-y-4 text-sm text-[#a9bfd4]">
        {blockedReason ? (
          <div className="flex gap-3 rounded-xl border border-amber-300/20 bg-amber-300/10 p-4 text-amber-100">
            <AlertTriangle className="mt-0.5 shrink-0" size={18} />
            <div>
              <p className="font-black text-white">Why you can’t upload a new map</p>
              <p className="mt-1">{blockedReason}</p>
            </div>
          </div>
        ) : null}

        {quota ? (
          <div className="rounded-xl border border-white/10 bg-white/[0.04] p-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div>
                <p className="text-xs font-black uppercase tracking-[0.12em] text-[#77f0be]">{quota.tier} creator tier</p>
                <p className="mt-1 text-white">
                  {quota.currentMaps.toLocaleString()} / {quota.maxMaps.toLocaleString()} maps ·{" "}
                  {quota.currentActiveLocations.toLocaleString()} / {quota.maxActiveLocations.toLocaleString()} active locations
                </p>
              </div>
              {!blockedReason ? <CheckCircle2 className="text-[#77f0be]" size={22} /> : null}
            </div>
            {quota.nextTier ? (
              <p className="mt-3 text-xs leading-5">
                To reach <span className="font-bold capitalize text-white">{quota.nextTier}</span>:{" "}
                {quota.favoritesNeeded || 0} more qualified favorites
                {quota.mapsNeeded ? ` across ${quota.mapsNeeded} more map${quota.mapsNeeded === 1 ? "" : "s"}` : ""}
                {quota.daysNeeded ? ` and ${quota.daysNeeded} more account day${quota.daysNeeded === 1 ? "" : "s"}` : ""}.
              </p>
            ) : (
              <p className="mt-3 text-xs text-[#77f0be]">You have unlocked the highest creator tier.</p>
            )}
          </div>
        ) : (
          <p className="rounded-xl border border-white/10 bg-white/[0.04] p-4">Creator limits are still loading.</p>
        )}

        <div className="overflow-hidden rounded-xl border border-white/10">
          {tiers.map((tier) => (
            <div key={tier.name} className="grid gap-2 border-b border-white/10 p-4 last:border-b-0 sm:grid-cols-[120px_1fr]">
              <p className="font-black text-white">{tier.name}</p>
              <div className="space-y-1 text-xs leading-5">
                <p>{tier.requirement}</p>
                <p>{tier.maps} maps · {tier.locations} active locations · {tier.throughput} uploaded locations/hour</p>
              </div>
            </div>
          ))}
        </div>

        <p className="text-xs leading-5 text-[#6f8998]">
          Every tier allows 10 uploads per hour and 30 per day. Favorites count once per eligible registered player.
          Active moderation restrictions force Base limits. Existing maps are preserved if your tier falls.
        </p>
      </div>
    </AppModalShell>
  );
}
