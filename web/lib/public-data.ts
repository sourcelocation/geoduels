import type { MapDetails } from "../features/maps/lib/maps-client";
import type { PublicPlayerProfile } from "../features/players/types";
import { apiFetch } from "./http";

const TIMEOUT_MS = 3000;

// A missing thing is null. Anything else going wrong throws, so a cached page is never replaced by
// one saying its content doesn't exist because the API was briefly away.
async function readPublic<T>(path: string): Promise<T | null> {
  const resp = await apiFetch(path, { signal: AbortSignal.timeout(TIMEOUT_MS) });
  if (resp.status === 404) return null;
  if (!resp.ok) throw new Error(`${path}: ${resp.status}`);
  return (await resp.json()) as T;
}

export function loadPublicMap(mapId: string) {
  return mapId ? readPublic<MapDetails>(`/api/maps/${encodeURIComponent(mapId)}`) : Promise.resolve(null);
}

export function loadPublicProfile(nickname: string) {
  return nickname
    ? readPublic<PublicPlayerProfile>(`/api/players/${encodeURIComponent(nickname)}`)
    : Promise.resolve(null);
}
