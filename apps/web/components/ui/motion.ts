import type { MotionPreset } from './types';

export const motionPresetClass: Record<MotionPreset, string> = {
  fast: 'duration-normal ease-standard',
  impact: 'duration-emphasis ease-emphasized',
  reveal: 'duration-slow ease-gameplay'
};
