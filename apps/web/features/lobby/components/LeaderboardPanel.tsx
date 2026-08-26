import type { LeaderboardSummary } from "../../auth/controllers/session-controller";
import PlayerProfileLink from "../../players/components/PlayerProfileLink";
import { useSeasonResetCountdown } from "../lib/season-countdown";
import { AppPanel } from "../../../components/ui/compositions";
import { Badge } from "../../../components/ui/Badge";
import { LobbySectionHeader } from "./lobby-primitives";
import { CenteredSpinner } from "../../../components/ui/Spinner";

export function LeaderboardPanel({
  leaderboard,
  leaderboardLoading,
  mmr,
  userId,
}: {
  leaderboard: LeaderboardSummary | null;
  leaderboardLoading: boolean;
  mmr: number;
  userId: string;
}) {
  const resetCountdown = useSeasonResetCountdown(leaderboard?.nextResetAt);

  return (
    <AppPanel className="flex w-full max-w-[980px] flex-col gap-5 rounded-2xl p-5 sm:p-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <LobbySectionHeader
          eyebrow={resetCountdown}
          title="Leaderboard"
          description={
            leaderboardLoading
              ? "Loading ranked players..."
              : leaderboard
                ? `${formatSeasonName(leaderboard.season)}, ${leaderboard.totalPlayers} players`
                : "No ranked players yet."
          }
        />
        <div className="grid grid-cols-2 gap-3 sm:min-w-[240px]">
          <LeaderboardMetric label="Your Rank" value={leaderboard?.selfRank ? `#${leaderboard.selfRank}` : "--"} />
          <LeaderboardMetric label="Rating" value={mmr.toLocaleString()} />
        </div>
      </div>

      <div className="overflow-hidden rounded-xl">
        <div className="grid grid-cols-[72px_minmax(0,1fr)_90px] gap-3 border-b border-border-default px-4 py-3 text-label font-strong text-content-secondary sm:grid-cols-[72px_minmax(0,1fr)_110px_110px]">
          <span>Rank</span>
          <span>Player</span>
          <span className="text-right">MMR</span>
          <span className="hidden text-right sm:block">Win Rate</span>
        </div>
        <div className="divide-y divide-border-default">
          {(leaderboard?.entries || []).map((entry) => {
            const isSelf = entry.userId === userId;
            const winsValue =
              entry.gamesPlayed > 0 ? Math.round((entry.wins / entry.gamesPlayed) * 100) : 0;
            return (
              <div
                key={`${entry.rank}-${entry.userId}`}
                className={`grid grid-cols-[72px_minmax(0,1fr)_90px] gap-3 px-4 py-3 text-body-sm sm:grid-cols-[72px_minmax(0,1fr)_110px_110px] ${
                  isSelf ? "bg-status-success/10" : "bg-transparent"
                }`}
              >
                <div className="flex items-center">
                  <Badge tone={entry.rank <= 3 ? "success" : "neutral"} className="min-w-[48px] justify-center text-label">
                    #{entry.rank}
                  </Badge>
                </div>
                <div className="min-w-0">
                  <PlayerProfileLink userId={entry.userId} nickname={entry.displayName} className="block truncate font-strong text-content-primary hover:text-status-success">
                    {entry.displayName || entry.userId}
                  </PlayerProfileLink>
                  <p className="truncate text-caption text-content-secondary">
                    {isSelf ? "You" : `${entry.gamesPlayed} games`}
                  </p>
                </div>
                <div className="flex items-center justify-end font-strong text-content-primary">{entry.mmr}</div>
                <div className="hidden items-center justify-end text-content-secondary sm:flex">{winsValue}%</div>
              </div>
            );
          })}
          {leaderboardLoading ? (
            <CenteredSpinner label="Loading leaderboard" />
          ) : !leaderboard || leaderboard.entries.length === 0 ? (
            <LeaderboardEmpty>No ranked players yet.</LeaderboardEmpty>
          ) : null}
        </div>
      </div>
    </AppPanel>
  );
}

function LeaderboardMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl bg-surface-inset p-4">
      <p className="text-label font-strong text-content-secondary">{label}</p>
      <p className="mt-2 text-display-md font-strong text-content-primary">{value}</p>
    </div>
  );
}

function LeaderboardEmpty({ children }: { children: string }) {
  return <div className="px-4 py-10 text-center text-body-sm text-content-secondary">{children}</div>;
}

export function formatSeasonName(season: string) {
  const number = season.trim().replace(/^s/i, "");
  return number ? `Season ${number}` : "Season";
}
