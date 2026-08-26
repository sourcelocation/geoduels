export type HotkeyAction =
  | "gameplay.primary"
  | "display.fullscreen"
  | "chat.focus"
  | "audio.toggleMute"
  | "gameplay.resetView"
  | "help.shortcuts";

export type HotkeyScope = "global" | "lobby" | "gameplay" | "modal" | "capture";

export type KeyBinding = {
  code: string;
  ctrl?: boolean;
  alt?: boolean;
  shift?: boolean;
  meta?: boolean;
};

export type HotkeyPreferences = {
  version: 1;
  bindings: Record<HotkeyAction, KeyBinding[]>;
  audioMuted: boolean;
  sfxMuted: boolean;
};

export type HotkeyRegistration = {
  action: HotkeyAction;
  scope?: HotkeyScope;
  enabled?: boolean;
  allowRepeat?: boolean;
  allowWhileTyping?: boolean;
  run: (event: KeyboardEvent) => void;
};
