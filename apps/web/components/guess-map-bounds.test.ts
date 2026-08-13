import { describe, expect, it } from 'vitest';
import { playRegionCorners } from './guess-map-bounds';

describe('playRegionCorners', () => {
  it('passes through a non-crossing region unchanged', () => {
    expect(playRegionCorners({ minLat: 10, maxLat: 20, minLng: 30, maxLng: 40 })).toEqual([
      [10, 30],
      [20, 40],
    ]);
  });

  it('unwraps the eastern edge for an antimeridian-crossing region', () => {
    // Region runs east from +177 to -178, i.e. a narrow 5-degree band.
    expect(playRegionCorners({ minLat: -18, maxLat: -17, minLng: 177, maxLng: -178 })).toEqual([
      [-18, 177],
      [-17, 182],
    ]);
  });
});
