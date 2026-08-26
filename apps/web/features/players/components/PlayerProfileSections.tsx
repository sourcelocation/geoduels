import { Check, Pencil, X } from "lucide-react";
import { CenteredSpinner, Spinner } from "../../../components/ui/Spinner";
import AvatarBadge from "./AvatarBadge";
import { Badge } from "../../../components/ui/Badge";
import { Button } from "../../../components/ui/button";
import { IconMetric } from "./IconMetric";
import { Input } from "../../../components/ui/input";
import { MmrDisplay } from "./MmrDisplay";
import PlayerBadge from "./PlayerBadge";
import { AppPanel } from "../../../components/ui/compositions";
import { Tabs } from "../../../components/ui/Tabs";
import type { useProfileEditor } from "../hooks/use-profile-editor";
import type { PlayerMatchSummary, PublicPlayerProfile } from "../types";
import {
  MatchHistoryRow,
  ProfileBadgeCollection,
  profileMetrics,
} from "./PlayerProfilePrimitives";

type Editor = ReturnType<typeof useProfileEditor>;

export function ProfileOverview({
  profile,
  editor,
  owner,
  socialActions,
}: {
  profile: PublicPlayerProfile;
  editor: Editor;
  owner: boolean;
  socialActions?: React.ReactNode;
}) {
  const rankedWinRate =
    profile.rankedGamesPlayed >= 10
      ? `${Math.round((profile.rankedWins / profile.rankedGamesPlayed) * 100)}%`
      : "—";
  const stats = [
    [profileMetrics.wins, "Duel wins", profile.wins],
    [profileMetrics.winRate, "Ranked win rate", rankedWinRate],
    [profileMetrics.winStreak, "Best win streak", profile.bestWinStreak],
    [profileMetrics.perfectGuesses, "Perfect guesses", profile.perfectGuesses],
    [profileMetrics.flawlessWins, "Flawless wins", profile.flawlessWins],
  ] as const;

  return (
    <AppPanel className="rounded-2xl p-5 sm:p-7">
      <div className="flex flex-col gap-5 sm:flex-row sm:items-center">
        <AvatarBadge
          avatarUrl={profile.avatarUrl}
          fallback={(profile.displayName || "?").slice(0, 1).toUpperCase()}
          alt={profile.displayName}
          size="xl"
          className="shrink-0 border-border-strong bg-surface-inset"
        />
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-center gap-2.5">
            {editor.editingName ? (
              <Input
                variant="game"
                value={editor.nickname}
                onChange={(event) =>
                  editor.setNickname(event.target.value.slice(0, 14))
                }
                onKeyDown={(event) => {
                  if (event.key === "Escape") editor.cancelName();
                  if (event.key === "Enter" && editor.nickname.trim().length >= 2)
                    editor.saveName();
                }}
                autoFocus
                className="max-w-sm flex-1 text-heading-lg font-strong"
              />
            ) : (
              <h1 className="truncate text-display-md font-strong text-content-primary">
                {profile.displayName}
              </h1>
            )}
            <PlayerBadge badge={profile.selectedBadge} size="md" />
            {owner ? (
              editor.editingName ? (
                <>
                  <Button
                    variant="icon"
                    size="icon"
                    aria-label="Save display name"
                    disabled={
                      editor.nickname.trim().length < 2 ||
                      editor.nicknameMutation.isPending
                    }
                    onClick={editor.saveName}
                    className="h-9 min-h-9 w-9"
                  >
                    {editor.nicknameMutation.isPending ? (
                      <Spinner size="sm" label="Saving display name" color="current" />
                    ) : (
                      <Check className="h-4 w-4" />
                    )}
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label="Cancel display name"
                    onClick={editor.cancelName}
                    className="h-9 min-h-9 w-9"
                  >
                    <X className="h-4 w-4" />
                  </Button>
                </>
              ) : (
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label="Edit display name"
                  onClick={() => editor.setEditingName(true)}
                  className="h-9 min-h-9 w-9"
                >
                  <Pencil className="h-4 w-4" />
                </Button>
              )
            ) : null}
          </div>
          <div className="mt-3 flex flex-wrap items-center gap-2">
            <MmrDisplay value={profile.mmr} size="md" label />
            {profile.leaderboardRank > 0 ? (
              <Badge
                tone="neutral"
                size="md"
                aria-label={`Ranked #${profile.leaderboardRank} of ${profile.leaderboardTotal}`}
                title={`Ranked #${profile.leaderboardRank.toLocaleString()} of ${profile.leaderboardTotal.toLocaleString()}`}
              >
                #{profile.leaderboardRank.toLocaleString()}
                <span className="ml-1.5 text-label">
                  ranked
                </span>
              </Badge>
            ) : null}
          </div>
          <MutationError
            mutation={editor.nicknameMutation}
            fallback="Failed to update display name"
          />
        </div>
        <div className="flex shrink-0 flex-wrap gap-2">
          {socialActions}
        </div>
      </div>
      <div className="mt-6 grid grid-cols-2 gap-2.5 border-t border-border-default pt-5 sm:grid-cols-3 lg:grid-cols-5">
        {stats.map(([icon, label, value], index) => (
          <IconMetric
            key={label}
            icon={icon}
            label={label}
            value={value}
            className={index === 4 ? "col-span-2 sm:col-span-1" : undefined}
          />
        ))}
      </div>
    </AppPanel>
  );
}

export function ProfileBadges({
  profile,
  editor,
  owner,
}: {
  profile: PublicPlayerProfile;
  editor: Editor;
  owner: boolean;
}) {
  return (
    <AppPanel className="rounded-2xl p-4 sm:p-6">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-heading-sm font-strong text-content-primary">Earned badges</h2>
        {owner ? (
          editor.choosingBadge ? (
            <div className="flex gap-2">
              <Button variant="ghost" size="sm" onClick={editor.cancelBadge}>
                Cancel
              </Button>
              <Button
                variant="primary"
                size="sm"
                onClick={editor.saveBadge}
                disabled={editor.badgeMutation.isPending}
              >
                {editor.badgeMutation.isPending ? (
                  <Spinner size="sm" label="Saving badge" color="current" />
                ) : null}
                Save
              </Button>
            </div>
          ) : (
            <Button
              onClick={() => editor.setChoosingBadge(true)}
            >
              Choose displayed badge
            </Button>
          )
        ) : null}
      </div>
      <MutationError
        mutation={editor.badgeMutation}
        fallback="Failed to update displayed badge"
        className="mb-3"
      />
      <ProfileBadgeCollection
        badges={profile.badges || []}
        editing={editor.choosingBadge}
        selectedBadgeId={
          editor.choosingBadge ? editor.badgeId : profile.selectedBadge?.id
        }
        onSelect={editor.setBadgeId}
      />
    </AppPanel>
  );
}

export function ProfileHistory({
  matches,
  query,
  filter,
  onFilterChange,
}: {
  matches: PlayerMatchSummary[];
  filter: "all" | "ranked";
  onFilterChange: (filter: "all" | "ranked") => void;
  query: {
    isLoading: boolean;
    isError: boolean;
    hasNextPage: boolean;
    isFetchingNextPage: boolean;
    fetchNextPage: () => unknown;
  };
}) {
  return (
    <AppPanel className="rounded-2xl p-4 sm:p-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <h2 className="text-heading-md font-strong text-content-primary">Match history</h2>
        <div className="flex flex-wrap items-center gap-3">
          <Tabs
            value={filter}
            onChange={onFilterChange}
            items={[
              { id: "all", label: "All" },
              { id: "ranked", label: "Ranked" },
            ]}
            className="grid-cols-2"
          />
          {matches.length ? (
            <span className="text-caption font-strong text-content-secondary">
              {matches.length} shown
            </span>
          ) : null}
        </div>
      </div>
      {query.isLoading ? <CenteredSpinner label="Loading match history" /> : null}
      {query.isError ? (
        <p className="mt-4 text-body-sm text-status-danger">
          Match history is temporarily unavailable.
        </p>
      ) : null}
      {!query.isLoading && !query.isError && !matches.length ? (
        <p className="mt-4 text-body-sm text-content-secondary">
          {filter === "ranked"
            ? "No ranked matches yet."
            : "No match history yet."}
        </p>
      ) : null}
      <div className="mt-4 space-y-2">
        {matches.map((match) => (
          <MatchHistoryRow key={match.matchId} match={match} />
        ))}
      </div>
      {query.hasNextPage ? (
        <Button
          onClick={() => void query.fetchNextPage()}
          disabled={query.isFetchingNextPage}
          className="mt-4 w-full rounded-xl"
        >
          {query.isFetchingNextPage ? "Loading…" : "Load more"}
        </Button>
      ) : null}
    </AppPanel>
  );
}

function MutationError({
  mutation,
  fallback,
  className = "mt-2",
}: {
  mutation: { isError: boolean; error: unknown };
  fallback: string;
  className?: string;
}) {
  if (!mutation.isError) return null;
  return (
    <p className={`${className} text-body-sm font-semibold text-status-danger`}>
      {mutation.error instanceof Error ? mutation.error.message : fallback}
    </p>
  );
}
