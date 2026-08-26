export type PlayerBadgeInfo = {
  id: string;
  kind: string;
  label: string;
  description: string;
  imageUrl: string;
  rarity?: string;
  level?: number;
  maxLevel?: number;
  extra?: number;
  owned?: boolean;
  unobtainable?: boolean;
};

type PlayerBadgeProps = {
  badge?: PlayerBadgeInfo | null;
  size?: "sm" | "md" | "lg";
  muted?: boolean;
  className?: string;
};

const sizeClass = {
  sm: "h-7 w-7",
  md: "h-9 w-9",
  lg: "h-14 w-14",
};

const extraTextClass = {
  sm: "text-caption",
  md: "text-label",
  lg: "text-heading-sm",
};

export function badgeTitle(badge?: PlayerBadgeInfo | null) {
  if (!badge) return "";
  return `${badge.label}${badge.description ? ` - ${badge.description}` : ""}`;
}

export default function PlayerBadge({
  badge,
  size = "sm",
  muted = false,
  className = "",
}: PlayerBadgeProps) {
  if (!badge) return null;
  const extraLabel =
    badge.kind === "legacy_top_finish" && badge.extra ? `#${badge.extra}` : "";
  return (
    <span
      className={`relative inline-flex shrink-0 items-center justify-center ${sizeClass[size]} ${muted ? "grayscale opacity-50" : ""} ${className}`.trim()}
      aria-label={badgeTitle(badge)}
    >
      <img
        src={badge.imageUrl}
        alt=""
        className="h-full w-full object-contain drop-shadow-md"
      />
      {extraLabel ? (
        <span
          className={`font-hud absolute inset-x-0 bottom-0 flex items-center justify-center font-strong leading-collapsed text-content-on-action drop-shadow-sm ${extraTextClass[size]}`}
        >
          {extraLabel}
        </span>
      ) : null}
    </span>
  );
}
