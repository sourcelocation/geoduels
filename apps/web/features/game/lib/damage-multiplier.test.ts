import { describe, expect, it } from 'vitest';
import { formatDamageMultiplierLabel } from './damage-multiplier';

describe('formatDamageMultiplierLabel', () => {
  it('hides the baseline multiplier', () => {
    expect(formatDamageMultiplierLabel(1)).toBeNull();
    expect(formatDamageMultiplierLabel(1.04)).toBeNull();
  });

  it('omits unnecessary decimals', () => {
    expect(formatDamageMultiplierLabel(1.5)).toBe('1.5x');
    expect(formatDamageMultiplierLabel(2)).toBe('2x');
    expect(formatDamageMultiplierLabel(2.01)).toBe('2x');
  });
});
