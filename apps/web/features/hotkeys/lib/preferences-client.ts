import type { RuntimeConfig } from "../../../lib/runtime-config";
import { apiFetch, authHeaders, mergeHeaders, readError } from "../../../lib/http";
import type { HotkeyPreferences } from "../model/types";

export async function fetchPreferences(config: RuntimeConfig, accessToken: string) {
  const response = await apiFetch(config, "/v1/me/preferences", {
    headers: authHeaders(accessToken),
  });
  if (!response.ok) throw new Error(await readError(response, "Could not load preferences"));
  return response.json() as Promise<{ preferences: HotkeyPreferences; revision: number }>;
}

export async function patchPreferences(
  config: RuntimeConfig,
  accessToken: string,
  preferences: HotkeyPreferences,
  revision: number,
) {
  const response = await apiFetch(config, "/v1/me/preferences", {
    method: "PATCH",
    headers: mergeHeaders({ "content-type": "application/json" }, authHeaders(accessToken)),
    body: JSON.stringify({ preferences, revision }),
  });
  if (!response.ok) throw new Error(await readError(response, "Could not save preferences"));
  return response.json() as Promise<{ preferences: HotkeyPreferences; revision: number }>;
}

