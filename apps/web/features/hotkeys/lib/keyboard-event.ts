import type { KeyBinding } from "../model/types";

export function isEditableTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) return false;
  return target.isContentEditable ||
    ["INPUT", "TEXTAREA", "SELECT"].includes(target.tagName) ||
    Boolean(target.closest('[data-hotkeys="off"]'));
}

export function eventMatchesBinding(event: KeyboardEvent, binding: KeyBinding) {
  return event.code === binding.code &&
    event.ctrlKey === Boolean(binding.ctrl) &&
    event.altKey === Boolean(binding.alt) &&
    event.shiftKey === Boolean(binding.shift) &&
    event.metaKey === Boolean(binding.meta);
}

export function bindingFromEvent(event: KeyboardEvent): KeyBinding | null {
  if (["ControlLeft", "ControlRight", "AltLeft", "AltRight", "ShiftLeft", "ShiftRight", "MetaLeft", "MetaRight"].includes(event.code)) return null;
  return {
    code: event.code,
    ctrl: event.ctrlKey || undefined,
    alt: event.altKey || undefined,
    shift: event.shiftKey || undefined,
    meta: event.metaKey || undefined,
  };
}

export function formatBinding(binding: KeyBinding) {
  const mac = typeof navigator !== "undefined" && /Mac|iPhone|iPad/.test(navigator.platform);
  const names: Record<string, string> = {
    Space: "Space", Enter: "Enter", Escape: "Esc", Slash: "/",
    Equal: "=", Minus: "-", Backquote: "`",
  };
  const key = names[binding.code] || binding.code.replace(/^Key/, "").replace(/^Digit/, "");
  return [
    binding.ctrl ? (mac ? "⌃" : "Ctrl") : "",
    binding.alt ? (mac ? "⌥" : "Alt") : "",
    binding.shift ? (mac ? "⇧" : "Shift") : "",
    binding.meta ? (mac ? "⌘" : "Meta") : "",
    key,
  ].filter(Boolean).join(mac ? "" : " + ");
}

