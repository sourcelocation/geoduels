import { apiFetch, readError } from "../../../lib/http";
import type { PlayerMatchesPage, PublicPlayerProfile } from "../types";

export async function requestPlayerProfile(
  nickname: string,
): Promise<PublicPlayerProfile> {
  const resp = await apiFetch(`/api/players/${encodeURIComponent(nickname)}`);
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load player profile"));
  }
  return resp.json();
}

export async function requestPlayerMatches(
  nickname: string,
  limit = 20,
  cursor = "",
  filter: "all" | "ranked" = "all",
): Promise<PlayerMatchesPage> {
  const query = new URLSearchParams({ limit: String(limit) });
  if (cursor) query.set("cursor", cursor);
  if (filter === "ranked") query.set("filter", "ranked");
  const resp = await apiFetch(
    `/api/players/${encodeURIComponent(nickname)}/matches?${query.toString()}`,
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load match history"));
  }
  return resp.json();
}
