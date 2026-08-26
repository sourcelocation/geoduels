import { MapPin } from 'lucide-react';

const NO_GUESS_DISTANCE_THRESHOLD_KM = 20000;

function formatDistanceLabel(distanceKm: number, isDuel: boolean) {
  if (isDuel && distanceKm >= NO_GUESS_DISTANCE_THRESHOLD_KM) return 'rip';
  return `${Math.round(distanceKm).toLocaleString()} km`;
}

export function ResultDistanceBar({
  selfDistanceKm,
  oppDistanceKm,
  compact = false,
}: {
  selfDistanceKm?: number;
  oppDistanceKm?: number;
  compact?: boolean;
}) {
  if (selfDistanceKm === undefined) return null;

  const iconSize = compact ? 'w-[28px] h-[28px]' : 'w-[40px] h-[40px] md:w-[48px] md:h-[48px]';
  const iconPositionClass = oppDistanceKm === undefined ? 'right-0 translate-x-1/2' : 'left-1/2 -translate-x-1/2';
  const containerClass = compact
    ? 'h-[48px] md:h-[56px] w-[140px] md:w-[170px] rounded-lg'
    : 'h-[48px] md:h-[56px] w-[280px] md:w-[340px] rounded-lg border-[4px] border-brand-blue';
  const selfInsetClass = oppDistanceKm === undefined ? 'rounded-md' : 'rounded-l-md';
  const selfPaddingClass = oppDistanceKm === undefined ? '' : 'pr-[30px]';
  const isDuel = oppDistanceKm !== undefined;
  const soloColor = 'rgb(var(--gd-status-success))';
  const iconBackground = compact ? soloColor : 'rgb(var(--gd-status-info))';
  const iconBorder = compact ? soloColor : 'rgb(var(--gd-status-info))';
  const iconForeground = compact ? 'var(--gd-content-inverse)' : 'var(--gd-content-primary)';
  const bgColor = compact ? soloColor : 'rgb(var(--gd-status-info) / 0.55)';

  return (
    <div className={`${containerClass} relative flex overflow-visible drop-shadow-lg`} style={{ backgroundColor: bgColor }}>
      <div className={`relative ${!isDuel ? 'w-full rounded-md' : 'flex-1 rounded-l-md'} flex items-center px-2 ${selfPaddingClass}`} style={{ backgroundColor: compact ? soloColor : 'rgb(var(--gd-status-success) / 0.8)' }}>
        <div className={`pointer-events-none absolute inset-[3px] ${selfInsetClass} border-[2.5px] border-dotted border-content-on-action`} />
        <span className="z-content w-full truncate text-center text-heading-sm font-strong italic tracking-heading text-content-on-action drop-shadow-sm">{formatDistanceLabel(selfDistanceKm, isDuel)}</span>
      </div>
      {isDuel ? <div className="relative flex flex-1 items-center rounded-r-md px-2 pl-[30px]" style={{ backgroundColor: bgColor }}><span className="z-content w-full truncate text-center text-heading-sm font-strong italic tracking-heading text-brand-blue drop-shadow-sm">{formatDistanceLabel(oppDistanceKm, true)}</span></div> : null}
      <div className={`absolute top-1/2 ${iconPositionClass} z-sticky flex ${iconSize} -translate-y-1/2 flex-col items-center justify-center rounded-full border-[3px] shadow-inner`} style={{ backgroundColor: iconBackground, borderColor: iconBorder }}>
        <MapPin size={22} fill="currentColor" color={iconForeground} strokeWidth={1} className="mt-0.5 scale-90 drop-shadow-sm md:scale-100" />
        <div className="w-[14px] -mt-[3px] border-b-2 border-dotted border-content-on-action md:-mt-[2px]" />
      </div>
    </div>
  );
}
