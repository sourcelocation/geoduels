export const CHAT_MUTED_STORAGE_KEY = "geoduels.chatMuted";

export function isChatMuted() {
  if (typeof window === "undefined") return false;

  try {
    return window.localStorage.getItem(CHAT_MUTED_STORAGE_KEY) === "true";
  } catch {
    return false;
  }
}

export function setChatMuted(muted: boolean) {
  if (typeof window === "undefined") return;

  try {
    window.localStorage.setItem(CHAT_MUTED_STORAGE_KEY, String(muted));
  } catch {
    // Chat remains usable when browser storage is unavailable.
  }
}
