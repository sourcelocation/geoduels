import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { useState, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useHotkey } from "../hooks/use-hotkey";
import { HOTKEY_PREFERENCES_STORAGE_KEY } from "../lib/storage";
import { setSfxMuted } from "../../../lib/audio/sfx-preferences";
import { HotkeyProvider } from "./HotkeyProvider";

function renderHotkeys(ui: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      {ui}
    </QueryClientProvider>,
  );
}

function Harness({ run }: { run: () => void }) {
  const [enabled, setEnabled] = useState(true);
  useHotkey({ action: "gameplay.primary", scope: "gameplay", enabled, run });
  return (
    <>
      <input aria-label="Message" />
      <button type="button" onClick={() => setEnabled(false)}>Disable</button>
    </>
  );
}

describe("HotkeyProvider", () => {
  afterEach(() => {
    cleanup();
    setSfxMuted(false);
  });

  beforeEach(() => {
    const values = new Map<string, string>();
    Object.defineProperty(window, "localStorage", {
      configurable: true,
      value: {
        clear: () => values.clear(),
        getItem: (key: string) => values.get(key) ?? null,
        removeItem: (key: string) => values.delete(key),
        setItem: (key: string, value: string) => values.set(key, value),
      },
    });
    window.localStorage.clear();
  });

  it("dispatches an enabled action and suppresses repeats and typing", () => {
    const run = vi.fn();
    const view = renderHotkeys(<HotkeyProvider><Harness run={run} /></HotkeyProvider>);

    fireEvent.keyDown(window, { code: "Space" });
    fireEvent.keyDown(window, { code: "Space", repeat: true });
    fireEvent.keyDown(view.getByRole("textbox", { name: "Message" }), { code: "Space" });
    expect(run).toHaveBeenCalledTimes(1);

    fireEvent.click(view.getByRole("button", { name: "Disable" }));
    fireEvent.keyDown(window, { code: "Space" });
    expect(run).toHaveBeenCalledTimes(1);
  });

  it("loads remapped bindings from local preferences", () => {
    window.localStorage.setItem(HOTKEY_PREFERENCES_STORAGE_KEY, JSON.stringify({
      version: 1,
      bindings: { "gameplay.primary": [{ code: "KeyG" }] },
      audioMuted: false,
    }));
    const run = vi.fn();
    renderHotkeys(<HotkeyProvider><Harness run={run} /></HotkeyProvider>);

    fireEvent.keyDown(window, { code: "Space" });
    fireEvent.keyDown(window, { code: "KeyG" });
    expect(run).toHaveBeenCalledTimes(1);
  });

  it("shows the consolidated settings tabs and captures a binding globally", () => {
    renderHotkeys(<HotkeyProvider><Harness run={() => undefined} /></HotkeyProvider>);

    fireEvent.keyDown(window, { code: "Slash", shiftKey: true });
    expect(screen.getAllByRole("button", { pressed: false }).map((button) => button.textContent)).toEqual([
      "Privacy",
      "Audio",
      "Account",
    ]);

    fireEvent.click(screen.getByRole("button", { name: "Change Guess / continue shortcut" }));
    expect(screen.getByText("Press a key…")).toBeInTheDocument();
    fireEvent.keyDown(window, { code: "KeyG" });
    expect(screen.getByRole("button", { name: "Change Guess / continue shortcut" })).toHaveTextContent("G");
  });

  it("persists the sound-effects toggle", () => {
    renderHotkeys(<HotkeyProvider><Harness run={() => undefined} /></HotkeyProvider>);

    fireEvent.keyDown(window, { code: "Slash", shiftKey: true });
    fireEvent.click(screen.getByRole("button", { name: "Audio" }));
    const toggle = screen.getByRole("switch", { name: "Enable sound effects" });
    expect(toggle).toBeChecked();

    fireEvent.click(toggle);

    expect(toggle).not.toBeChecked();
    expect(JSON.parse(window.localStorage.getItem(HOTKEY_PREFERENCES_STORAGE_KEY) || "{}")).toMatchObject({ sfxMuted: true });
  });
});
