import type { RoundResultAnimationConfig } from '../model/types';

const scoreTravelDurationMs = Math.max(160, 500 * 0.72);
const damageTravelAtMs = Math.round(2000 + scoreTravelDurationMs);
const hpApplyAtMs = 3100;

export const RESULT_ANIMATION_CONFIG: RoundResultAnimationConfig = {
  timeline: {
    scoresAtMs: 1300,
    crushAtMs: 2000,
    damageTravelAtMs,
    damageMultiplierAtMs: Math.round(damageTravelAtMs + (hpApplyAtMs - damageTravelAtMs) / 2),
    hpApplyAtMs,
    endPageDelayMs: 450
  },
  overlayEnter: {
    fadeDuration: 0.2,
    mapScaleFrom: 0.95,
    mapSpringStiffness: 300,
    mapSpringDamping: 25,
    mapDelay: 0.1
  },
  roundBadge: {
    yFrom: -10,
    duration: 0.2,
    ease: 'easeOut'
  },
  panelGlow: {
    durationMs: 500
  },
  scoreReveal: {
    yFrom: 20,
    scaleFrom: 0.9,
    springStiffness: 560,
    springDamping: 30
  },
  scoreTravel: {
    durationMs: 500,
    minDurationMs: 160,
    durationScale: 0.72,
    ease: 'cubic-bezier(0.55,0,0.2,1)',
    exitDurationMs: 80,
    exitEase: 'easeIn'
  },
  scoreImpact: {
    y: -44,
    scale: 1.14,
    durationMs: 240,
    ease: 'cubic-bezier(0.22,1,0.36,1)',
    idleDurationMs: 0.2,
    idleEase: 'easeOut'
  },
  hpBar: {
    durationMs: 750,
    timingFunction: 'cubic-bezier(0.16, 1, 0.3, 1)'
  },
  hpShake: {
    keyframesX: [-4, 4, -4, 4, 0],
    durationMs: 400
  }
};
