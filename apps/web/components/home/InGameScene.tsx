import { AnimatePresence, motion } from 'framer-motion';
import { AppPanel } from '../ui/compositions';
import { Badge } from '../ui/Badge';
import { Button } from '../ui/button';
import { AlertTriangle, Flag, LogOut, X } from 'lucide-react';
import { useCallback, useEffect, useRef, useState, useMemo, type ReactNode } from 'react';
import GameHUD from '../../features/game/components/overlays/GameHUD';
import MinimapPanel from '../../features/game/components/overlays/MinimapPanel';
import MatchSideHPCard from '../../features/game/components/overlays/MatchSideHPCard';
import RoundResultOverlay from '../../features/game/components/overlays/RoundResultOverlay';
import { ResultDistanceBar } from '../../features/game/components/overlays/ResultDistanceBar';
import GameStartOverlay from '../../features/game/components/overlays/GameStartOverlay';
import DuelOverlayBackground from '../../features/game/components/overlays/DuelOverlayBackground';
import IntroCountdownText from '../../features/game/components/overlays/IntroCountdownText';
import { ParticipantIdentityRow } from '../../features/game/components/overlays/ParticipantIdentity';
import { motionPresetClass } from '../ui/motion';
import type { RoundResultOverlayProps, UIPhase } from '../../features/game/model/types';
import type { MatchSidesView, PlayerIdentityView } from '../../features/game/components/overlays/ParticipantIdentity';
import { StreetViewEnhancements } from '../../features/browser-extension/components/StreetViewEnhancements';
import { useGeoDuelsExtension } from '../../features/browser-extension/hooks/use-geoduels-extension';
import { useHotkey } from '../../features/hotkeys/hooks/use-hotkey';

export type InGameSceneProps = {
  uiPhase: UIPhase;
  streetViewSrc: string;
  streetViewInteractive: boolean;
  ruleset: "moving" | "no_move" | "nmpz";
  streetNames: "shown" | "hidden";
  showResultStage: boolean;
  isSingleplayer: boolean;
  isPointsMode: boolean;
  partyMode?: "duel" | "team_duel" | "free_for_all";
  backLabel?: string;
  resultOverlay?: RoundResultOverlayProps;
  sides: MatchSidesView;
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
  roundResults?: import('../../features/game/model/types').RoundResult[];
  resultPlayerNames?: Record<string, string | undefined>;
  resultPlayerAvatars?: Record<string, string | undefined>;
  resultPlayerFallbacks?: Record<string, string | undefined>;
  participantsById?: Record<string, PlayerIdentityView>;
  damageMultiplier: number;
  multiplierMode?: "shared" | "individual";
  selfDamageMultiplier?: number;
  oppDamageMultiplier?: number;
  guessSubmitted: boolean;
  opponentGuessAlert: boolean;
  connectionIssue: string;
  roundNumber?: number;
  totalRounds?: number;
  modeName?: string;
  mapName?: string;
  selfUserId: string;
};

function buildExtensionStreetViewSrc(
  streetViewSrc: string,
  ruleset: InGameSceneProps["ruleset"],
  streetNames: InGameSceneProps["streetNames"],
) {
  if (!streetViewSrc) return streetViewSrc;
  try {
    const url = new URL(streetViewSrc);
    const hashParams = new URLSearchParams(url.hash.slice(1));
    hashParams.set(
      "geoduels",
      JSON.stringify({
        version: 1,
        ruleset,
        streetNames,
      }),
    );
    url.hash = hashParams.toString();
    return url.toString();
  } catch {
    return streetViewSrc;
  }
}

export default function InGameScene({
  uiPhase,
  streetViewSrc,
  streetViewInteractive,
  ruleset,
  streetNames,
  showResultStage,
  isSingleplayer,
  isPointsMode,
  partyMode = "duel",
  backLabel = "Back to lobby",
  resultOverlay,
  sides,
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
  damageMultiplier,
  multiplierMode = "shared",
  selfDamageMultiplier = 1,
  oppDamageMultiplier = 1,
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
  const streetViewFrameSrc = useMemo(
    () => buildExtensionStreetViewSrc(streetViewSrc, ruleset, streetNames),
    [streetNames, streetViewSrc, ruleset],
  );
  const extension = useGeoDuelsExtension(
    streetViewFrameRef,
    streetViewFrameSrc,
    ruleset,
    streetNames,
  );
  const extensionRequired = ruleset === "no_move" || streetNames === "hidden";
  const streetViewReady = !extensionRequired || extension.configured;
  const canShowForfeit = uiPhase !== 'match_end';
  const disableStreetViewTabbing = !streetViewInteractive;
  const utilityControlPosition = 'absolute left-3 top-3 z-game-controls pointer-events-auto md:bottom-4 md:left-4 md:top-auto';

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
      } as PlayerIdentityView,
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

  useHotkey({
    action: "gameplay.resetView",
    scope: "gameplay",
    enabled: streetViewInteractive && (uiPhase === "live_round" || uiPhase === "prematch_countdown"),
    run: () => setStreetViewResetCount((count) => count + 1),
  });

  return (
    <section
      ref={sceneRef}
      tabIndex={-1}
      className={`fixed inset-0 overflow-hidden focus:outline-none ${motionPresetClass.reveal}`}
    >
      {(uiPhase === 'live_round' || uiPhase === 'prematch_countdown') && (
        <div className="absolute inset-0 overflow-hidden">
          <iframe
            key={`${streetViewFrameSrc}-${streetViewResetCount}`}
            ref={streetViewFrameRef}
            title="Street View"
            src={streetViewFrameSrc}
            tabIndex={disableStreetViewTabbing ? -1 : undefined}
            onFocus={disableStreetViewTabbing ? releaseStreetViewFocus : undefined}
            onLoad={extension.onFrameLoad}
            className={`absolute left-0 top-[-75px] h-[calc(100%+75px)] w-full border-0 ${streetViewInteractive ? '' : 'pointer-events-none'}`}
            allowFullScreen
            loading="eager"
          />
          {!streetViewInteractive ? <div className="absolute inset-0 z-underlay" aria-hidden="true" /> : null}
          {!streetViewReady ? (
            <div className="absolute inset-0 z-content grid place-items-center bg-surface-page px-6 text-center text-body-sm font-strong text-content-secondary">
              {extension.unsupportedVersion ? (
                "Update the official extension to keep playing this mode."
              ) : extension.timedOut ? (
                <div className="flex flex-col items-center gap-3">
                  <span>Couldn&apos;t reach the official extension.</span>
                  <Button
                    variant="secondary"
                    type="button"
                    onClick={() => {
                      extension.retry();
                      setStreetViewResetCount((count) => count + 1);
                    }}
                    className="rounded-full bg-surface-fill px-4 py-2 text-content-primary transition hover:bg-surface-raised"
                  >
                    Retry
                  </Button>
                </div>
              ) : (
                "Preparing official extension…"
              )}
            </div>
          ) : null}
        </div>
      )}

      {extension.available && extension.capabilities ? (
        <StreetViewEnhancements
          capabilities={extension.capabilities}
          heading={extension.heading}
        />
      ) : null}

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
            className="pointer-events-none absolute left-1/2 top-5 z-game-controls -translate-x-1/2"
          >
            <Badge tone="danger" className="font-hud border border-status-danger/30 px-4 py-2 text-label shadow-elev-2">
              {connectionIssue}
            </Badge>
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
            sides={sides}
            isFreeForAll={partyMode === 'free_for_all'}
          />
        )}
        {showCountdown && roundNumber > 1 && (
          <div className="pointer-events-none absolute inset-0 z-game-overlay flex items-center justify-center">
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
              hideMultiplier={partyMode === "free_for_all" || multiplierMode === "individual"}
              hasTopCompass={
                extension.available &&
                extension.capabilities?.heading === true
              }
            />
          </motion.div>
        )}
      </AnimatePresence>

      {isSingleplayer || partyMode === "free_for_all" ? (
        <div className="absolute right-3 top-3 z-game-controls flex items-center gap-5 rounded-xl border border-hud-border bg-hud-surface px-4 py-3 text-content-primary shadow-elev-2 backdrop-blur-hud md:right-4 md:top-4">
          <div>
            <p className="font-hud text-hud-label uppercase tracking-eyebrow-wide text-content-secondary">Round</p>
            <p className="mt-1 font-display text-heading-md font-strong text-content-primary">
              {roundNumber}
              {totalRounds ? `/${totalRounds}` : ''}
            </p>
          </div>
          <div>
            <p className="font-hud text-hud-label uppercase tracking-eyebrow-wide text-content-secondary">Points</p>
            <p className="mt-1 font-display text-heading-md font-strong text-content-primary">{totalScore.toLocaleString()}</p>
          </div>
        </div>
      ) : (
        <>
          <MatchSideHPCard
            position="left"
            side={{ ...sides.self, hp: selfHP }}
            hpPct={hpPct(selfHP)}
            damageMultiplier={multiplierMode === "individual" ? selfDamageMultiplier : undefined}
          />
          <MatchSideHPCard
            position="right"
            side={{ ...sides.opponent, hp: oppHP }}
            hpPct={hpPct(oppHP)}
            opponent
            damageMultiplier={multiplierMode === "individual" ? oppDamageMultiplier : undefined}
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
              <AppPanel className="w-[min(calc(100vw-1.5rem),19rem)] rounded-xl p-3 text-content-primary md:w-[19rem]">
                <div className="flex items-start gap-3">
                  <div className="mt-0.5 flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-full border border-status-danger/20 bg-status-danger/15 text-status-danger">
                    <AlertTriangle size={18} strokeWidth={2.4} />
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="font-hud text-hud-label uppercase tracking-eyebrow-wide text-status-danger">Forfeit Match</p>
                    <p className="mt-1 text-body-sm font-semibold text-content-primary">
                      {isSingleplayer ? 'This ends the current practice run.' : isPointsMode ? 'This leaves the current match.' : 'This counts as a loss and ends the duel now.'}
                    </p>
                  </div>
                  <Button
                    variant="ghost"
                    type="button"
                    onClick={() => {
                      setConfirmForfeit(false);
                      setForfeitRequested(false);
                    }}
                    className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full bg-surface-fill text-content-secondary transition hover:bg-surface-raised hover:text-content-primary"
                    aria-label="Cancel forfeit"
                  >
                    <X size={16} strokeWidth={2.5} />
                  </Button>
                </div>
                <div className="mt-3 flex gap-2">
                  <Button
                    variant="secondary"
                    type="button"
                    onClick={() => {
                      setConfirmForfeit(false);
                      setForfeitRequested(false);
                    }}
                    className="font-hud min-h-11 flex-1 rounded-full border border-border-default bg-surface-fill px-4 py-2 text-label uppercase tracking-control text-content-primary transition hover:bg-surface-raised"
                  >
                    Keep Playing
                  </Button>
                  <Button
                    variant="danger"
                    type="button"
                    onClick={handleForfeitConfirm}
                    disabled={forfeitRequested}
                    className="font-hud min-h-11 flex-1 rounded-full border border-status-danger/35 bg-action-danger px-4 py-2 text-label uppercase tracking-control text-content-on-danger shadow-elev-2 transition hover:brightness-110 disabled:cursor-not-allowed disabled:opacity-75"
                  >
                    {forfeitRequested ? 'Forfeiting...' : 'Confirm'}
                  </Button>
                </div>
              </AppPanel>
            ) : (
              <div className="flex items-center gap-2">
                {streetViewInteractive ? (
                  <Button
                    variant="ghost"
                    type="button"
                    onClick={() => setStreetViewResetCount((count) => count + 1)}
                    aria-label="Return to spawn location"
                    className="flex h-11 w-11 items-center justify-center rounded-full bg-hud-surface text-content-primary shadow-elev-2 backdrop-blur-hud transition hover:bg-surface-fill"
                  >
                    <Flag size={16} strokeWidth={2.4} />
                  </Button>
                ) : null}
                <Button
                  variant="ghost"
                  type="button"
                  onClick={() => setConfirmForfeit(true)}
                  aria-label="Forfeit match"
                  className="flex h-11 w-11 items-center justify-center rounded-full bg-hud-surface text-content-primary shadow-elev-2 backdrop-blur-hud transition hover:bg-surface-fill"
                >
                  <LogOut size={16} strokeWidth={2.4} />
                </Button>
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
          reserveNativeStreetViewControls={!extension.available}
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
            className="absolute inset-0 z-sticky pointer-events-none"
          >
            <DuelOverlayBackground variant="points">
              <div className="pointer-events-auto h-full w-full">
                <div className="absolute left-1/2 top-10 z-game-controls flex w-[min(calc(100vw-2rem),24rem)] -translate-x-1/2 flex-col items-center md:top-12">
                  <motion.div
                    initial={{ y: 36, opacity: 0, scale: 0.92 }}
                  animate={{ y: 0, opacity: 1, scale: 1 }}
                  transition={{ type: 'spring', stiffness: 420, damping: 28 }}
                  className="font-hud text-display-lg text-center font-strong leading-collapsed text-content-primary drop-shadow-md"
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
          <div className="absolute inset-x-3 bottom-3 top-44 z-sticky flex flex-col gap-3 md:inset-x-4 md:bottom-4 md:top-48">
            <div className="min-h-0 flex-1">
              {resultMapNode}
            </div>
            {partyMode === "free_for_all" ? (
              <motion.div
                initial={{ y: 20, opacity: 0, scale: 0.98 }}
                animate={{ y: 0, opacity: 1, scale: 1 }}
                transition={{ duration: 0.22, ease: 'easeOut', delay: 0.12 }}
                className="mx-auto w-full max-w-2xl shrink-0 overflow-hidden rounded-xl border border-border-default bg-surface-raised shadow-elev-2 backdrop-blur-md flex flex-col max-h-[35vh]"
              >
                <div className="overflow-y-auto w-full p-1 scrollbar-thin scrollbar-thumb-white/20 scrollbar-track-transparent">
                  <table className="w-full text-left text-body-sm text-content-primary whitespace-nowrap">
                    <thead className="sticky top-0 bg-surface-raised z-content">
                      <tr className="border-b border-border-default text-content-secondary">
                        <th className="py-2 px-3 font-strong uppercase tracking-wide w-12 text-center">#</th>
                        <th className="py-2 px-3 font-strong uppercase tracking-wide">Player</th>
                        <th className="py-2 px-3 font-strong uppercase tracking-wide text-right">Points</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-border-default">
                      {scoreboardPlayers.map((player, idx) => (
                        <tr key={player.id} className={player.id === selfUserId ? 'bg-surface-fill' : 'hover:bg-surface-fill'}>
                          <td className="py-2 px-3 text-center font-strong text-content-secondary">{idx + 1}</td>
                          <td className="py-2 px-3">
                            <div className="flex items-center gap-2">
                              <ParticipantIdentityRow
                                participant={player.participant}
                                nameClassName={player.id === selfUserId ? 'font-strong text-brand-blue' : 'font-strong text-content-primary'}
                              />
                            </div>
                          </td>
                          <td className="py-2 px-3 text-right font-strong text-status-success">{player.score.toLocaleString()}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </motion.div>
            ) : (
              <Button
                variant="primary"
                type="button"
                onClick={canAdvanceRound ? onAdvanceRound : onLeaveGame}
                className="mx-auto inline-flex items-center justify-center rounded-xl bg-action-primary px-8 py-4 text-label font-strong uppercase tracking-label text-content-on-action shadow-elev-2 transition-all duration-normal hover:scale-[1.01] hover:bg-action-primary-hover active:scale-[0.98]"
              >
                {canAdvanceRound ? 'Next Round' : backLabel}
              </Button>
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
            className="pointer-events-none absolute inset-0 z-sticky"
            style={{
              boxShadow:
                'var(--gd-forfeit-glow)'
            }}
          />
        )}
      </AnimatePresence>
    </section>
  );
}
