import { cn } from "../../lib/cn";

export function RatingTrophyIcon({ className = "" }: { className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 24 24"
      fill="currentColor"
      aria-hidden="true"
    >
      <path d="M19 4h-2V2H7v2H5C3.34 4 2 5.34 2 7v3c0 1.9 1.25 3.51 3 4.15V15c0 1.66 1.34 3 3 3h4c0 1.25-.84 2.33-2 2.8v2.2L12 24l2-1v-2.2c-1.16-.47-2-1.55-2-2.8h4c1.66 0 3-1.34 3-3v-.85c1.75-.64 3-2.25 3-4.15V7c0-1.66-1.34-3-3-3zM5 12c-.55 0-1-.45-1-1V7c0-.55.45-1 1-1h2v6H5zm14-1c0 .55-.45 1-1 1h-2V6h2c.55 0 1 .45 1 1v4z" />
    </svg>
  );
}

type MmrDisplayProps = {
  value: number;
  size?: "sm" | "md";
  label?: boolean;
  className?: string;
};

export function MmrDisplay({
  value,
  size = "sm",
  label = false,
  className,
}: MmrDisplayProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full bg-black/20 font-bold text-accentPrimary shadow-inner",
        size === "sm" ? "gap-1 px-2 py-1 text-xs" : "gap-1.5 px-3 py-1.5 text-sm",
        className,
      )}
      aria-label={`${value} MMR`}
    >
      <RatingTrophyIcon className={size === "sm" ? "h-3 w-3" : "h-4 w-4"} />
      <span>{value.toLocaleString()}</span>
      {label ? (
        <span className="text-[0.78em] uppercase tracking-[0.08em]">MMR</span>
      ) : null}
    </span>
  );
}
