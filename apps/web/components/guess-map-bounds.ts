export type PlayRegionBounds = {
  minLat: number;
  maxLat: number;
  minLng: number;
  maxLng: number;
};

/**
 * Converts stored play-region bounds into Leaflet corners.
 *
 * Longitude bounds follow the shortest-circular-interval convention: the region
 * runs eastward from `minLng` to `maxLng`, and `maxLng < minLng` signals that it
 * crosses the antimeridian. In that case the eastern edge is unwrapped past
 * +180 so Leaflet frames the narrow region instead of the whole globe.
 */
export function playRegionCorners(bounds: PlayRegionBounds): [[number, number], [number, number]] {
  const east = bounds.maxLng < bounds.minLng ? bounds.maxLng + 360 : bounds.maxLng;
  return [
    [bounds.minLat, bounds.minLng],
    [bounds.maxLat, east],
  ];
}
