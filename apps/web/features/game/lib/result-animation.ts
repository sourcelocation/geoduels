import { RESULT_ANIMATION_CONFIG } from './round-result-animation-config';
import type { ResultPhase } from '../model/types';

export class ResultAnimation {
  private timers: Array<ReturnType<typeof setTimeout>> = [];

  clear() {
    this.timers.forEach((timer) => clearTimeout(timer));
    this.timers = [];
  }

  schedule(
    reduceMotion: boolean,
    handlers: {
      onImmediate: () => void;
      onPhase: (phase: ResultPhase) => void;
      onHpApply: () => void;
    }
  ) {
    this.clear();
    if (reduceMotion) {
      handlers.onImmediate();
      return;
    }
    handlers.onPhase('base');
    this.timers.push(setTimeout(() => handlers.onPhase('scores'), RESULT_ANIMATION_CONFIG.timeline.scoresAtMs));
    this.timers.push(setTimeout(() => handlers.onPhase('crush'), RESULT_ANIMATION_CONFIG.timeline.crushAtMs));
    this.timers.push(setTimeout(() => handlers.onPhase('damage_travel'), RESULT_ANIMATION_CONFIG.timeline.damageTravelAtMs));
    this.timers.push(
      setTimeout(() => handlers.onPhase('damage_multiplier'), RESULT_ANIMATION_CONFIG.timeline.damageMultiplierAtMs)
    );
    this.timers.push(setTimeout(() => handlers.onHpApply(), RESULT_ANIMATION_CONFIG.timeline.hpApplyAtMs));
  }
}
