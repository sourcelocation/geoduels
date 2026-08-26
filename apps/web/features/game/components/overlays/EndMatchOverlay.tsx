import dynamic from "next/dynamic";
import { motion, AnimatePresence } from "framer-motion";
import { Flag, RotateCcw, List, LogOut } from "lucide-react";
import { useState, useMemo } from "react";
import {
  ParticipantIdentityCard,
  ParticipantIdentityRow,
  type MatchSideView,
  type MatchSidesView,
  type PlayerIdentityView,
} from "./ParticipantIdentity";
import { AppPanel } from "../../../../components/ui/compositions";
import { Button, IconButton } from "../../../../components/ui/button";
import { Heading } from "../../../../components/ui/typography";
import type { RoundResult } from "../../model/types";
import { getHealthFillStyle } from "../health-bar";
import ReportPlayerDialog from "./ReportPlayerDialog";

const GuessMap = dynamic(() => import("../../../../components/GuessMap"), { ssr: false });

type Props = {
  onLeaveGame: () => void;
  onPlayAgain?: () => Promise<string> | void;
  backLabel?: string;
  mode: EndMatchMode;
  outcome?: "win" | "lose" | "draw";
  sides: MatchSidesView;
  selfUserId: string;
  totalScore: number;
  maxHP: number;
  roundResults: RoundResult[];
  resultPlayerNames: Record<string, string | undefined>;
  resultPlayerAvatars: Record<string, string | undefined>;
  resultPlayerFallbacks: Record<string, string | undefined>;
  resultPlayerBorderColors?: Record<string, string | undefined>;
  participantsById?: Record<string, PlayerIdentityView>;
  onReportPlayer?: (
    reportedUserId: string,
    category?: string,
    reason?: string,
  ) => Promise<void> | void;
  asPage?: boolean;
};

function formatGuessTime(ms?: number) {
  if (typeof ms !== "number" || !Number.isFinite(ms) || ms <= 0) return "";
  return `${(ms / 1000).toFixed(1)}s`;
}

type EndMatchMode = "duel" | "singleplayer" | "team_duel" | "free_for_all";

export default function EndMatchOverlay({
  onLeaveGame,
  mode,
  outcome,
  sides,
  selfUserId,
  totalScore,
  maxHP,
  roundResults,
  resultPlayerNames,
  resultPlayerAvatars,
  resultPlayerFallbacks,
  resultPlayerBorderColors,
  participantsById = {},
  onReportPlayer,
  onPlayAgain,
  backLabel = "Back to lobby",
  asPage = false,
}: Props) {
  const [reportedUserIds, setReportedUserIds] = useState<Record<string, boolean>>({});
  const [reportBusyUserId, setReportBusyUserId] = useState("");
  const [reportError, setReportError] = useState("");
  const [reportCategory, setReportCategory] = useState("cheating");
  const [reportReason, setReportReason] = useState("");
  const [pendingReport, setPendingReport] = useState<{ userId: string; name: string; } | null>(null);
  const [playAgainBusy, setPlayAgainBusy] = useState(false);
  const [showBreakdown, setShowBreakdown] = useState(false);
  const [duelBreakdownMode, setDuelBreakdownMode] = useState<"health" | "points">("health");

  const totalRounds = roundResults.length;
  const hasRoundResults = totalRounds > 0;
  const playerIds = Object.keys(roundResults[0]?.players || {});
  const selfPlayerId = selfUserId || playerIds[0] || "self";
  const showPlayAgain = mode === "singleplayer" && !!onPlayAgain;

  const isDuelsMode = mode === 'duel' || mode === 'team_duel';
  const isFFAMode = mode === 'free_for_all';
  const isSPMode = mode === 'singleplayer';
  const playerScores = useMemo(() => {
    const totals: Record<string, { score: number; time: number; distance: number }> = {};
    for (const round of roundResults) {
      for (const [pid, p] of Object.entries(round.players)) {
        if (!totals[pid]) totals[pid] = { score: 0, time: 0, distance: 0 };
        totals[pid].score += p.score;
        totals[pid].time += p.guessMs || 0;
        totals[pid].distance += p.distanceKm;
      }
    }
    const arr = Object.entries(totals).map(([id, stats]) => ({
      id,
      name: resultPlayerNames[id] || 'Unknown',
      avatar: resultPlayerAvatars[id],
      fallback: resultPlayerFallbacks[id] || '?',
      participant: participantsById[id] || {
        kind: "player" as const,
        id,
        name: resultPlayerNames[id] || 'Unknown',
        avatarUrl: resultPlayerAvatars[id],
        avatarFallback: resultPlayerFallbacks[id] || '?',
      },
      ...stats
    }));
    arr.sort((a, b) => b.score - a.score);
    return arr;
  }, [roundResults, resultPlayerNames, resultPlayerAvatars, resultPlayerFallbacks]);

  const ffaWinner = playerScores.length > 0 ? playerScores[0] : null;
  const myIndex = playerScores.findIndex(p => p.id === selfPlayerId);
  const myPlacement = myIndex >= 0 ? myIndex + 1 : 0;
  const placementText = myPlacement === 1 ? '1st' : myPlacement === 2 ? '2nd' : myPlacement === 3 ? '3rd' : `${myPlacement}th`;
  const outcomeLabel =
    outcome === "win" ? "Win" : outcome === "lose" ? "Defeat" : "Draw";

  async function handlePlayAgain() {
    if (!onPlayAgain || playAgainBusy) return;
    setPlayAgainBusy(true);
    try {
      await onPlayAgain();
    } finally {
      setPlayAgainBusy(false);
    }
  }

  function renderScoreCell(round: RoundResult, playerId: string | undefined, highlight = false) {
    if (!playerId) return <span className="text-content-secondary">-</span>;
    const player = round.players[playerId];
    if (!player) return <span className="text-content-secondary">-</span>;
    const guessTime = formatGuessTime(player.guessMs);
    return (
      <div className={`flex items-baseline gap-2 ${highlight ? "font-strong text-content-primary" : "font-strong text-content-secondary"}`}>
        <span>{player.score.toLocaleString()}</span>
        {guessTime ? (
          <span className="text-caption font-strong tracking-display text-content-secondary">
            {guessTime}
          </span>
        ) : null}
      </div>
    );
  }
  function renderHealthCell(
    round: RoundResult,
    side: MatchSideView,
    highlight = false,
  ) {
    const hp =
      side.participant.kind === "team"
        ? round.teams?.[side.id]?.hpAfterRound
        : round.players[side.id]?.hpAfterRound;
    if (hp === undefined) return <span className="text-content-secondary">-</span>;
    const guessPlayerId =
      side.participant.kind === "team"
        ? round.teams?.[side.id]?.representativeUserId
        : side.id;
    const guessTime = formatGuessTime(
      guessPlayerId ? round.players[guessPlayerId]?.guessMs : undefined,
    );
    const healthPercentage =
      maxHP > 0 ? Math.max(0, Math.min(100, (hp / maxHP) * 100)) : 0;
    return (
      <div className={`flex min-w-[210px] items-center gap-3 ${highlight ? "font-strong text-content-primary" : "font-strong text-content-secondary"}`}>
        <div className="flex min-w-[104px] items-baseline gap-2">
          <span>{hp.toLocaleString()} HP</span>
          {guessTime ? (
            <span className="text-caption font-strong tracking-display text-content-secondary">
              {guessTime}
            </span>
          ) : null}
        </div>
        <div
          role="progressbar"
          aria-label={`${side.participant.name} health`}
          aria-valuemin={0}
          aria-valuemax={maxHP}
          aria-valuenow={hp}
          className="h-3 w-24 shrink-0 overflow-hidden rounded-full bg-surface-fill"
        >
          <div
            className="h-full rounded-full transition-[width] duration-dramatic"
            style={{
              width: `${healthPercentage}%`,
              ...getHealthFillStyle(healthPercentage),
            }}
          />
        </div>
      </div>
    );
  }

  function renderSideScoreCell(
    round: RoundResult,
    side: MatchSideView,
    highlight = false,
  ) {
    const result =
      side.participant.kind === "team"
        ? round.teams?.[side.id]
        : round.players[side.id];
    if (!result) return <span className="text-content-secondary">-</span>;
    const guessPlayerId =
      side.participant.kind === "team"
        ? round.teams?.[side.id]?.representativeUserId
        : side.id;
    const guessTime = formatGuessTime(
      guessPlayerId ? round.players[guessPlayerId]?.guessMs : undefined,
    );
    return (
      <div className={`flex items-baseline gap-2 ${highlight ? "font-strong text-content-primary" : "font-strong text-content-secondary"}`}>
        <span>{result.score.toLocaleString()} pts</span>
        {guessTime ? (
          <span className="text-caption font-strong tracking-display text-content-secondary">
            {guessTime}
          </span>
        ) : null}
      </div>
    );
  }

  function renderSideCard(side: MatchSideView, opponent = false) {
    const reportUserId =
      opponent && side.participant.kind === "player"
        ? side.participant.id
        : undefined;
    const canReport = !!reportUserId && !!onReportPlayer && !reportedUserIds[reportUserId];
    const reportButton = reportUserId && onReportPlayer ? (
      <IconButton
        size="icon-sm"
        type="button"
        aria-label={reportedUserIds[reportUserId] ? "Report sent" : "Report player"}
        title={reportedUserIds[reportUserId] ? "Report sent" : "Report player"}
        disabled={!canReport || reportBusyUserId === reportUserId}
        onClick={() => {
          setReportError("");
          setReportCategory("cheating");
          setReportReason("");
          setPendingReport({ userId: reportUserId, name: side.participant.name });
        }}
        className="flex h-7 w-7 items-center justify-center rounded-full border border-status-danger/35 bg-status-danger/15 text-status-danger transition hover:bg-status-danger/25 disabled:cursor-not-allowed disabled:opacity-50"
      >
        <Flag size={13} />
      </IconButton>
    ) : null;
    return (
      <AppPanel className={`flex flex-col items-center gap-3 rounded-xl p-6 text-center ${opponent ? 'bg-status-danger/5 border-status-danger/15' : 'bg-brand-blue/5 border-brand-blue/15'}`}>
        <div className="flex flex-col items-center">
          <ParticipantIdentityCard
            participant={side.participant}
            opponent={opponent}
            ratingAction={reportButton}
            showRatingPreview={false}
            wrapRatingDelta={false}
          />
        </div>
      </AppPanel>
    );
  }

  const breakdownTable = (
    <div className="mt-2">
      <AppPanel className="rounded-xl p-1">
        {isDuelsMode ? (
          <div className="w-full overflow-x-auto">
            <div className="flex items-center justify-end gap-3 px-4 py-3">
              <div
                role="group"
                aria-label="Breakdown preview mode"
                className="inline-flex rounded-full border border-border-default bg-surface-inset p-1"
              >
                {(["health", "points"] as const).map((previewMode) => (
                  <Button
                    variant="ghost"
                    key={previewMode}
                    type="button"
                    aria-pressed={duelBreakdownMode === previewMode}
                    onClick={() => setDuelBreakdownMode(previewMode)}
                    className={`rounded-full px-3 py-1.5 text-label font-strong uppercase tracking-label transition ${
                      duelBreakdownMode === previewMode
                        ? "bg-surface-fill text-content-primary"
                        : "text-content-secondary hover:text-content-primary"
                    }`}
                  >
                    {previewMode}
                  </Button>
                ))}
              </div>
            </div>
            <table className="w-full text-left text-body-sm text-content-primary">
              <thead>
                <tr className="border-b border-border-default text-content-secondary">
                  <th className="py-3 px-4 font-strong uppercase tracking-wide">Round</th>
                  <th className="py-3 px-4 font-strong uppercase tracking-wide">{sides.self.participant.name}</th>
                  <th className="py-3 px-4 font-strong uppercase tracking-wide">{sides.opponent.participant.name}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border-default">
                {roundResults.map((round) => (
                  <tr key={round.roundId} className="hover:bg-surface-fill">
                    <td className="py-3 px-4 font-strong text-content-primary">R{round.roundNumber}</td>
                    <td className="py-3 px-4">
                      {duelBreakdownMode === "health"
                        ? renderHealthCell(round, sides.self, true)
                        : renderSideScoreCell(round, sides.self, true)}
                    </td>
                    <td className="py-3 px-4">
                      {duelBreakdownMode === "health"
                        ? renderHealthCell(round, sides.opponent)
                        : renderSideScoreCell(round, sides.opponent)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="w-full overflow-x-auto">
            <table className="w-full text-left text-body-sm text-content-primary whitespace-nowrap">
              <thead>
                <tr className="border-b border-border-default text-content-secondary">
                  <th className="py-3 px-4 font-strong uppercase tracking-wide">Player</th>
                  {roundResults.map(r => (
                    <th key={r.roundId} className="py-3 px-4 font-strong uppercase tracking-wide">R{r.roundNumber}</th>
                  ))}
                  <th className="py-3 px-4 font-strong uppercase tracking-wide text-right">Total</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border-default">
                {playerScores.map(player => (
                  <tr key={player.id} className={player.id === selfPlayerId ? 'bg-surface-fill' : 'hover:bg-surface-fill'}>
                    <td className="py-3 px-4">
                      <div className="flex items-center gap-3">
                        <ParticipantIdentityRow
                          participant={player.participant}
                          nameClassName={player.id === selfPlayerId ? 'font-strong text-brand-blue' : 'font-strong text-content-primary'}
                        />
                      </div>
                    </td>
                    {roundResults.map(r => (
                      <td key={r.roundId} className="py-3 px-4">
                        {renderScoreCell(r, player.id, player.id === selfPlayerId)}
                      </td>
                    ))}
                    <td className="py-3 px-4 text-right font-strong text-status-success">{player.score.toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </AppPanel>
    </div>
  );
  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      className={`flex flex-col ${asPage ? "app-layer-match-end fixed inset-0 bg-surface-page" : "absolute inset-0 z-game bg-surface-page/90"}`}
    >
      <div className="flex-1 min-h-0 w-full relative">
        {hasRoundResults ? (
          <GuessMap
            mode="result"
            results={roundResults}
            interactiveInResult
            resultPlayerAvatars={resultPlayerAvatars}
            resultPlayerFallbacks={resultPlayerFallbacks}
            resultPlayerBorderColors={resultPlayerBorderColors}
          />
        ) : (
          <div className="flex h-full items-center justify-center bg-scrim text-center text-body-sm font-semibold text-content-secondary">
            The match ended before any round results were recorded.
          </div>
        )}
      </div>

      <div className="shrink-0 bg-surface-page border-t border-border-default p-6 md:p-8 shadow-elev-4 relative z-content max-h-[50vh] overflow-y-auto">
        <div className="mx-auto max-w-6xl">
          {reportError && <p className="mb-4 text-center text-body-sm font-strong text-status-danger">{reportError}</p>}

          {showBreakdown ? (
            <div className="w-full flex flex-col">
              <div className="mb-6 flex items-center justify-between">
                <h2 className="font-display text-heading-md font-strong uppercase tracking-display text-content-primary">Match Breakdown</h2>
                <Button
                  variant="secondary"
                  type="button"
                  onClick={() => setShowBreakdown(false)}
                  className="inline-flex items-center rounded-full bg-surface-fill px-6 py-2.5 text-body-sm font-strong uppercase tracking-display text-content-primary transition hover:bg-surface-raised hover:scale-105 active:scale-95"
                >
                  Back
                </Button>
              </div>
              {breakdownTable}
            </div>
          ) : (
            <div>
              <h2 className="sr-only">Match Complete</h2>
              {isDuelsMode ? (
                <div className="flex flex-col md:flex-row items-center justify-center gap-6 md:gap-12">
                  <div className="flex-1 max-w-[320px] w-full">
                    {renderSideCard(sides.self)}
                  </div>
                  <div className="flex flex-col items-center gap-4">
                    <Heading as="p" variant="display-md" className={`sm:text-display-lg ${
                      outcome === "lose" ? "text-status-danger" : outcome === "draw" ? "text-content-secondary" : "text-status-success"
                    }`}>
                      {outcomeLabel}
                    </Heading>
                    <Button
                      variant="primary"
                      type="button"
                      className="group relative overflow-hidden rounded-full px-10 py-4 text-body-sm font-strong uppercase tracking-control-wide text-content-on-action bg-action-primary shadow-elev-2 hover:bg-action-primary-hover transition-all hover:scale-105 active:scale-95"
                      onClick={onLeaveGame}
                    >
                      <span className="relative z-content">{backLabel}</span>
                      <div className="absolute inset-0 bg-content-on-action opacity-0 transition-opacity group-hover:opacity-100" />
                    </Button>
                    <Button
                      variant="secondary"
                      type="button"
                      className="inline-flex h-12 items-center justify-center gap-2 rounded-full px-6 transition hover:scale-105 active:scale-95"
                      onClick={() => setShowBreakdown(true)}
                      title="Toggle Breakdown"
                    >
                      <List size={20} />
          <span className="text-body-sm font-strong uppercase tracking-display">Breakdown</span>
                    </Button>
                  </div>
                  <div className="flex-1 max-w-[320px] w-full">
                    {renderSideCard(sides.opponent, true)}
                  </div>
                </div>
              ) : (
                <div className="flex flex-col items-center text-center">
                  {isFFAMode && ffaWinner && (
                    <p className="mb-2 text-body-sm font-strong uppercase tracking-display-wide text-status-success">
                      {ffaWinner.id === selfPlayerId ? 'You won!' : `${ffaWinner.name} won!`}
                    </p>
                  )}
                  {isFFAMode && (
                    <p className="mb-4 text-body-sm font-semibold uppercase tracking-control-wide text-content-secondary">
                      You placed <span className="text-content-primary">{placementText}</span>
                    </p>
                  )}
                  <h2 className="font-display text-display-lg font-strong text-content-primary drop-shadow-md">
                    {totalScore.toLocaleString()} <span className="text-heading-sm text-brand-blue">pts</span>
                  </h2>

                  <div className="mt-8 flex items-center justify-center gap-4">
                    <Button
                      variant="secondary"
                      type="button"
                      className="inline-flex h-12 items-center justify-center gap-2 rounded-full px-6 transition hover:scale-105 active:scale-95"
                      onClick={() => setShowBreakdown(true)}
                      title="Toggle Breakdown"
                    >
                      <List size={20} />
          <span className="text-body-sm font-strong uppercase tracking-display">Breakdown</span>
                    </Button>
                    <Button
                      variant="primary"
                      type="button"
                      className="group relative overflow-hidden rounded-full px-10 py-4 text-body-sm font-strong uppercase tracking-control-wide text-content-on-action bg-action-primary shadow-elev-2 hover:bg-action-primary-hover transition-all hover:scale-105 active:scale-95"
                      onClick={onLeaveGame}
                    >
                      <span className="relative z-content">{backLabel}</span>
                      <div className="absolute inset-0 bg-content-on-action opacity-0 transition-opacity group-hover:opacity-100" />
                    </Button>
                    {showPlayAgain && (
                      <Button
                        variant="secondary"
                        type="button"
                        className="flex h-12 w-12 shrink-0 items-center justify-center rounded-full bg-surface-fill text-content-primary transition hover:bg-surface-raised hover:scale-105 active:scale-95"
                        onClick={() => void handlePlayAgain()}
                        disabled={playAgainBusy}
                        aria-label="Play again"
                        title="Play Again"
                      >
                        <RotateCcw size={20} className={playAgainBusy ? "animate-spin" : ""} />
                      </Button>
                    )}
                    <Button
                      variant="danger"
                      type="button"
                      className="flex h-12 w-12 shrink-0 items-center justify-center rounded-full border border-status-danger/30 bg-status-danger/15 text-status-danger transition hover:bg-status-danger/25 hover:scale-105 active:scale-95"
                      onClick={onLeaveGame}
                      aria-label="Exit match"
                      title="Exit"
                    >
                      <LogOut size={20} />
                    </Button>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {pendingReport ? <ReportPlayerDialog pendingReport={pendingReport} onClose={() => { setPendingReport(null); setReportReason(""); }} onReportPlayer={onReportPlayer} onReported={(userId) => setReportedUserIds((current) => ({ ...current, [userId]: true }))} reportBusyUserId={reportBusyUserId} setReportBusyUserId={setReportBusyUserId} reportCategory={reportCategory} setReportCategory={setReportCategory} reportReason={reportReason} setReportReason={setReportReason} reportError={reportError} setReportError={setReportError} /> : null}
    </motion.div>
  );
}
