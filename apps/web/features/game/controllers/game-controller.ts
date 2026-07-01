import type { ResultPhase, RoundResult, Snapshot } from '../../../components/ui/types';
import type { RuntimeConfig } from '../../../lib/runtime-config';
import type { SfxController } from '../../../lib/audio/sfx';
import { ObservableStore } from '../../../lib/observable-store';
import type { SessionController } from '../../auth/controllers/session-controller';
import type { MatchController } from '../../matchmaking/controllers/match-controller';
import { ResultAnimation } from '../lib/result-animation';
import { GUESS_INPUT_CUTOFF_MS, PRESSURE_VISIBLE_MS, RoundClock } from '../lib/round-clock';

type Guess = { lat: number; lng: number } | undefined;

export type GameState = {
  persistedRoundResultCtx: { matchId: string; result: RoundResult } | null;
  guess: Guess;
  roundMSLeft: number;
  displayRoundSeconds: number;
  displayHP: Record<string, number>;
  opponentGuessAlert: boolean;
  guessSubmitted: boolean;
  resultPhase: ResultPhase;
  resultShownHP: { self: number; opp: number };
  showMatchEndPage: boolean;
};

const initialState: GameState = {
  persistedRoundResultCtx: null,
  guess: undefined,
  roundMSLeft: 0,
  displayRoundSeconds: 0,
  displayHP: {},
  opponentGuessAlert: false,
  guessSubmitted: false,
  resultPhase: 'base',
  resultShownHP: { self: 0, opp: 0 },
  showMatchEndPage: false
};

export class GameController extends ObservableStore<GameState> {
  private readonly config: RuntimeConfig;
  private state: GameState = initialState;
  private readonly matchController: MatchController;
  private readonly sessionController: SessionController;
  private readonly sfxController: SfxController;
  private unsubscribeMatch: (() => void) | null = null;
  private prevSnapshot: Snapshot | null = null;
  private prevRoundId = '';
  private prevSeq = 0;
  private resultAnimRound = '';
  private roundTimerSyncKey = '';
  private readonly resultAnimation: ResultAnimation;
  private readonly roundClock = new RoundClock();
  private hpTransitionTimer: ReturnType<typeof setTimeout> | null = null;
  private opponentGuessTimer: ReturnType<typeof setTimeout> | null = null;
  private introCountdownSfxKey = '';
  private roundCountdownSfxKey = '';
  private gameStartSfxKey = '';
  private readonly guessSfxKeys = new Set<string>();
  private readonly resultExitSfxKeys = new Set<string>();
  private destroyed = false;
  private started = false;

  constructor(params: {
    config: RuntimeConfig;
    matchController: MatchController;
    sessionController: SessionController;
    sfxController: SfxController;
  }) {
    super();
    this.config = params.config;
    this.matchController = params.matchController;
    this.sessionController = params.sessionController;
    this.sfxController = params.sfxController;
    this.resultAnimation = new ResultAnimation();
  }

  start() {
    if (this.started) return;
    this.destroyed = false;
    this.started = true;
    this.unsubscribeMatch = this.matchController.subscribe(() => {
      this.handleMatchChange(this.matchController.getState().snapshot);
    });
    this.handleMatchChange(this.matchController.getState().snapshot);
  }

  destroy() {
    this.destroyed = true;
    this.started = false;
    this.unsubscribeMatch?.();
    this.unsubscribeMatch = null;
    this.clearTimers();
    this.roundClock.reset();
  }

  getState() {
    return this.state;
  }

  private patchState(patch: Partial<GameState>) {
    this.state = { ...this.state, ...patch };
    if (!this.destroyed) {
      this.emit();
    }
  }

  private clearTimers() {
    this.resultAnimation.clear();
    if (this.hpTransitionTimer) clearTimeout(this.hpTransitionTimer);
    this.hpTransitionTimer = null;
    if (this.opponentGuessTimer) clearTimeout(this.opponentGuessTimer);
    this.opponentGuessTimer = null;
    this.introCountdownSfxKey = '';
    this.stopRoundCountdown();
    this.gameStartSfxKey = '';
    this.guessSfxKeys.clear();
    this.resultExitSfxKeys.clear();
    this.roundTimerSyncKey = '';
  }

  private handleMatchChange(snapshot: Snapshot | null) {
    const session = this.sessionController.getState();
    const userId = session.userId;
    const prev = this.prevSnapshot;

    if (snapshot?.matchId) {
      this.sfxController.start();
    }

    if (snapshot?.lastRoundResult && snapshot.matchId) {
      this.patchState({
        persistedRoundResultCtx: {
          matchId: snapshot.matchId,
          result: snapshot.lastRoundResult
        }
      });
    } else if (!snapshot) {
      this.patchState({ persistedRoundResultCtx: null });
    }

    const currentRoundId = snapshot?.currentRound?.roundId || '';
    if (currentRoundId !== this.prevRoundId) {
      this.prevRoundId = currentRoundId;
      this.patchState({
        guess: undefined,
        guessSubmitted: false
      });
    }

    if (snapshot && typeof snapshot.serverUnixMs === 'number' && Number.isFinite(snapshot.serverUnixMs)) {
      this.roundClock.setServerTime(snapshot.serverUnixMs);
    }

    this.syncGameStartSfx(prev, snapshot);
    this.syncGuessSfx(prev, snapshot);
    this.syncSelfGuess(snapshot, userId);
    this.syncCountdownLoopSnapshot(snapshot);
    this.syncPressureTimerCap(snapshot);
    this.syncRoundTimer(snapshot);
    this.syncDisplayHP(snapshot);
    this.syncResultAnimation(snapshot, userId);
    this.syncRecoveredEndedMatch(snapshot, userId);
    this.syncOpponentFinalized(prev, snapshot, userId);

    this.prevSnapshot = snapshot;
  }

  private syncSelfGuess(snapshot: Snapshot | null, userId: string) {
    if (!snapshot || snapshot.phase !== 'live' || snapshot.roundPhase !== 'round_live') return;
    const selfPlayer = snapshot.players?.[userId];
    const currentGuess = snapshot.self?.userId === userId ? snapshot.self.currentGuess : undefined;
    const patch: Partial<GameState> = {};
    if (currentGuess) {
      patch.guess = { lat: currentGuess.lat, lng: currentGuess.lng };
    }
    if (selfPlayer?.finalized) {
      patch.guessSubmitted = true;
    }
    if (Object.keys(patch).length > 0) {
      this.patchState(patch);
    }
  }

  private syncRoundTimer(snapshot: Snapshot | null) {
    if (!snapshot) {
      this.roundTimerSyncKey = '';
      this.roundClock.reset();
      this.patchState({ roundMSLeft: 0, displayRoundSeconds: 0 });
      this.stopRoundCountdown();
      return;
    }
    if (snapshot?.mode === 'singleplayer') {
      this.roundTimerSyncKey = '';
      this.roundClock.reset();
      this.patchState({ roundMSLeft: 0, displayRoundSeconds: 0 });
      this.introCountdownSfxKey = '';
      this.stopRoundCountdown();
      this.gameStartSfxKey = '';
      return;
    }
    const timerKey = this.roundTimerKey(snapshot);
    if (timerKey === this.roundTimerSyncKey) return;
    this.roundTimerSyncKey = timerKey;
    this.roundClock.start(snapshot, ({ roundMSLeft, displayRoundSeconds }) => {
      this.syncRoundSfx(snapshot, displayRoundSeconds);
      this.patchState({ roundMSLeft, displayRoundSeconds });
    });
  }

  private roundTimerKey(snapshot: Snapshot | null) {
    if (!snapshot) return 'none';
    const roundId = snapshot.currentRound?.roundId || '';
    const timerState = snapshot.currentRound?.timerStarted === false ? 'idle' : 'running';
    const deadline = snapshot.phaseEndsAt || 0;
    const fallbackMsLeft = deadline > 0 ? 0 : snapshot.roundMsLeft || 0;
    return [
      snapshot.mode || 'duel',
      snapshot.matchId,
      snapshot.phase,
      snapshot.roundPhase,
      roundId,
      timerState,
      deadline,
      fallbackMsLeft
    ].join(':');
  }

  private syncRoundSfx(snapshot: Snapshot | null, displayRoundSeconds: number) {
    if (
      !snapshot ||
      snapshot.mode === 'singleplayer' ||
      snapshot.phase !== 'live'
    ) {
      this.stopRoundCountdown();
      return;
    }
    if (snapshot.roundPhase === 'round_intro') {
      this.stopRoundCountdown();
      this.syncIntroCountdownSfx(snapshot, displayRoundSeconds);
      this.syncRoundIntroExitSfx(snapshot, displayRoundSeconds);
      return;
    }
    if (snapshot.roundPhase === 'round_live') {
      this.syncRoundCountdown(snapshot, displayRoundSeconds);
      return;
    }
    this.stopRoundCountdown();
  }

  private syncGameStartSfx(prev: Snapshot | null, next: Snapshot | null) {
    if (!next || next.mode !== 'singleplayer' || next.phase !== 'live' || next.roundPhase !== 'round_live') return;
    const roundId = next.currentRound?.roundId || '';
    if (!roundId || next.currentRound?.roundNumber !== 1) return;
    const isNewLiveRound =
      !prev ||
      prev.matchId !== next.matchId ||
      prev.currentRound?.roundId !== roundId ||
      prev.phase !== 'live' ||
      prev.roundPhase !== 'round_live';
    if (!isNewLiveRound) return;
    const key = `${next.matchId}:game-start`;
    if (key === this.gameStartSfxKey) return;
    this.gameStartSfxKey = key;
    this.sfxController.play('duel-game-start');
  }

  private syncCountdownLoopSnapshot(snapshot: Snapshot | null) {
    if (!snapshot || snapshot.mode === 'singleplayer' || snapshot.phase !== 'live' || snapshot.roundPhase !== 'round_live') {
      this.stopRoundCountdown();
    }
  }

  private syncPressureTimerCap(snapshot: Snapshot | null) {
    if (!snapshot || snapshot.mode === 'singleplayer' || snapshot.phase !== 'live' || snapshot.roundPhase !== 'round_live') return;
    if (!this.hasPressureTimer(snapshot)) return;
    const roundId = snapshot.currentRound?.roundId || '';
    if (!roundId) return;
    const finalizedCount = Object.values(snapshot.players || {}).filter((player) => player.finalized).length;
    if (finalizedCount > 0 && finalizedCount < Object.keys(snapshot.players || {}).length) {
      this.roundClock.forceRound(roundId);
    }
  }

  private hasPressureTimer(snapshot: Snapshot) {
    return (
      (typeof snapshot.config?.pressureTimeLimitMs === 'number' && snapshot.config.pressureTimeLimitMs > 0) ||
      snapshot.config?.roundTimerMode === 'pressure'
    );
  }

  private syncIntroCountdownSfx(snapshot: Snapshot, displayRoundSeconds: number) {
    if (displayRoundSeconds < 1 || displayRoundSeconds > 3) return;
    const key = `${snapshot.matchId}:${snapshot.currentRound?.roundId || ''}:intro`;
    if (key === this.introCountdownSfxKey) return;
    this.introCountdownSfxKey = key;
    this.sfxController.play('duel-round-result-countdown');
  }

  private syncRoundIntroExitSfx(snapshot: Snapshot, displayRoundSeconds: number) {
    if (displayRoundSeconds > 0) return;
    this.playResultExitSfx(`${snapshot.matchId}:${snapshot.currentRound?.roundId || ''}:round-intro-hide`);
  }

  private syncRoundCountdown(snapshot: Snapshot, displayRoundSeconds: number) {
    if (this.roundCountdownSfxKey) return;
    if (displayRoundSeconds < 1 || displayRoundSeconds > 15) return;
    const key = `${snapshot.matchId}:${snapshot.currentRound?.roundId || ''}:round-countdown`;
    this.roundCountdownSfxKey = key;
    this.sfxController.playManaged('duel-round-countdown');
  }

  private stopRoundCountdown() {
    if (!this.roundCountdownSfxKey) return;
    this.roundCountdownSfxKey = '';
    this.sfxController.stop('duel-round-countdown');
  }

  private syncGuessSfx(prev: Snapshot | null, next: Snapshot | null) {
    if (!prev || !next) return;
    const roundId = next.currentRound?.roundId || next.lastRoundResult?.roundId || '';
    const sameRound = prev.matchId === next.matchId && !!roundId && prev.currentRound?.roundId === roundId;
    if (!sameRound || prev.phase !== 'live' || prev.roundPhase !== 'round_live') return;

    if (next.mode === 'singleplayer') {
      const playerId = this.sessionController.getState().userId;
      const hasRoundResult = !!next.lastRoundResult?.players?.[playerId];
      if (hasRoundResult) {
        this.playGuessSfx(next.matchId, roundId, playerId);
      }
      return;
    }

    Object.entries(next.players || {}).forEach(([id, nextPlayer]) => {
      const prevPlayer = prev.players?.[id];
      if (!prevPlayer || prevPlayer.finalized || !nextPlayer.finalized) return;
      this.playGuessSfx(next.matchId, roundId, id);
    });
  }

  private playGuessSfx(matchId: string, roundId: string, playerId: string) {
    const key = `${matchId}:${roundId}:guess:${playerId}`;
    if (this.guessSfxKeys.has(key)) return;
    this.guessSfxKeys.add(key);
    this.sfxController.play('duel-round-guess');
  }

  private playResultExitSfx(key: string) {
    if (this.resultExitSfxKeys.has(key)) return;
    this.resultExitSfxKeys.add(key);
    this.sfxController.play('duel-round-result-exit');
  }

  private syncDisplayHP(snapshot: Snapshot | null) {
    if (!snapshot) return;
    if (snapshot.mode === 'singleplayer' || snapshot.mode === 'free_for_all') {
      const totalScore = snapshot.players[this.sessionController.getState().userId]?.totalScore || 0;
      this.patchState({ displayHP: { [this.sessionController.getState().userId]: totalScore } });
      this.prevSeq = snapshot.eventSequence;
      return;
    }
    const hpNow: Record<string, number> = {};
    Object.entries(snapshot.players).forEach(([id, player]) => {
      hpNow[id] = player.hp;
    });

    const isResultTick =
      !!snapshot.lastRoundResult &&
      (snapshot.phase === 'round_result' || snapshot.state === 'ended') &&
      snapshot.eventSequence !== this.prevSeq;

    if (!isResultTick) {
      this.patchState({ displayHP: hpNow });
      this.prevSeq = snapshot.eventSequence;
      return;
    }

    const roundResult = snapshot.lastRoundResult;
    if (!roundResult) {
      this.patchState({ displayHP: hpNow });
      this.prevSeq = snapshot.eventSequence;
      return;
    }

    const before: Record<string, number> = {};
    Object.entries(roundResult.players).forEach(([id, result]) => {
      before[id] = this.getAnimatedStartHP(snapshot, id, result.hpAfterRound || 0, hpNow[id] ?? 0);
    });
    this.patchState({ displayHP: before });
    if (this.hpTransitionTimer) clearTimeout(this.hpTransitionTimer);
    this.hpTransitionTimer = setTimeout(() => {
      this.patchState({ displayHP: hpNow });
    }, 90);
    this.prevSeq = snapshot.eventSequence;
  }

  private syncResultAnimation(snapshot: Snapshot | null, userId: string) {
    if (!snapshot?.lastRoundResult || !snapshot.matchId) {
      this.resultAnimation.clear();
      return;
    }
    if (snapshot.mode === 'singleplayer' || snapshot.mode === 'free_for_all') {
      const totalScore = snapshot.players[userId]?.totalScore || 0;
      if (this.resultAnimRound === snapshot.lastRoundResult.roundId) return;
      this.resultAnimation.clear();
      this.resultAnimRound = snapshot.lastRoundResult.roundId;
      this.sfxController.play('duel-round-result-enter');
      this.sfxController.play('duel-round-result-score-reveal');
      this.patchState({
        showMatchEndPage: false,
        resultPhase: 'hp_apply',
        resultShownHP: { self: totalScore, opp: 0 }
      });
      return;
    }
    if (
      (snapshot.mode && snapshot.mode !== 'duel' && snapshot.mode !== 'team_duel') ||
      !(snapshot.phase === 'round_result' || snapshot.state === 'ended')
    ) {
      this.resultAnimation.clear();
      return;
    }
    const selfTeamId = snapshot.players[userId]?.teamId || 'a';
    const playerIds = Object.keys(snapshot.players || {});
    const oppId = playerIds.find((id) => id !== userId) || '';
    const rr = snapshot.lastRoundResult;
    if (this.resultAnimRound === rr.roundId) return;
    const oppTeamId =
      snapshot.mode === 'team_duel'
        ? Object.keys(snapshot.teams || {}).find((teamId) => teamId !== selfTeamId) || 'b'
        : '';
    const selfResult = snapshot.mode === 'team_duel' ? rr.teams?.[selfTeamId] : rr.players[userId];
    const oppResult = snapshot.mode === 'team_duel' ? rr.teams?.[oppTeamId] : rr.players[oppId];
    if (!selfResult || !oppResult) return;

    this.resultAnimation.clear();
    const oldSelf =
      snapshot.mode === 'team_duel'
        ? this.getAnimatedStartTeamHP(
            snapshot,
            selfTeamId,
            selfResult.hpAfterRound || 0,
            snapshot.teams?.[selfTeamId]?.hp || 0,
          )
        : this.getAnimatedStartHP(snapshot, userId, selfResult.hpAfterRound || 0, snapshot.players[userId]?.hp || 0);
    const oldOpp =
      snapshot.mode === 'team_duel'
        ? this.getAnimatedStartTeamHP(
            snapshot,
            oppTeamId,
            oppResult.hpAfterRound || 0,
            snapshot.teams?.[oppTeamId]?.hp || 0,
          )
        : this.getAnimatedStartHP(snapshot, oppId, oppResult.hpAfterRound || 0, snapshot.players[oppId]?.hp || 0);
    this.resultAnimRound = rr.roundId;
    this.patchState({
      showMatchEndPage: false,
      resultShownHP: { self: oldSelf, opp: oldOpp }
    });
    this.sfxController.play('duel-round-result-enter');

    this.resultAnimation.schedule(false, {
      onImmediate: () => {
        this.patchState({
          resultPhase: 'hp_apply',
          resultShownHP: {
            self: selfResult.hpAfterRound || 0,
            opp: oppResult.hpAfterRound || 0
          }
        });
      },
      onPhase: (phase) => {
        if (phase === 'scores') {
          this.sfxController.play('duel-round-result-score-reveal');
        }
        this.patchState({ resultPhase: phase });
      },
      onHpApply: () => {
        if ((selfResult.damageTaken || 0) > 0 || (oppResult.damageTaken || 0) > 0) {
          this.sfxController.play('duel-round-result-hp-hit');
        }
        this.patchState({
          resultPhase: 'hp_apply',
          resultShownHP: {
            self: selfResult.hpAfterRound || 0,
            opp: oppResult.hpAfterRound || 0
          }
        });
      }
    });
  }

  private syncRecoveredEndedMatch(snapshot: Snapshot | null, userId: string) {
    if (!snapshot || snapshot.state !== 'ended' || snapshot.lastRoundResult || this.state.showMatchEndPage) return;
    if (snapshot.mode === 'singleplayer' || snapshot.mode === 'free_for_all') {
      this.patchState({
        resultPhase: 'hp_apply',
        showMatchEndPage: false,
        resultShownHP: {
          self: snapshot.players[userId]?.totalScore || 0,
          opp: 0
        }
      });
      return;
    }
    const selfTeamId = snapshot.players[userId]?.teamId || '';
    const opponentTeamId =
      snapshot.mode === 'team_duel'
        ? Object.keys(snapshot.teams || {}).find((teamId) => teamId !== selfTeamId) || ''
        : '';
    const playerIds = Object.keys(snapshot.players || {});
    const oppId = playerIds.find((id) => id !== userId) || '';
    this.patchState({
      resultPhase: 'hp_apply',
      showMatchEndPage: true,
      resultShownHP: {
        self:
          snapshot.mode === 'team_duel'
            ? snapshot.teams?.[selfTeamId]?.hp || 0
            : snapshot.players[userId]?.hp || 0,
        opp:
          snapshot.mode === 'team_duel'
            ? snapshot.teams?.[opponentTeamId]?.hp || 0
            : snapshot.players[oppId]?.hp || 0
      }
    });
  }

  private syncOpponentFinalized(prev: Snapshot | null, next: Snapshot | null, userId: string) {
    if (next?.mode !== 'duel' || !this.hasPressureTimer(next)) return;
    if (!prev || !next) return;
    const ids = Object.keys(next.players || {});
    const oppId = ids.find((id) => id !== userId) || '';
    const prevOpp = oppId ? prev.players?.[oppId] : undefined;
    const nextOpp = oppId ? next.players?.[oppId] : undefined;
    const nextSelf = next.players?.[userId];
    const sameRound = !!prev.currentRound?.roundId && prev.currentRound.roundId === next.currentRound?.roundId;
    if (
      next.phase === 'live' &&
      sameRound &&
      prevOpp &&
      nextOpp &&
      !prevOpp.finalized &&
      nextOpp.finalized &&
      !nextSelf?.finalized
    ) {
      this.roundClock.forceRound(next.currentRound?.roundId || '');
      this.patchState({
        roundMSLeft: Math.min(PRESSURE_VISIBLE_MS, this.state.roundMSLeft || PRESSURE_VISIBLE_MS),
        opponentGuessAlert: true
      });
      if (this.opponentGuessTimer) clearTimeout(this.opponentGuessTimer);
      this.opponentGuessTimer = setTimeout(() => this.patchState({ opponentGuessAlert: false }), 1000);
    }
  }

  private getAnimatedStartHP(snapshot: Snapshot, playerId: string, hpAfterRound: number, hpNow: number) {
    const prevHP = this.prevSnapshot?.matchId === snapshot.matchId ? this.prevSnapshot.players?.[playerId]?.hp : undefined;
    if (typeof prevHP === 'number') {
      return Math.max(hpAfterRound, Math.min(this.config.maxHP, prevHP));
    }

    const displayedHP = this.state.displayHP[playerId];
    if (typeof displayedHP === 'number') {
      return Math.max(hpAfterRound, Math.min(this.config.maxHP, displayedHP));
    }

    return Math.max(hpAfterRound, Math.min(this.config.maxHP, hpNow));
  }

  private getAnimatedStartTeamHP(
    snapshot: Snapshot,
    teamId: string,
    hpAfterRound: number,
    hpNow: number,
  ) {
    const prevHP =
      this.prevSnapshot?.matchId === snapshot.matchId
        ? this.prevSnapshot.teams?.[teamId]?.hp
        : undefined;
    if (typeof prevHP === 'number') {
      return Math.max(hpAfterRound, Math.min(this.config.maxHP, prevHP));
    }

    const displayedHP = (snapshot.teams?.[teamId]?.players || [])
      .map((playerId) => this.state.displayHP[playerId])
      .find((hp) => typeof hp === 'number');
    if (typeof displayedHP === 'number') {
      return Math.max(hpAfterRound, Math.min(this.config.maxHP, displayedHP));
    }

    return Math.max(hpAfterRound, Math.min(this.config.maxHP, hpNow));
  }

  private isGuessInputClosed(snapshot: Snapshot) {
    if (snapshot.mode === 'singleplayer') return false;
    if (snapshot.roundPhase === 'round_live' && snapshot.currentRound?.timerStarted === false) return false;
    return this.roundClock.getAuthoritativeRoundMSLeft(snapshot) < GUESS_INPUT_CUTOFF_MS;
  }

  placeGuess = (lat: number, lng: number) => {
    const snapshot = this.matchController.getState().snapshot;
    const userId = this.sessionController.getState().userId;
    if (snapshot?.phase !== 'live' || snapshot?.roundPhase !== 'round_live' || this.state.guessSubmitted) return;
    if (!snapshot?.currentRound) return;
    if (this.isGuessInputClosed(snapshot)) return;
    const sent = this.matchController.sendGameCommand(
      'guess.place',
      {
        userId,
        matchId: snapshot.matchId,
        roundId: snapshot.currentRound.roundId,
        lat,
        lng
      },
      { errorMessage: this.config.gameConnectionErrorMessage, forceReconnect: true }
    );
    if (!sent) return;
    this.matchController.setConnectionIssue('');
    this.patchState({ guess: { lat, lng } });
  };

  finalizeGuess = () => {
    const snapshot = this.matchController.getState().snapshot;
    const userId = this.sessionController.getState().userId;
    if (!this.state.guess || !snapshot?.currentRound || snapshot.phase !== 'live' || this.state.guessSubmitted) return;
    const sent = this.matchController.sendGameCommand(
      'guess.finalize',
      {
        userId,
        matchId: snapshot.matchId,
        roundId: snapshot.currentRound.roundId,
        lat: this.state.guess.lat,
        lng: this.state.guess.lng
      },
      { errorMessage: this.config.gameConnectionErrorMessage, forceReconnect: true }
    );
    if (!sent) return;
    this.matchController.setConnectionIssue('');
    this.playGuessSfx(snapshot.matchId, snapshot.currentRound.roundId, userId);
    this.patchState({ guessSubmitted: true });
  };

  advanceRound = () => {
    const snapshot = this.matchController.getState().snapshot;
    const userId = this.sessionController.getState().userId;
    if (!snapshot?.matchId || snapshot.mode !== 'singleplayer' || snapshot.phase !== 'round_result') return false;
    const sent = this.matchController.sendGameCommand(
      'round.advance',
      { userId, matchId: snapshot.matchId },
      { errorMessage: this.config.gameConnectionErrorMessage, forceReconnect: true }
    );
    if (!sent) return false;
    this.matchController.setConnectionIssue('');
    return true;
  };

  leaveGame = () => {
    const snapshot = this.matchController.getState().snapshot;
    const session = this.sessionController.getState();
    if (snapshot?.matchId) {
      this.matchController.sendGameCommand('session.leave_match', { userId: session.userId, matchId: snapshot.matchId }, { silent: true });
    }
    this.matchController.resetConnectionState();
    this.patchState(initialState);
    this.roundClock.reset();
    this.resultAnimRound = '';
    this.matchController.setStatus(session.nicknameRequired ? 'idle' : 'ready');
  };

  forfeitMatch = () => {
    const snapshot = this.matchController.getState().snapshot;
    const userId = this.sessionController.getState().userId;
    if (!snapshot?.matchId || snapshot.state === 'ended') return false;
    const sent = this.matchController.sendGameCommand(
      'match.forfeit',
      { userId, matchId: snapshot.matchId },
      { errorMessage: this.config.gameConnectionErrorMessage, forceReconnect: true }
    );
    if (!sent) return false;
    this.matchController.setConnectionIssue('');
    return true;
  };

  setShowMatchEndPage = (value: boolean) => {
    if (value && !this.state.showMatchEndPage) {
      const snapshot = this.matchController.getState().snapshot;
      const roundId = snapshot?.lastRoundResult?.roundId || 'recovered';
      const matchId = snapshot?.matchId || 'match';
      this.playResultExitSfx(`${matchId}:${roundId}:round-result-hide`);
    }
    this.patchState({ showMatchEndPage: value });
  };
}
