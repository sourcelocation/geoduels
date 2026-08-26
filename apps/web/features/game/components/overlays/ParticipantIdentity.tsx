import type { ReactNode } from "react";
import AvatarBadge from "../../../players/components/AvatarBadge";
import { Badge } from "../../../../components/ui/Badge";
import { MmrDisplay, RatingTrophyIcon } from "../../../players/components/MmrDisplay";
export { RatingTrophyIcon } from "../../../players/components/MmrDisplay";
import PlayerNameWithBadge from "../../../players/components/PlayerNameWithBadge";
import PlayerProfileLink from "../../../players/components/PlayerProfileLink";
import type { RatingDeltaPreview } from "../../model/types";
import type {
  ParticipantIdentityView,
  PlayerIdentityView,
  TeamIdentityView,
} from "./participant-types";

export type {
  MatchSideConnection,
  MatchSidesView,
  MatchSideView,
  ParticipantIdentityView,
  PlayerIdentityView,
  TeamIdentityView,
} from "./participant-types";

type AvatarSize = "sm" | "md" | "lg" | "xl";

export function formatRatingDelta(value?: number) {
  if (value === undefined) return "";
  return value > 0 ? `+${value}` : `${value}`;
}

export function ParticipantAvatar({
  participant,
  size = "md",
  opponent = false,
  className = "",
}: {
  participant: ParticipantIdentityView;
  size?: AvatarSize;
  opponent?: boolean;
  className?: string;
}) {
  const avatar = (
    <AvatarBadge
      avatarUrl={participant.kind === "player" ? participant.avatarUrl : undefined}
      fallback={participant.avatarFallback}
      alt={participant.name}
      opponent={opponent}
      size={size}
      className={className}
      avatarColor={participant.kind === "team" ? participant.avatarColor : undefined}
    />
  );
  if (participant.kind === "team") return avatar;
  return <PlayerProfileLink userId={participant.id} nickname={participant.name} disabled={participant.isGuest} className="inline-flex">{avatar}</PlayerProfileLink>;
}

export function ParticipantName({
  participant,
  nameClassName = "",
  wrapperClassName = "",
}: {
  participant: ParticipantIdentityView;
  nameClassName?: string;
  wrapperClassName?: string;
}) {
  if (participant.kind === "team") {
    return (
      <span
        className={`inline-flex max-w-full items-center ${wrapperClassName}`.trim()}
      >
        <span className={`truncate ${nameClassName}`.trim()}>
          {participant.name}
        </span>
      </span>
    );
  }
  return (
    <PlayerNameWithBadge
      name={participant.name}
      userId={participant.id}
      profileDisabled={participant.isGuest}
      isAdmin={participant.isAdmin}
      selectedBadge={participant.selectedBadge}
      nameClassName={nameClassName}
      wrapperClassName={wrapperClassName}
    />
  );
}

export function PlayerRating({
  rating,
  ratingDelta,
  ratingPreview,
  trailingAction,
  compact = false,
  showRatingPreview = true,
  wrapRatingDelta = true,
}: {
  rating?: number;
  ratingDelta?: number;
  ratingPreview?: RatingDeltaPreview;
  trailingAction?: ReactNode;
  compact?: boolean;
  showRatingPreview?: boolean;
  wrapRatingDelta?: boolean;
}) {
  if (rating === undefined && !(showRatingPreview && ratingPreview) && !trailingAction) return null;
  const delta = formatRatingDelta(ratingDelta);
  return (
    <div className="flex flex-wrap items-center justify-center gap-2">
      {rating !== undefined ? (
        <div className="flex items-center gap-1.5">
          <MmrDisplay value={rating} size={compact ? "sm" : "md"} />
          {delta ? (
            <span className={ratingDelta && ratingDelta > 0 ? "text-status-success" : ratingDelta && ratingDelta < 0 ? "text-status-danger" : "text-content-secondary"}>
              {wrapRatingDelta ? `(${delta})` : delta}
            </span>
          ) : null}
        </div>
      ) : null}
      {showRatingPreview && ratingPreview ? (
        <Badge
          tone="neutral"
          className="border border-border-default bg-surface-inset text-caption text-content-secondary"
          aria-label={`Rating change preview: win ${formatRatingDelta(ratingPreview.win)}, lose ${formatRatingDelta(ratingPreview.lose)}`}
        >
          <span className="text-status-success">W {formatRatingDelta(ratingPreview.win)}</span>
          <span className="text-content-secondary">/</span>
          <span className="text-status-danger">L {formatRatingDelta(ratingPreview.lose)}</span>
        </Badge>
      ) : null}
      {trailingAction}
    </div>
  );
}

export function ParticipantIdentityRow({
  participant,
  avatarSize = "sm",
  nameClassName = "font-strong text-content-primary",
  className = "",
  opponent = false,
}: {
  participant: ParticipantIdentityView;
  avatarSize?: AvatarSize;
  nameClassName?: string;
  className?: string;
  opponent?: boolean;
}) {
  return (
    <div className={`flex min-w-0 items-center gap-3 ${className}`.trim()}>
      <ParticipantAvatar participant={participant} size={avatarSize} opponent={opponent} />
      <ParticipantName
        participant={participant}
        nameClassName={nameClassName}
        wrapperClassName="min-w-0"
      />
    </div>
  );
}

export function ParticipantIdentityCard({
  participant,
  opponent = false,
  ratingAction,
  size = "xl",
  avatarClassName = "",
  nameClassName = "font-display text-heading-md font-strong text-content-primary",
  className = "",
  showRatingPreview = true,
  wrapRatingDelta = true,
}: {
  participant: ParticipantIdentityView;
  opponent?: boolean;
  ratingAction?: ReactNode;
  size?: AvatarSize;
  avatarClassName?: string;
  nameClassName?: string;
  className?: string;
  showRatingPreview?: boolean;
  wrapRatingDelta?: boolean;
}) {
  const resolvedRating =
    participant.kind === "player" ? participant.rating : undefined;
  const resolvedDelta =
    participant.kind === "player" ? participant.ratingDelta : undefined;
  const resolvedPreview =
    participant.kind === "player" ? participant.ratingPreview : undefined;
  return (
    <div className={`flex flex-col items-center gap-3 text-center ${className}`.trim()}>
      <ParticipantAvatar participant={participant} size={size} opponent={opponent} className={avatarClassName} />
      <div className="flex flex-col items-center">
        <ParticipantName
          participant={participant}
          nameClassName={nameClassName}
        />
        <div className="mt-1">
          <PlayerRating
            rating={resolvedRating}
            ratingDelta={resolvedDelta}
            ratingPreview={resolvedPreview}
            trailingAction={ratingAction}
            showRatingPreview={showRatingPreview}
            wrapRatingDelta={wrapRatingDelta}
          />
        </div>
      </div>
    </div>
  );
}
