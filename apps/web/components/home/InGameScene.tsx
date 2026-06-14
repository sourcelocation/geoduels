import { AnimatePresence, motion } from 'framer-motion';
import { AlertTriangle, LogOut, RotateCcw, X } from 'lucide-react';
import { useCallback, useEffect, useRef, useState, useMemo, type ReactNode } from 'react';
import GameHUD from '../ui/GameHUD';
import MinimapPanel from '../ui/MinimapPanel';
import PlayerHPCard from '../ui/PlayerHPCard';
import RoundResultOverlay from '../ui/RoundResultOverlay';
import GameStartOverlay from '../ui/GameStartOverlay';
import DuelOverlayBackground from '../ui/DuelOverlayBackground';
import IntroCountdownText from '../ui/IntroCountdownText';
import { PlayerIdentityRow } from '../ui/PlayerIdentity';
import { motionPresetClass } from '../ui/motion';
import { ResultDistanceBar } from '../ui/RoundResultOverlay';
import type { RatingDeltaPreview, RoundResultOverlayProps, UIPhase } from '../ui/types';
import type { PlayerBadgeInfo } from '../ui/PlayerBadge';
import type { ParticipantIdentityView } from '../ui/PlayerIdentity';

export type InGameSceneProps = {
  uiPhase: UIPhase;
  streetViewSrc: string;
  streetViewInteractive: boolean;
  showResultStage: boolean;
  isSingleplayer: boolean;
  isPointsMode: boolean;
  partyMode?: "duel" | "team_duel" | "free_for_all";
  resultOverlay?: RoundResultOverlayProps;
  selfName: string;
  selfAvatarUrl?: string;
  selfFallback: string;
  selfAvatarColor?: string;
  selfIsAdmin: boolean;
  selfSelectedBadge?: PlayerBadgeInfo | null;
  opponentName: string;
  opponentIsAdmin: boolean;
  opponentSelectedBadge?: PlayerBadgeInfo | null;
  opponentDisconnected: boolean;
  oppAvatarUrl?: string;
  oppFallback: string;
  oppAvatarColor?: string;
  hpPct: (hp: number) => string;
  mm: string;
  ss: string;
  isRoundTimerRunning: boolean;
  timerProgressPct: number;
  isTimerCritical: boolean;
  isTimerPulseActive: boolean;
  resultMode: boolean;
  selfHP: number;
  oppHP: number;
  totalScore: number;
  currentRoundScore: number;
  currentRoundDistanceKm: number;
  onForfeit: () => boolean;
  onAdvanceRound: () => boolean;
  onLeaveGame: () => void;
  canFinalizeGuess: boolean;
  canAdvanceRound: boolean;
  onFinalizeGuess: () => void;
  guessMapNode: ReactNode;
  resultMapNode?: ReactNode;
  roundResults?: import('../ui/types').RoundResult[];
  resultPlayerNames?: Record<string, string | undefined>;
  resultPlayerAvatars?: Record<string, string | undefined>;
  resultPlayerFallbacks?: Record<string, string | undefined>;
  participantsById?: Record<string, ParticipantIdentityView>;
  selfElo: number;
  opponentElo: number;
  selfRatingPreview?: RatingDeltaPreview;
  damageMultiplier: number;
  guessSubmitted: boolean;
  opponentGuessAlert: boolean;
  connectionIssue: string;
  roundNumber?: number;
  totalRounds?: number;
  modeName?: string;
  mapName?: string;
  selfUserId: string;
};

export default function InGameScene({
  uiPhase,
  streetViewSrc,
  streetViewInteractive,
  showResultStage,
  isSingleplayer,
  isPointsMode,
  partyMode = "duel",
  resultOverlay,
  selfName,
  selfAvatarUrl,
  selfFallback,
  selfAvatarColor,
  selfIsAdmin,
  selfSelectedBadge,
  opponentName,
  opponentIsAdmin,
  opponentSelectedBadge,
  opponentDisconnected,
  oppAvatarUrl,
  oppFallback,
  oppAvatarColor,
  hpPct,
  mm,
  ss,
  isRoundTimerRunning,
  timerProgressPct,
  isTimerCritical,
  isTimerPulseActive,
  resultMode,
  selfHP,
  oppHP,
  totalScore,
  currentRoundScore,
  currentRoundDistanceKm,
  onForfeit,
  onAdvanceRound,
  onLeaveGame,
  canFinalizeGuess,
  canAdvanceRound,
  onFinalizeGuess,
  guessMapNode,
  resultMapNode,
  roundResults = [],
  resultPlayerNames = {},
  resultPlayerAvatars = {},
  resultPlayerFallbacks = {},
  participantsById = {},
  selfElo,
  opponentElo,
  selfRatingPreview,
  damageMultiplier,
  guessSubmitted,
  opponentGuessAlert,
  connectionIssue,
  roundNumber = 1,
  totalRounds,
  modeName = 'Moving',
  mapName = 'A Source World',
  selfUserId
}: InGameSceneProps) {
  const showGuessAlertBorder = opponentGuessAlert;
  const [confirmForfeit, setConfirmForfeit] = useState(false);
  const [forfeitRequested, setForfeitRequested] = useState(false);
  const [streetViewResetCount, setStreetViewResetCount] = useState(0);
  const sceneRef = useRef<HTMLElement | null>(null);
  const streetViewFrameRef = useRef<HTMLIFrameElement | null>(null);
  const canShowForfeit = uiPhase !== 'match_end';
  const disableStreetViewTabbing = !streetViewInteractive;
  const utilityControlPosition = 'absolute left-3 top-3 z-40 pointer-events-auto md:bottom-4 md:left-4 md:top-auto';

  const releaseStreetViewFocus = useCallback(() => {
    const frame = streetViewFrameRef.current;
    if (!frame || document.activeElement !== frame) return;

    frame.blur();
    sceneRef.current?.focus({ preventScroll: true });
  }, []);

  const playerScores = useMemo(() => {
    if (partyMode !== "free_for_all" || !roundResults.length) return [];
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
      participant: {
        kind: 'player' as const,
        id,
        name: resultPlayerNames[id] || 'Unknown',
        avatarUrl: resultPlayerAvatars[id],
        avatarFallback: resultPlayerFallbacks[id] || '?',
      } as ParticipantIdentityView,
      ...stats
    }));
    arr.sort((a, b) => b.score - a.score);
    return arr;
  }, [roundResults, resultPlayerNames, resultPlayerAvatars, resultPlayerFallbacks, partyMode]);

  const scoreboardPlayers = useMemo(
    () =>
      playerScores.map((player) => ({
        ...player,
        participant: participantsById[player.id] || player.participant,
      })),
    [participantsById, playerScores],
  );

  const countdownSec = (parseInt(ss, 10) || 0) + (parseInt(mm, 10) || 0) * 60;
  const showCountdown = !isSingleplayer && uiPhase === 'prematch_countdown' && countdownSec > 0 && countdownSec <= 3;

  useEffect(() => {
    document.documentElement.classList.add('game-active');
    return () => document.documentElement.classList.remove('game-active');
  }, []);

  useEffect(() => {
    if (canShowForfeit) return;
    setConfirmForfeit(false);
    setForfeitRequested(false);
  }, [canShowForfeit]);

  useEffect(() => {
    setStreetViewResetCount(0);
  }, [streetViewSrc]);

  useEffect(() => {
    if (!disableStreetViewTabbing) return;
    if (uiPhase !== 'live_round' && uiPhase !== 'prematch_countdown') return;

    const handleWindowBlur = () => {
      window.setTimeout(releaseStreetViewFocus, 0);
    };

    window.addEventListener('blur', handleWindowBlur);
    return () => window.removeEventListener('blur', handleWindowBlur);
  }, [disableStreetViewTabbing, releaseStreetViewFocus, uiPhase]);

  const handleForfeitConfirm = () => {
    const sent = onForfeit();
    if (!sent) {
      setConfirmForfeit(false);
      return;
    }
    if (isSingleplayer) {
      setConfirmForfeit(false);
      setForfeitRequested(false);
      onLeaveGame();
      return;
    }
    setForfeitRequested(true);
  };

  return (
    <section
      ref={sceneRef}
      tabIndex={-1}
      className={`fixed inset-0 overflow-hidden focus:outline-none ${motionPresetClass.reveal}`}
    >
      {(uiPhase === 'live_round' || uiPhase === 'prematch_countdown') && (
        <div className="absolute inset-0 overflow-hidden">
          <iframe
            key={`${streetViewSrc}-${streetViewResetCount}`}
            ref={streetViewFrameRef}
            title="Street View"
            src={streetViewSrc}
            tabIndex={disableStreetViewTabbing ? -1 : undefined}
            onFocus={disableStreetViewTabbing ? releaseStreetViewFocus : undefined}
            className={`absolute left-0 top-[-75px] h-[calc(100%+75px)] w-full border-0 ${streetViewInteractive ? '' : 'pointer-events-none'}`}
            allowFullScreen
            loading="eager"
          />
          {!streetViewInteractive ? <div className="absolute inset-0 z-[1]" aria-hidden="true" /> : null}
        </div>
      )}

      <AnimatePresence>
        {showResultStage && resultOverlay && <RoundResultOverlay {...resultOverlay} />}
      </AnimatePresence>

      <AnimatePresence>
        {connectionIssue && (
          <motion.div
            key="connection-issue-banner"
            initial={{ opacity: 0, y: -10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -10 }}
            transition={{ duration: 0.18, ease: 'easeOut' }}
            className="font-hud pointer-events-none absolute left-1/2 top-5 z-30 -translate-x-1/2 rounded-full border border-red-400/30 bg-[#2a1010]/90 px-4 py-2 text-[11px] uppercase tracking-[0.14em] text-red-100 shadow-[0_12px_30px_rgba(0,0,0,0.35)]"
          >
            {connectionIssue}
          </motion.div>
        )}
      </AnimatePresence>

      <AnimatePresence>
        {showCountdown && roundNumber === 1 && (
          <GameStartOverlay
            roundNumber={roundNumber}
            modeName={modeName}
            mapName={mapName}
            countdownSec={countdownSec}
            selfName={selfName}
            selfElo={selfElo}
            selfRatingPreview={selfRatingPreview}
            selfAvatarUrl={selfAvatarUrl}
            selfFallback={selfFallback}
            selfIsAdmin={selfIsAdmin}
            selfSelectedBadge={selfSelectedBadge}
            oppName={opponentName}
            oppElo={opponentElo}
            oppAvatarUrl={oppAvatarUrl}
            oppFallback={oppFallback}
            oppIsAdmin={opponentIsAdmin}
            oppSelectedBadge={opponentSelectedBadge}
            isFreeForAll={partyMode === 'free_for_all'}
          />
        )}
        {showCountdown && roundNumber > 1 && (
          <div className="pointer-events-none absolute inset-0 z-[100] flex items-center justify-center">
            <IntroCountdownText countdownSec={countdownSec} />
          </div>
        )}
      </AnimatePresence>

      <AnimatePresence>
        {uiPhase === 'live_round' && !isSingleplayer && (
          <motion.div
            key="game-hud"
            initial={{ opacity: 0, y: -10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -10 }}
            transition={{ duration: 0.2, ease: 'easeOut' }}
          >
            <GameHUD
              mm={mm}
              ss={ss}
              isRoundTimerRunning={isRoundTimerRunning}
              damageMultiplier={damageMultiplier}
              timerProgressPct={timerProgressPct}
              isTimerCritical={isTimerCritical}
              isTimerPulseActive={isTimerPulseActive}
              hideMultiplier={partyMode === "free_for_all"}
            />
          </motion.div>
        )}
      </AnimatePresence>

      {isSingleplayer || partyMode === "free_for_all" ? (
        <div className="absolute right-3 top-3 z-30 flex items-center gap-5 rounded-[18px] border border-white/10 bg-hudBg px-4 py-3 text-white shadow-elev-2 backdrop-blur-hud md:right-4 md:top-4">
          <div>
            <p className="font-hud text-[10px] uppercase tracking-[0.16em] text-white/60">Round</p>
            <p className="mt-1 text-2xl font-black text-white">
              {roundNumber}
              {totalRounds ? `/${totalRounds}` : ''}
            </p>
          </div>
          <div>
            <p className="font-hud text-[10px] uppercase tracking-[0.16em] text-white/60">Points</p>
            <p className="mt-1 text-2xl font-black text-white">{totalScore.toLocaleString()}</p>
          </div>
        </div>
      ) : (
        <>
          <PlayerHPCard
            side="left"
            name={selfName}
            elo={selfElo}
            hideElo={partyMode === "team_duel"}
            hp={selfHP}
            hpPct={hpPct(selfHP)}
            avatarUrl={selfAvatarUrl}
            fallback={selfFallback}
            avatarColor={selfAvatarColor}
            isAdmin={selfIsAdmin}
            selectedBadge={selfSelectedBadge}
          />
          <PlayerHPCard
            side="right"
            name={opponentName}
            elo={opponentElo}
            hideElo={partyMode === "team_duel"}
            hp={oppHP}
            hpPct={hpPct(oppHP)}
            avatarUrl={oppAvatarUrl}
            fallback={oppFallback}
            avatarColor={oppAvatarColor}
            isAdmin={opponentIsAdmin}
            selectedBadge={opponentSelectedBadge}
            opponent
            disconnected={opponentDisconnected}
          />
        </>
      )}

      <AnimatePresence>
        {canShowForfeit && (
          <motion.div
            key="forfeit-control"
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 10 }}
            transition={{ duration: 0.18, ease: 'easeOut' }}
            className={utilityControlPosition}
          >
            {confirmForfeit ? (
              <div className="glass-panel w-[min(calc(100vw-1.5rem),19rem)] rounded-[22px] p-3 text-white md:w-[19rem]">
                <div className="flex items-start gap-3">
                  <div className="mt-0.5 flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-full border border-red-300/15 bg-red-500/12 text-red-200">
                    <AlertTriangle size={18} strokeWidth={2.4} />
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="font-hud text-[11px] uppercase tracking-[0.16em] text-red-200/85">Forfeit Match</p>
                    <p className="mt-1 text-sm font-semibold text-white">
                      {isSingleplayer ? 'This ends the current practice run.' : isPointsMode ? 'This leaves the current match.' : 'This counts as a loss and ends the duel now.'}
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() => {
                      setConfirmForfeit(false);
                      setForfeitRequested(false);
                    }}
                    className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full bg-white/5 text-white/65 transition hover:bg-white/10 hover:text-white"
                    aria-label="Cancel forfeit"
                  >
                    <X size={16} strokeWidth={2.5} />
                  </button>
                </div>
                <div className="mt-3 flex gap-2">
                  <button
                    type="button"
                    onClick={() => {
                      setConfirmForfeit(false);
                      setForfeitRequested(false);
                    }}
                    className="font-hud min-h-11 flex-1 rounded-pill border border-white/10 bg-white/5 px-4 py-2 text-[11px] uppercase tracking-[0.14em] text-white/80 transition hover:bg-white/10 hover:text-white"
                  >
                    Keep Playing
                  </button>
                  <button
                    type="button"
                    onClick={handleForfeitConfirm}
                    disabled={forfeitRequested}
                    className="font-hud min-h-11 flex-1 rounded-pill border border-red-200/25 bg-[linear-gradient(135deg,rgba(255,109,66,0.96)_0%,rgba(196,57,35,0.96)_100%)] px-4 py-2 text-[11px] uppercase tracking-[0.14em] text-white shadow-[0_10px_24px_rgba(196,57,35,0.35)] transition hover:brightness-110 disabled:cursor-not-allowed disabled:opacity-75"
                  >
                    {forfeitRequested ? 'Forfeiting...' : 'Confirm'}
                  </button>
                </div>
              </div>
            ) : (
              <div className="flex items-center gap-2">
                {streetViewInteractive ? (
                  <button
                    type="button"
                    onClick={() => setStreetViewResetCount((count) => count + 1)}
                    aria-label="Return to spawn location"
                    className="flex h-11 w-11 items-center justify-center rounded-full bg-hudBg text-white/80 shadow-elev-2 backdrop-blur-hud transition hover:bg-white/10 hover:text-white"
                  >
                    <RotateCcw size={16} strokeWidth={2.4} />
                  </button>
                ) : null}
                <button
                  type="button"
                  onClick={() => setConfirmForfeit(true)}
                  aria-label="Forfeit match"
                  className="flex h-11 w-11 items-center justify-center rounded-full bg-hudBg text-white/80 shadow-elev-2 backdrop-blur-hud transition hover:bg-white/10 hover:text-white"
                >
                  <LogOut size={16} strokeWidth={2.4} />
                </button>
              </div>
            )}
          </motion.div>
        )}
      </AnimatePresence>

      {uiPhase === 'live_round' && (
        <MinimapPanel
          onFinalize={onFinalizeGuess}
          canFinalizeGuess={canFinalizeGuess}
          guessSubmitted={guessSubmitted}
          roundKey={streetViewSrc}
        >
          {guessMapNode}
        </MinimapPanel>
      )}

      <AnimatePresence>
        {isPointsMode && showResultStage && (
          <motion.div
            key="points-mode-result-stage"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="absolute inset-0 z-20 pointer-events-none"
          >
            <DuelOverlayBackground variant="points">
              <div className="pointer-events-auto h-full w-full">
                <div className="absolute left-1/2 top-10 z-30 flex w-[min(calc(100vw-2rem),24rem)] -translate-x-1/2 flex-col items-center md:top-12">
                  <motion.div
                    initial={{ y: 36, opacity: 0, scale: 0.92 }}
                  animate={{ y: 0, opacity: 1, scale: 1 }}
                  transition={{ type: 'spring', stiffness: 420, damping: 28 }}
                  className="font-hud font-bold text-center text-[clamp(3rem,10vw,4.8rem)] leading-none text-white drop-shadow-[0_6px_12px_rgba(59,130,246,0.95)]"
                >
              {currentRoundScore}
            </motion.div>
            <motion.div
              initial={{ y: -18, opacity: 0, scale: 0.95 }}
              animate={{ y: 0, opacity: 1, scale: 1 }}
              transition={{ type: 'spring', stiffness: 500, damping: 30, delay: 0.06 }}
              className="relative mt-5"
            >
              <ResultDistanceBar selfDistanceKm={currentRoundDistanceKm} compact />
            </motion.div>
          </div>
          <div className="absolute inset-x-3 bottom-3 top-44 z-20 flex flex-col gap-3 md:inset-x-4 md:bottom-4 md:top-48">
            <div className="min-h-0 flex-1">
              {resultMapNode}
            </div>
            {partyMode === "free_for_all" ? (
              <motion.div
                initial={{ y: 20, opacity: 0, scale: 0.98 }}
                animate={{ y: 0, opacity: 1, scale: 1 }}
                transition={{ duration: 0.22, ease: 'easeOut', delay: 0.12 }}
                className="mx-auto w-full max-w-2xl shrink-0 overflow-hidden rounded-[16px] border border-white/10 bg-[#162130]/80 shadow-[0_4px_24px_rgba(0,0,0,0.4)] backdrop-blur-md flex flex-col max-h-[35vh]"
              >
                <div className="overflow-y-auto w-full p-1 scrollbar-thin scrollbar-thumb-white/20 scrollbar-track-transparent">
                  <table className="w-full text-left text-sm text-white whitespace-nowrap">
                    <thead className="sticky top-0 bg-[#162130] z-10">
                      <tr className="border-b border-white/10 text-[#8caab0]">
                        <th className="py-2 px-3 font-bold uppercase tracking-wider w-12 text-center">#</th>
                        <th className="py-2 px-3 font-bold uppercase tracking-wider">Player</th>
                        <th className="py-2 px-3 font-bold uppercase tracking-wider text-right">Points</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-white/5">
                      {scoreboardPlayers.map((player, idx) => (
                        <tr key={player.id} className={player.id === selfUserId ? 'bg-white/[0.04]' : 'hover:bg-white/[0.02]'}>
                          <td className="py-2 px-3 text-center font-bold text-[#8caab0]">{idx + 1}</td>
                          <td className="py-2 px-3">
                            <div className="flex items-center gap-2">
                              <PlayerIdentityRow
                                participant={player.participant}
                                nameClassName={player.id === selfUserId ? 'font-black text-[#7dc3ff]' : 'font-bold'}
                              />
                            </div>
                          </td>
                          <td className="py-2 px-3 text-right font-black text-[#2ad18f]">{player.score.toLocaleString()}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </motion.div>
            ) : (
              <motion.button
                type="button"
                initial={{ y: 20, opacity: 0, scale: 0.98 }}
                animate={{ y: 0, opacity: 1, scale: 1 }}
                transition={{ duration: 0.22, ease: 'easeOut', delay: 0.12 }}
                onClick={canAdvanceRound ? onAdvanceRound : onLeaveGame}
                className="mx-auto inline-flex items-center justify-center rounded-[16px] bg-[#22d385] px-8 py-[16px] text-[16px] font-extrabold uppercase tracking-[0.08em] text-white shadow-[0_4px_16px_rgba(34,211,133,0.3)] transition-all duration-200 hover:scale-[1.01] hover:bg-[#2ae091] hover:shadow-[0_6px_24px_rgba(34,211,133,0.4)] active:scale-[0.98]"
              >
                {canAdvanceRound ? 'Next Round' : 'Back To Lobby'}
              </motion.button>
            )}
          </div>
              </div>
            </DuelOverlayBackground>
          </motion.div>
        )}
      </AnimatePresence>

      <AnimatePresence>
        {showGuessAlertBorder && (
          <motion.div
            key="opponent-guess-border"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.18, ease: 'easeOut' }}
            className="pointer-events-none absolute inset-0 z-20"
            style={{
              boxShadow:
                'inset 0 0 120px rgba(239, 68, 68, 0.2), inset 0 0 100px rgba(239, 68, 68, 0.35), inset 0 0 0 2px rgba(248, 113, 113, 0.35)'
            }}
          />
        )}
      </AnimatePresence>
    </section>
  );
}
