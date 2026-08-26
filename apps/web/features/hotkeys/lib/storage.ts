import { DEFAULT_HOTKEY_PREFERENCES } from "../model/defaults";
import type { HotkeyAction, HotkeyPreferences, KeyBinding } from "../model/types";

export const HOTKEY_PREFERENCES_STORAGE_KEY = "geoduels.preferences.v1";
const LEGACY_CHAT_MUTED_STORAGE_KEY = "geoduels.chatMuted";

function validBinding(value: unknown): value is KeyBinding {
  if (!value || typeof value !== "object") return false;
  const binding = value as Partial<KeyBinding>;
  return typeof binding.code === "string" && binding.code.length > 0 && binding.code.length < 40;
}

export function normalizePreferences(value: unknown): HotkeyPreferences {
  const input = value && typeof value === "object" ? value as Partial<HotkeyPreferences> : {};
  const rawBindings = input.bindings && typeof input.bindings === "object" ? input.bindings : {};
  const bindings = { ...DEFAULT_HOTKEY_PREFERENCES.bindings };
  for (const action of Object.keys(bindings) as HotkeyAction[]) {
    const candidate = (rawBindings as Partial<Record<HotkeyAction, unknown>>)[action];
    if (Array.isArray(candidate)) bindings[action] = candidate.filter(validBinding).slice(0, 2);
  }
  return {
    version: 1,
    bindings,
    audioMuted: typeof input.audioMuted === "boolean" ? input.audioMuted : DEFAULT_HOTKEY_PREFERENCES.audioMuted,
    sfxMuted: typeof input.sfxMuted === "boolean" ? input.sfxMuted : DEFAULT_HOTKEY_PREFERENCES.sfxMuted,
  };
}

export function loadLocalPreferences(): HotkeyPreferences {
  if (typeof window === "undefined") return normalizePreferences(null);
  try {
    const raw = window.localStorage.getItem(HOTKEY_PREFERENCES_STORAGE_KEY);
    const loaded = raw ? normalizePreferences(JSON.parse(raw)) : normalizePreferences(null);
    if (!raw && window.localStorage.getItem(LEGACY_CHAT_MUTED_STORAGE_KEY) === "true") loaded.audioMuted = true;
    return loaded;
  } catch {
    return normalizePreferences(null);
  }
}

export function saveLocalPreferences(preferences: HotkeyPreferences) {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(HOTKEY_PREFERENCES_STORAGE_KEY, JSON.stringify(preferences));
    window.localStorage.removeItem(LEGACY_CHAT_MUTED_STORAGE_KEY);
  } catch {
    // Preferences remain active for this tab when storage is unavailable.
  }
}
