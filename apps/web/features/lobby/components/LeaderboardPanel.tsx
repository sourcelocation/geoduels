import type { LeaderboardSummary } from "../../auth/controllers/session-controller";
import PlayerProfileLink from "../../../components/ui/PlayerProfileLink";
import { useSeasonResetCountdown } from "../lib/season-countdown";
import { LobbyPanel, LobbySectionHeader } from "./lobby-primitives";

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
    <LobbyPanel radius="3xl" className="flex w-full max-w-[980px] flex-col gap-5 p-5 sm:p-6">
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

      <div className="overflow-hidden rounded-[20px]">
        <div className="grid grid-cols-[72px_minmax(0,1fr)_90px] gap-3 border-b border-white/[0.06] px-4 py-3 text-[11px] font-bold uppercase tracking-[0.14em] text-[#6b8b80] sm:grid-cols-[72px_minmax(0,1fr)_110px_110px]">
          <span>Rank</span>
          <span>Player</span>
          <span className="text-right">MMR</span>
          <span className="hidden text-right sm:block">Win Rate</span>
        </div>
        <div className="divide-y divide-white/[0.06]">
          {(leaderboard?.entries || []).map((entry) => {
            const isSelf = entry.userId === userId;
            const winsValue =
              entry.gamesPlayed > 0 ? Math.round((entry.wins / entry.gamesPlayed) * 100) : 0;
            return (
              <div
                key={`${entry.rank}-${entry.userId}`}
                className={`grid grid-cols-[72px_minmax(0,1fr)_90px] gap-3 px-4 py-3 text-sm sm:grid-cols-[72px_minmax(0,1fr)_110px_110px] ${
                  isSelf ? "bg-[#18382e]/70" : "bg-transparent"
                }`}
              >
                <div className="flex items-center">
                  <span
                    className={`inline-flex min-w-[48px] items-center justify-center rounded-full px-3 py-1 text-[12px] font-black ${
                      entry.rank <= 3 ? "bg-[#2ad18f]/16 text-[#77f0be]" : "bg-white/[0.05] text-white"
                    }`}
                  >
                    #{entry.rank}
                  </span>
                </div>
                <div className="min-w-0">
                  <PlayerProfileLink userId={entry.userId} nickname={entry.displayName} className="block truncate font-bold text-white hover:text-emerald-200">
                    {entry.displayName || entry.userId}
                  </PlayerProfileLink>
                  <p className="truncate text-[12px] text-[#8caab0]">
                    {isSelf ? "You" : `${entry.gamesPlayed} games`}
                  </p>
                </div>
                <div className="flex items-center justify-end font-black text-white">{entry.mmr}</div>
                <div className="hidden items-center justify-end text-[#a9bfd4] sm:flex">{winsValue}%</div>
              </div>
            );
          })}
          {leaderboardLoading ? (
            <LeaderboardEmpty>Loading leaderboard...</LeaderboardEmpty>
          ) : !leaderboard || leaderboard.entries.length === 0 ? (
            <LeaderboardEmpty>No ranked players yet.</LeaderboardEmpty>
          ) : null}
        </div>
      </div>
    </LobbyPanel>
  );
}

function LeaderboardMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl bg-black/30 p-4">
      <p className="text-[11px] font-bold uppercase tracking-[0.14em] text-[#6b8b80]">{label}</p>
      <p className="mt-2 text-3xl font-black text-white">{value}</p>
    </div>
  );
}

function LeaderboardEmpty({ children }: { children: string }) {
  return <div className="px-4 py-10 text-center text-[14px] text-[#a9bfd4]">{children}</div>;
}

export function formatSeasonName(season: string) {
  const number = season.trim().replace(/^s/i, "");
  return number ? `Season ${number}` : "Season";
}
