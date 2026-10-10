import type { MapDetails } from "../features/maps/lib/maps-client";
import {
  fetchMatchView,
  type MatchView,
} from "../features/matchmaking/lib/queue-client";
import type { PublicPlayerProfile } from "../features/players/types";
import { apiFetch } from "./http";
import type { RuntimeConfig } from "./runtime-config";

const PREVIEW_TIMEOUT_MS = 3000;

export async function loadPublicMapPreview(
  config: RuntimeConfig,
  mapId: string,
): Promise<MapDetails | null> {
  if (!mapId) return null;
  try {
    const resp = await apiFetch(config, `/api/maps/${encodeURIComponent(mapId)}`, {
      signal: AbortSignal.timeout(PREVIEW_TIMEOUT_MS),
    });
    if (!resp.ok) return null;
    return (await resp.json()) as MapDetails;
  } catch {
    return null;
  }
}

export async function loadPublicProfilePreview(
  config: RuntimeConfig,
  nickname: string,
): Promise<{ profile: PublicPlayerProfile | null; missing: boolean }> {
  if (!nickname) return { profile: null, missing: true };
  try {
    const resp = await apiFetch(
      config,
      `/api/players/${encodeURIComponent(nickname)}`,
      { signal: AbortSignal.timeout(PREVIEW_TIMEOUT_MS) },
    );
    if (resp.status === 404) return { profile: null, missing: true };
    if (!resp.ok) return { profile: null, missing: false };
    return { profile: (await resp.json()) as PublicPlayerProfile, missing: false };
  } catch {
    return { profile: null, missing: false };
  }
}

export async function loadPublicMatchPreview(
  config: RuntimeConfig,
  matchId: string,
): Promise<MatchView | null> {
  if (!matchId) return null;
  try {
    return await fetchMatchView(
      config,
      matchId,
      AbortSignal.timeout(PREVIEW_TIMEOUT_MS),
    );
  } catch {
    return null;
  }
}
