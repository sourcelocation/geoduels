import type { HotkeyAction, HotkeyPreferences, KeyBinding } from "./types";

export const HOTKEY_ACTIONS: Array<{
  action: HotkeyAction;
  label: string;
  description: string;
  group: "Controls" | "Display" | "Audio";
  remappable?: boolean;
}> = [
  { action: "gameplay.primary", label: "Guess / continue", description: "Submit a guess, continue after a round, or replay a finished solo game.", group: "Controls" },
  { action: "chat.focus", label: "Open chat", description: "Open chat and focus its message field.", group: "Controls" },
  { action: "gameplay.resetView", label: "Return to spawn", description: "Reset Street View to the round's starting position.", group: "Controls" },
  { action: "display.fullscreen", label: "Toggle fullscreen", description: "Enter or leave browser fullscreen.", group: "Display" },
  { action: "audio.toggleMute", label: "Mute chat alerts", description: "Mute or restore incoming chat alerts.", group: "Audio" },
  { action: "help.shortcuts", label: "Open shortcut settings", description: "Show this shortcut reference.", group: "Controls", remappable: false },
];

export const DEFAULT_HOTKEY_PREFERENCES: HotkeyPreferences = {
  version: 1,
  bindings: {
    "gameplay.primary": [{ code: "Space" }],
    "display.fullscreen": [{ code: "KeyF" }],
    "chat.focus": [{ code: "Enter" }],
    "audio.toggleMute": [{ code: "KeyM" }],
    "gameplay.resetView": [{ code: "KeyR" }],
    "help.shortcuts": [{ code: "Slash", shift: true }],
  },
  audioMuted: false,
  sfxMuted: false,
};

export function bindingKey(binding: KeyBinding) {
  return [
    binding.ctrl ? "Ctrl" : "",
    binding.alt ? "Alt" : "",
    binding.shift ? "Shift" : "",
    binding.meta ? "Meta" : "",
    binding.code,
  ].filter(Boolean).join("+");
}
