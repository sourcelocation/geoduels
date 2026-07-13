import type { RuntimeConfig } from "../../../lib/runtime-config";
import { apiFetch, authHeaders, expectJSON, mergeHeaders } from "../../../lib/http";

export type CustomMap = {
  id: string;
  mapKey?: string;
  ownerUserId?: string;
  authorName?: string;
  displayName: string;
  description?: string;
  visibility: "private" | "unlisted" | "public";
  status: "processing" | "ready" | "rejected" | "archived";
  difficulty: "easy" | "normal" | "hard";
  thumbnailVariant: number;
  thumbnailKey: string;
  locationCount: number;
  personalBest?: {
    score: number;
    matchId: string;
    achievedAt: string;
  };
  system: boolean;
  official?: boolean;
  publishedAt?: string;
  playCount: number;
  favoriteCount: number;
  commentCount: number;
  trendingScore: number;
  favorited?: boolean;
  officialRegion?: string;
  autoZoomPlayRegion?: boolean;
  rankedMoving?: boolean;
  rankedNmpz?: boolean;
  defaultMoving?: boolean;
  defaultNmpz?: boolean;
  createdAt: string;
  updatedAt: string;
};
export type MapVisibility = CustomMap["visibility"];

export type MapScope = "official" | "community" | "favorites" | "mine";
export type MapSort = "trending" | "popular" | "new";
export type MapCountryStat = { country: string; locationCount: number };
export type MapComment = {
  id: string;
  mapId: string;
  parentId?: string;
  userId: string;
  userDisplayName: string;
  avatarUrl?: string;
  body: string;
  status: "visible" | "deleted" | "moderated";
  canDelete?: boolean;
  likeCount: number;
  liked?: boolean;
  createdAt: string;
  updatedAt: string;
  replies?: MapComment[];
};
export type MapDetails = { map: CustomMap; countryStats: MapCountryStat[]; comments: MapComment[] };
export type MapUpdateInput = {
  displayName: string;
  description: string;
  visibility: MapVisibility;
  difficulty: CustomMap["difficulty"];
  thumbnailKey: string;
  thumbnailVariant?: number;
  autoZoomPlayRegion: boolean;
};
export type MapUploadQuota = {
  tier: "base" | "trusted" | "established";
  tierOverride?: "base" | "trusted" | "established";
  qualifiedFavorites: number;
  qualifiedMaps: number;
  accountAgeDays: number;
  nextTier?: "trusted" | "established";
  favoritesNeeded?: number;
  mapsNeeded?: number;
  daysNeeded?: number;
  maxMaps: number;
  maxActiveLocations: number;
  maxMapLocations: number;
  maxUploadsPerHour: number;
  maxUploadsPerDay: number;
  maxUploadedLocationsPerHour: number;
  currentMaps: number;
  currentActiveLocations: number;
  restrictedByModeration?: boolean;
};

function headers(accessToken: string): HeadersInit {
  return { Authorization: `Bearer ${accessToken}` };
}

export async function listMaps(config: RuntimeConfig, accessToken: string | undefined, input: { scope: MapScope; sort?: MapSort; search?: string }): Promise<CustomMap[]> {
  const params = new URLSearchParams({ scope: input.scope });
  if (input.sort) params.set("sort", input.sort);
  const search = input.search?.trim();
  if (search) params.set("search", search);
  return expectJSON(await apiFetch(config, `/v1/maps?${params}`, { headers: authHeaders(accessToken) }), "Map request failed");
}

export async function getMap(config: RuntimeConfig, accessToken: string | undefined, mapId: string): Promise<MapDetails> {
  return expectJSON(await apiFetch(config, `/v1/maps/${encodeURIComponent(mapId)}`, { headers: authHeaders(accessToken) }), "Map request failed");
}

export async function getMapUploadQuota(config: RuntimeConfig, accessToken: string): Promise<MapUploadQuota> {
  return expectJSON(await apiFetch(config, "/v1/maps/quota", { headers: headers(accessToken) }), "Map quota request failed");
}

export async function createMap(config: RuntimeConfig, accessToken: string, input: { file: File; displayName: string; description: string; visibility: MapVisibility; difficulty: "easy" | "normal" | "hard"; thumbnailKey: string; thumbnailVariant?: number }): Promise<CustomMap> {
  const body = new FormData();
  body.set("file", input.file);
  body.set("displayName", input.displayName);
  body.set("description", input.description);
  body.set("visibility", input.visibility);
  body.set("difficulty", input.difficulty);
  body.set("thumbnailKey", input.thumbnailKey);
  body.set("thumbnailVariant", String(input.thumbnailVariant || 1));
  return expectJSON(await apiFetch(config, "/v1/maps", { method: "POST", headers: headers(accessToken), body }), "Map request failed");
}

export async function updateMap(config: RuntimeConfig, accessToken: string, mapId: string, input: MapUpdateInput): Promise<CustomMap> {
  return expectJSON(await apiFetch(config, `/v1/maps/${encodeURIComponent(mapId)}`, {
    method: "PATCH",
    headers: mergeHeaders(headers(accessToken), { "Content-Type": "application/json" }),
    body: JSON.stringify({
      displayName: input.displayName,
      description: input.description,
      visibility: input.visibility,
      difficulty: input.difficulty,
      thumbnailKey: input.thumbnailKey,
      thumbnailVariant: input.thumbnailVariant || 1,
      autoZoomPlayRegion: input.autoZoomPlayRegion,
    }),
  }), "Map request failed");
}

export async function replaceMapLocations(config: RuntimeConfig, accessToken: string, mapId: string, file: File): Promise<CustomMap> {
  const body = new FormData();
  body.set("file", file);
  return expectJSON(await apiFetch(config, `/v1/maps/${encodeURIComponent(mapId)}/locations`, { method: "PUT", headers: headers(accessToken), body }), "Map request failed");
}

export async function archiveMap(config: RuntimeConfig, accessToken: string, mapId: string): Promise<void> {
  const response = await apiFetch(config, `/v1/maps/${encodeURIComponent(mapId)}`, { method: "DELETE", headers: headers(accessToken) });
  if (!response.ok) throw new Error((await response.text()) || "Could not delete map");
}

export async function publishMap(config: RuntimeConfig, accessToken: string, mapId: string): Promise<CustomMap> {
  return expectJSON(await apiFetch(config, `/v1/maps/${encodeURIComponent(mapId)}/publish`, { method: "POST", headers: headers(accessToken) }), "Map request failed");
}

export async function setMapOfficial(config: RuntimeConfig, accessToken: string, mapId: string, official: boolean): Promise<CustomMap> {
  return expectJSON(await apiFetch(config, `/v1/maps/${encodeURIComponent(mapId)}/official`, { method: official ? "POST" : "DELETE", headers: headers(accessToken) }), "Map request failed");
}

export type GameplayMapRole = "ranked_moving" | "ranked_nmpz" | "singleplayer_moving" | "singleplayer_nmpz";

export async function setGameplayMapRole(config: RuntimeConfig, accessToken: string, mapId: string, role: GameplayMapRole): Promise<CustomMap> {
  return expectJSON(await apiFetch(config, `/v1/maps/${encodeURIComponent(mapId)}/roles/${encodeURIComponent(role)}`, { method: "POST", headers: headers(accessToken) }), "Map request failed");
}

export async function setMapFavorite(config: RuntimeConfig, accessToken: string, mapId: string, favorite: boolean): Promise<CustomMap> {
  return expectJSON(await apiFetch(config, `/v1/maps/${encodeURIComponent(mapId)}/favorite`, { method: favorite ? "POST" : "DELETE", headers: headers(accessToken) }), "Map request failed");
}

export async function createMapComment(config: RuntimeConfig, accessToken: string, mapId: string, input: { body: string; parentId?: string }): Promise<MapComment> {
  return expectJSON(await apiFetch(config, `/v1/maps/${encodeURIComponent(mapId)}/comments`, {
    method: "POST",
    headers: mergeHeaders(headers(accessToken), { "Content-Type": "application/json" }),
    body: JSON.stringify(input),
  }), "Map request failed");
}

export async function deleteMapComment(config: RuntimeConfig, accessToken: string, mapId: string, commentId: string): Promise<void> {
  const response = await apiFetch(config, `/v1/maps/${encodeURIComponent(mapId)}/comments/${encodeURIComponent(commentId)}`, { method: "DELETE", headers: headers(accessToken) });
  if (!response.ok) throw new Error((await response.text()) || "Could not delete comment");
}

export async function setMapCommentLike(config: RuntimeConfig, accessToken: string, mapId: string, commentId: string, liked: boolean): Promise<MapComment> {
  return expectJSON(await apiFetch(config, `/v1/maps/${encodeURIComponent(mapId)}/comments/${encodeURIComponent(commentId)}/like`, {
    method: liked ? "POST" : "DELETE",
    headers: headers(accessToken),
  }), "Could not update comment like");
}

export async function validateMapFile(file: File, maxLocations = 1_000_000): Promise<number> {
  if (file.size > 128 * 1024 * 1024) throw new Error("Map file exceeds 128 MiB");
  const raw = JSON.parse(await file.text());
  const locations = Array.isArray(raw) ? raw : raw && typeof raw === "object" && Array.isArray(raw.customCoordinates) ? raw.customCoordinates : null;
  if (!locations) throw new Error("Map JSON must be an array or a map-making.app export");
  if (locations.length > maxLocations) throw new Error(`Map exceeds ${maxLocations.toLocaleString()} locations for your tier`);
  const coordinates = new Set<string>();
  const panos = new Set<string>();
  let valid = 0;
  for (const item of locations) {
    if (!item || typeof item !== "object") continue;
    const lat = Number(item.lat);
    const lng = Number(item.lng);
    if (!Number.isFinite(lat) || !Number.isFinite(lng) || lat < -90 || lat > 90 || lng < -180 || lng > 180) continue;
    const coordinate = `${lat.toFixed(8)}:${lng.toFixed(8)}`;
    const extra = item.extra && typeof item.extra === "object" ? item.extra : undefined;
    const pano =
      typeof item.panoId === "string" && item.panoId.trim()
        ? item.panoId.trim()
        : typeof extra?.panoId === "string"
          ? extra.panoId.trim()
          : "";
    if (coordinates.has(coordinate) || (pano && panos.has(pano))) continue;
    coordinates.add(coordinate);
    if (pano) panos.add(pano);
    valid += 1;
  }
  if (valid < 5) throw new Error("Map requires at least 5 unique valid locations");
  return valid;
}
