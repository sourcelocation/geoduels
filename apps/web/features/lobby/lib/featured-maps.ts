import type { CustomMap } from "../../maps/lib/maps-client";

export function createSeededRandom(seed: string) {
  let state = 2166136261;
  for (let index = 0; index < seed.length; index += 1) {
    state ^= seed.charCodeAt(index);
    state = Math.imul(state, 16777619);
  }
  return () => {
    state += 0x6d2b79f5;
    let value = state;
    value = Math.imul(value ^ (value >>> 15), value | 1);
    value ^= value + Math.imul(value ^ (value >>> 7), value | 61);
    return ((value ^ (value >>> 14)) >>> 0) / 4294967296;
  };
}

export function featuredMapDay(date = new Date()) {
  return date.toISOString().slice(0, 10);
}

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
