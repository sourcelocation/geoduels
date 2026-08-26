import type { CustomMap } from "../../maps/lib/maps-client";

function featuredMapWeight(map: CustomMap) {
  // Logarithmic weighting makes large regional maps more likely without letting
  // location count turn the result into a deterministic size ranking.
  return Math.max(1, Math.log1p(Math.max(0, map.locationCount)));
}

export function selectFeaturedOfficialMaps(
  maps: CustomMap[],
  limit: number,
  random: () => number = Math.random,
) {
  return maps
    .filter(
      (map) =>
        map.status === "ready" &&
        (map.official || map.system) &&
        !map.modeMoving &&
        !map.modeNoMove &&
        !map.modeNmpz,
    )
    .map((map) => ({
      map,
      // Weighted reservoir sampling without replacement. Every eligible map
      // retains a non-zero chance; higher weights only improve its odds.
      key: -Math.log(Math.max(Number.EPSILON, random())) / featuredMapWeight(map),
    }))
    .sort((left, right) => left.key - right.key)
    .slice(0, Math.max(0, limit))
    .map(({ map }) => map);
}
