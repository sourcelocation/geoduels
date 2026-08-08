import { useEffect } from "react";

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  const tag = target.tagName.toLowerCase();
  return (
    tag === "input" ||
    tag === "textarea" ||
    tag === "select" ||
    target.isContentEditable
  );
}

function isFullscreenSupported(): boolean {
  return (
    typeof document !== "undefined" &&
    typeof document.documentElement.requestFullscreen === "function"
  );
}

export function useFullscreenShortcut() {
  useEffect(() => {
    if (!isFullscreenSupported()) return;

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.code !== "KeyF" || event.repeat) return;
      if (event.ctrlKey || event.metaKey || event.altKey || event.shiftKey) return;
      if (isEditableTarget(event.target)) return;

      event.preventDefault();

      if (document.fullscreenElement) {
        void document.exitFullscreen();
      } else {
        void document.documentElement.requestFullscreen();
      }
    };

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);
}