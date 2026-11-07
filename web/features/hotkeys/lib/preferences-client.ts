
import { apiFetch, authHeaders, mergeHeaders, readError } from "../../../lib/http";
import type { HotkeyPreferences } from "../model/types";

export async function patchPreferences(
  accessToken: string,
  preferences: HotkeyPreferences,
  revision: number,
) {
  const response = await apiFetch("/api/me/preferences", {
    method: "PATCH",
    headers: mergeHeaders({ "content-type": "application/json" }, authHeaders(accessToken)),
    body: JSON.stringify({ preferences, revision }),
  });
  if (!response.ok) throw new Error(await readError(response, "Could not save preferences"));
  return response.json() as Promise<{ preferences: HotkeyPreferences; revision: number }>;
}
