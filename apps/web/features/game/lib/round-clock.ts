import type { Snapshot } from '../model/types';

type RoundClockState = {
  roundMSLeft: number;
  displayRoundSeconds: number;
};

export const PRESSURE_VISIBLE_MS = 15_000;
export const GUESS_INPUT_CUTOFF_MS = 500;

export class RoundClock {
  private rafId = 0;
  private roundMSLeftRef = 0;
  private clockOffsetMs = 0;
  private timerDisplayKey = '';
  private timerDisplaySec = 0;
  private forcedRoundId = '';

  setServerTime(serverUnixMs: number) {
    this.clockOffsetMs = serverUnixMs - Date.now();
  }

  forceRound(roundId: string) {
    this.forcedRoundId = roundId;
  }

  reset() {
    if (typeof window !== 'undefined' && this.rafId) {
      window.cancelAnimationFrame(this.rafId);
    }
    this.rafId = 0;
    this.roundMSLeftRef = 0;
    this.clockOffsetMs = 0;
    this.timerDisplayKey = '';
    this.timerDisplaySec = 0;
    this.forcedRoundId = '';
  }

  getAuthoritativeRoundMSLeft(snapshot: Snapshot | null) {
    if (!snapshot || snapshot.phase !== 'live') return 0;
    if (snapshot.roundPhase === 'round_live' && snapshot.currentRound?.timerStarted === false) {
      return Number.POSITIVE_INFINITY;
    }
    if (snapshot.phaseEndsAt > 0) {
      return Math.max(0, snapshot.phaseEndsAt - (Date.now() + this.clockOffsetMs));
    }
    return Math.max(0, snapshot.roundMsLeft || 0);
  }

  start(snapshot: Snapshot | null, onTick: (state: RoundClockState) => void) {
    if (typeof window === 'undefined') return;
    if (this.rafId) {
      window.cancelAnimationFrame(this.rafId);
      this.rafId = 0;
    }
    if (!snapshot) {
      this.reset();
      onTick({ roundMSLeft: 0, displayRoundSeconds: 0 });
      return;
    }

    let lastTs = 0;
    const tick = (ts: number) => {
      let nextMsLeft = Math.max(0, snapshot.roundMsLeft || 0);
      if (snapshot.phase === 'live') {
        const phaseEndsAt = snapshot.phaseEndsAt || 0;
        if (phaseEndsAt > 0) {
          const serverNow = Date.now() + this.clockOffsetMs;
          nextMsLeft = Math.max(0, phaseEndsAt - serverNow);
          const roundId = snapshot.currentRound?.roundId || '';
          if (this.forcedRoundId && roundId === this.forcedRoundId) {
            nextMsLeft = Math.min(PRESSURE_VISIBLE_MS, nextMsLeft);
          } else if (this.forcedRoundId && roundId !== this.forcedRoundId) {
            this.forcedRoundId = '';
          }
        } else if (lastTs > 0) {
          nextMsLeft = Math.max(0, this.roundMSLeftRef - (ts - lastTs));
        }
      }

      this.roundMSLeftRef = nextMsLeft;
      const timerKey = `${snapshot.matchId}:${snapshot.currentRound?.roundId || ''}:${snapshot.phase}:${snapshot.roundPhase}:${snapshot.currentRound?.timerStarted === false ? 'idle' : 'running'}`;
      const rawSeconds = Math.max(0, Math.ceil(nextMsLeft / 1000));
      let stableSeconds = rawSeconds;
      if (this.timerDisplayKey !== timerKey) {
        this.timerDisplayKey = timerKey;
      } else if (rawSeconds > this.timerDisplaySec) {
        stableSeconds = this.timerDisplaySec;
      }
      this.timerDisplaySec = stableSeconds;
      onTick({
        roundMSLeft: nextMsLeft,
        displayRoundSeconds: stableSeconds
      });
      lastTs = ts;
      this.rafId = window.requestAnimationFrame(tick);
    };

    this.rafId = window.requestAnimationFrame(tick);
  }
}
