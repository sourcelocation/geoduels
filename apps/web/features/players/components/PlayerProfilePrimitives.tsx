import {
  CircleMinus,
  ExternalLink,
  Gamepad2,
  Percent,
  Flame,
  Crown,
  Sparkles,
  Target,
  TrendingDown,
  TrendingUp,
  Trophy,
  X,
  type LucideIcon,
} from "lucide-react";
import { Badge } from "../../../components/ui/Badge";
import { Button } from "../../../components/ui/button";
import Link from "next/link";
import PlayerBadge, {
  badgeTitle,
  type PlayerBadgeInfo,
} from "./PlayerBadge";
import { RelativeTime } from "../../../components/ui/RelativeTime";
import { Tooltip } from "../../../components/ui/Tooltip";
import { cn } from "../../../lib/cn";
import { toPublicEntityId } from "../../../lib/entity-id";
import type { PlayerMatchSummary } from "../types";

export const profileMetrics = {
  wins: Trophy,
  winRate: Percent,
  winStreak: Flame,
  perfectGuesses: Target,
  flawlessWins: Crown,
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
    return <p className="text-body-sm font-semibold text-content-secondary">No badges earned yet.</p>;
  }

  return (
    <div className="flex flex-wrap gap-3">
      {editing ? (
        <Button
          variant="ghost"
          type="button"
          onClick={() => onSelect?.("")}
          className={cn(
            "flex h-16 w-16 items-center justify-center rounded-xl border text-label font-strong uppercase transition",
            !selectedBadgeId
              ? "border-status-success/70 bg-status-success/15 text-content-primary"
              : "border-border-default bg-surface-grouped text-content-secondary hover:border-border-strong hover:bg-surface-fill",
          )}
        >
          None
        </Button>
      ) : null}
      {badges.map((badge) => {
        const selected = selectedBadgeId === badge.id;
        const medal = editing ? (
          <Button
            variant="ghost"
            type="button"
            onClick={() => onSelect?.(badge.id)}
            aria-label={`Display ${badge.label}`}
            className={cn(
              "flex h-16 w-16 items-center justify-center rounded-xl border transition",
              selected
                ? "border-status-success/70 bg-status-success/15 shadow-elev-1"
                : "border-border-default bg-surface-grouped hover:border-border-strong hover:bg-surface-fill",
            )}
          >
            <PlayerBadge badge={badge} size="lg" />
          </Button>
        ) : (
          <span
            tabIndex={0}
            aria-label={badgeTitle(badge)}
            className="flex h-16 w-16 items-center justify-center rounded-xl border border-border-default bg-surface-fill outline-none focus:border-border-focus"
          >
            <PlayerBadge badge={badge} size="lg" />
          </span>
        );
        return (
          <Tooltip key={badge.id} content={
            <span>
              <strong className="block text-content-primary">{badge.label}</strong>
              {badge.description ? (
                <span className="mt-0.5 block text-content-secondary">
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
      ? "text-content-secondary"
      : match.ratingDelta > 0
        ? "text-status-success"
        : "text-status-danger";
  const RatingIcon =
    typeof match.ratingDelta === "number" && match.ratingDelta < 0
      ? TrendingDown
      : TrendingUp;

  return (
    <Link
      href={`/match/${encodeURIComponent(toPublicEntityId(match.matchId))}`}
      className="group grid grid-cols-[minmax(0,1fr)_auto] gap-x-4 gap-y-3 rounded-lg border border-border-default bg-surface-grouped px-3.5 py-3 transition hover:border-border-strong hover:bg-surface-fill sm:grid-cols-[minmax(180px,0.85fr)_minmax(160px,1fr)_120px_24px] sm:items-center"
    >
      <div className="flex min-w-0 items-center gap-3">
        <span
          className={cn(
            "flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-surface-fill",
            outcome.tone,
          )}
        >
          <OutcomeIcon size={17} aria-hidden="true" />
        </span>
        <div className="min-w-0">
          <div className={cn("font-strong capitalize", outcome.tone)}>
            {match.outcome}
          </div>
          <div className="mt-0.5 text-caption font-semibold text-content-secondary">
            <RelativeTime value={match.endedAt} />
          </div>
        </div>
      </div>

      <div className="col-start-1 flex min-w-0 flex-wrap items-center gap-2 pl-12 sm:col-start-auto sm:pl-0">
        <span className="inline-flex min-w-0 items-center gap-1.5 text-body-sm font-strong text-content-primary">
          <Gamepad2 size={15} className="shrink-0 text-status-success" aria-hidden="true" />
          <span className="truncate">
            {match.opponentDisplayName
              ? `vs ${match.opponentDisplayName}`
              : formatMode(match.mode)}
          </span>
        </span>
        {match.ranked ? (
          <Badge className="px-2 py-0.5 text-caption">Ranked</Badge>
        ) : null}
      </div>

      <div className="col-start-2 row-span-2 row-start-1 flex min-w-[90px] items-center justify-end text-body-sm font-strong sm:col-start-auto sm:row-span-1 sm:row-start-auto">
        {singleplayer && typeof match.totalScore === "number" ? (
          <span className="inline-flex items-center gap-1.5 text-content-primary">
            <Target size={15} className="text-status-success" aria-hidden="true" />
            {match.totalScore.toLocaleString()}
            <span className="text-caption font-strong text-content-secondary">pts</span>
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
          className="text-content-secondary transition group-hover:text-content-primary"
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
  if (outcome === "win") return { icon: Trophy, tone: "text-status-success" };
  if (outcome === "loss") return { icon: X, tone: "text-status-danger" };
  if (outcome === "draw") return { icon: CircleMinus, tone: "text-status-warning" };
  return { icon: Sparkles, tone: "text-status-info" };
}

function formatMode(mode: string) {
  return mode
    .split("_")
    .map((part) => part.slice(0, 1).toUpperCase() + part.slice(1))
    .join(" ");
}
