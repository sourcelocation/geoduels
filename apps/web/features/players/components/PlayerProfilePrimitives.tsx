import {
  CircleMinus,
  ExternalLink,
  Gamepad2,
  Medal,
  Percent,
  ShieldCheck,
  Sparkles,
  Target,
  TrendingDown,
  TrendingUp,
  Trophy,
  X,
  type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import PlayerBadge, {
  badgeTitle,
  type PlayerBadgeInfo,
} from "../../../components/ui/PlayerBadge";
import { RelativeTime } from "../../../components/ui/RelativeTime";
import { Tooltip } from "../../../components/ui/Tooltip";
import { cn } from "../../../lib/cn";
import { toPublicEntityId } from "../../../lib/entity-id";
import type { PlayerMatchSummary } from "../types";

export const profileMetrics = {
  games: Gamepad2,
  wins: Trophy,
  winRate: Percent,
  rankedGames: ShieldCheck,
  rankedWins: Medal,
} as const;

export function ProfileBadgeCollection({
  badges,
  editing = false,
  selectedBadgeId,
  onSelect,
}: {
  badges: PlayerBadgeInfo[];
  editing?: boolean;
  selectedBadgeId?: string;
  onSelect?: (badgeId: string) => void;
}) {
  if (!badges.length) {
    return <p className="text-sm font-semibold text-[#8caab0]">No badges earned yet.</p>;
  }

  return (
    <div className="flex flex-wrap gap-3">
      {editing ? (
        <button
          type="button"
          onClick={() => onSelect?.("")}
          className={cn(
            "flex h-16 w-16 items-center justify-center rounded-2xl border text-[10px] font-black uppercase tracking-[0.08em] transition",
            !selectedBadgeId
              ? "border-accentPrimary/70 bg-accentPrimary/15 text-white"
              : "border-white/10 bg-black/20 text-[#8caab0] hover:border-white/25",
          )}
        >
          None
        </button>
      ) : null}
      {badges.map((badge) => {
        const selected = selectedBadgeId === badge.id;
        const medal = editing ? (
          <button
            type="button"
            onClick={() => onSelect?.(badge.id)}
            aria-label={`Display ${badge.label}`}
            className={cn(
              "flex h-16 w-16 items-center justify-center rounded-2xl border transition",
              selected
                ? "border-accentPrimary/70 bg-accentPrimary/15 shadow-elev-1"
                : "border-white/10 bg-black/20 hover:border-white/25 hover:bg-white/[0.05]",
            )}
          >
            <PlayerBadge badge={badge} size="lg" />
          </button>
        ) : (
          <span
            tabIndex={0}
            aria-label={badgeTitle(badge)}
            className="flex h-16 w-16 items-center justify-center rounded-2xl border border-white/[0.07] bg-gradient-to-b from-white/[0.055] to-black/10 outline-none focus:border-white/25"
          >
            <PlayerBadge badge={badge} size="lg" />
          </span>
        );
        return (
          <Tooltip key={badge.id} content={
            <span>
              <strong className="block text-white">{badge.label}</strong>
              {badge.description ? (
                <span className="mt-0.5 block text-slate-200">
                  {badge.description}
                </span>
              ) : null}
            </span>
          } side="bottom">
            {medal}
          </Tooltip>
        );
      })}
    </div>
  );
}

export function MatchHistoryRow({ match }: { match: PlayerMatchSummary }) {
  const outcome = outcomePresentation(match.outcome);
  const OutcomeIcon = outcome.icon;
  const singleplayer = match.mode === "singleplayer";
  const duel = match.mode === "duel" || match.mode === "team_duel";
  const ratingTone =
    typeof match.ratingDelta !== "number" || match.ratingDelta === 0
      ? "text-[#a9bfd4]"
      : match.ratingDelta > 0
        ? "text-emerald-300"
        : "text-red-300";
  const RatingIcon =
    typeof match.ratingDelta === "number" && match.ratingDelta < 0
      ? TrendingDown
      : TrendingUp;

  return (
    <Link
      href={`/match/${encodeURIComponent(toPublicEntityId(match.matchId))}`}
      className="group grid grid-cols-[minmax(0,1fr)_auto] gap-x-4 gap-y-3 rounded-xl border border-white/10 bg-black/20 px-3.5 py-3 transition hover:border-white/20 hover:bg-white/[0.06] sm:grid-cols-[minmax(180px,0.85fr)_minmax(160px,1fr)_120px_24px] sm:items-center"
    >
      <div className="flex min-w-0 items-center gap-3">
        <span
          className={cn(
            "flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-white/[0.05]",
            outcome.tone,
          )}
        >
          <OutcomeIcon size={17} aria-hidden="true" />
        </span>
        <div className="min-w-0">
          <div className={cn("font-extrabold capitalize", outcome.tone)}>
            {match.outcome}
          </div>
          <div className="mt-0.5 text-xs font-semibold text-[#a9bfd4]">
            <RelativeTime value={match.endedAt} />
          </div>
        </div>
      </div>

      <div className="col-start-1 flex min-w-0 flex-wrap items-center gap-2 pl-12 sm:col-start-auto sm:pl-0">
        <span className="inline-flex min-w-0 items-center gap-1.5 text-sm font-bold text-white">
          <Gamepad2 size={15} className="shrink-0 text-[#77f0be]" aria-hidden="true" />
          <span className="truncate">
            {match.opponentDisplayName
              ? `vs ${match.opponentDisplayName}`
              : formatMode(match.mode)}
          </span>
        </span>
        {match.ranked ? (
          <span className="rounded-full bg-white/[0.07] px-2 py-0.5 text-[9px] font-black uppercase tracking-[0.1em] text-[#a9bfd4]">
            Ranked
          </span>
        ) : null}
      </div>

      <div className="col-start-2 row-span-2 row-start-1 flex min-w-[90px] items-center justify-end text-sm font-extrabold sm:col-start-auto sm:row-span-1 sm:row-start-auto">
        {singleplayer && typeof match.totalScore === "number" ? (
          <span className="inline-flex items-center gap-1.5 text-white">
            <Target size={15} className="text-[#77f0be]" aria-hidden="true" />
            {match.totalScore.toLocaleString()}
            <span className="text-[10px] font-bold text-[#8caab0]">pts</span>
          </span>
        ) : duel &&
          typeof match.ratingDelta === "number" &&
          match.ratingDelta !== 0 ? (
          <span className={cn("inline-flex items-center gap-1", ratingTone)}>
            <RatingIcon size={15} aria-hidden="true" />
            {match.ratingDelta > 0 ? "+" : ""}
            {match.ratingDelta}
          </span>
        ) : null}
      </div>
      <div className="col-start-2 row-start-2 flex items-center justify-end sm:col-start-auto sm:row-start-auto">
        <ExternalLink
          size={15}
          className="text-[#718b94] transition group-hover:text-white"
          aria-hidden="true"
        />
      </div>
    </Link>
  );
}

function outcomePresentation(outcome: PlayerMatchSummary["outcome"]): {
  icon: LucideIcon;
  tone: string;
} {
  if (outcome === "win") return { icon: Trophy, tone: "text-emerald-300" };
  if (outcome === "loss") return { icon: X, tone: "text-red-300" };
  if (outcome === "draw") return { icon: CircleMinus, tone: "text-amber-200" };
  return { icon: Sparkles, tone: "text-sky-200" };
}

function formatMode(mode: string) {
  return mode
    .split("_")
    .map((part) => part.slice(0, 1).toUpperCase() + part.slice(1))
    .join(" ");
}
