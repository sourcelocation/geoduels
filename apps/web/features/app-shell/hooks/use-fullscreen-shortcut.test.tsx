import { cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useFullscreenShortcut } from "./use-fullscreen-shortcut";

function Harness() {
  useFullscreenShortcut();
  return null;
}

function fireKey(key: string, code = "KeyF") {
  window.dispatchEvent(
    new KeyboardEvent("keydown", {
      key,
      code,
      bubbles: true,
      cancelable: true,
    }),
  );
}

const originalDescriptors = {
  requestFullscreen: Object.getOwnPropertyDescriptor(
    document.documentElement,
    "requestFullscreen",
  ),
  exitFullscreen: Object.getOwnPropertyDescriptor(document, "exitFullscreen"),
  fullscreenElement: Object.getOwnPropertyDescriptor(
    document,
    "fullscreenElement",
  ),
};

function setFullscreenApi({
  requestFullscreen,
  exitFullscreen,
  fullscreenElement,
}: {
  requestFullscreen?: unknown;
  exitFullscreen?: unknown;
  fullscreenElement?: unknown;
}) {
  if (requestFullscreen !== undefined) {
    Object.defineProperty(document.documentElement, "requestFullscreen", {
      configurable: true,
      value: requestFullscreen,
    });
  }
  if (exitFullscreen !== undefined) {
    Object.defineProperty(document, "exitFullscreen", {
      configurable: true,
      value: exitFullscreen,
    });
  }
  if (fullscreenElement !== undefined) {
    Object.defineProperty(document, "fullscreenElement", {
      configurable: true,
      value: fullscreenElement,
    });
  }
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  if (originalDescriptors.requestFullscreen) {
    Object.defineProperty(
      document.documentElement,
      "requestFullscreen",
      originalDescriptors.requestFullscreen,
    );
  }
  if (originalDescriptors.exitFullscreen) {
    Object.defineProperty(
      document,
      "exitFullscreen",
      originalDescriptors.exitFullscreen,
    );
  }
  if (originalDescriptors.fullscreenElement) {
    Object.defineProperty(
      document,
      "fullscreenElement",
      originalDescriptors.fullscreenElement,
    );
  }
});

describe("useFullscreenShortcut", () => {
  it("requests fullscreen on F key when not in fullscreen", () => {
    const requestFullscreen = vi.fn().mockResolvedValue(undefined);
    setFullscreenApi({
      requestFullscreen,
      fullscreenElement: null,
    });

    render(<Harness />);
    fireKey("f");

    expect(requestFullscreen).toHaveBeenCalledTimes(1);
  });

  it("exits fullscreen on F key when already in fullscreen", () => {
    const requestFullscreen = vi.fn().mockResolvedValue(undefined);
    const exitFullscreen = vi.fn().mockResolvedValue(undefined);
    setFullscreenApi({
      requestFullscreen,
      exitFullscreen,
      fullscreenElement: document.documentElement,
    });

    render(<Harness />);
    fireKey("f");

    expect(exitFullscreen).toHaveBeenCalledTimes(1);
  });

  it("ignores F key when typing in an input", () => {
    const requestFullscreen = vi.fn().mockResolvedValue(undefined);
    setFullscreenApi({
      requestFullscreen,
      fullscreenElement: null,
    });

    render(<Harness />);
    const input = document.createElement("input");
    document.body.appendChild(input);
    input.dispatchEvent(
      new KeyboardEvent("keydown", {
        key: "f",
        code: "KeyF",
        bubbles: true,
        cancelable: true,
      }),
    );

    expect(requestFullscreen).not.toHaveBeenCalled();
    input.remove();
  });

  it("ignores F key when Ctrl is held", () => {
    const requestFullscreen = vi.fn().mockResolvedValue(undefined);
    setFullscreenApi({
      requestFullscreen,
      fullscreenElement: null,
    });

    render(<Harness />);
    window.dispatchEvent(
      new KeyboardEvent("keydown", {
        key: "f",
        code: "KeyF",
        ctrlKey: true,
        bubbles: true,
        cancelable: true,
      }),
    );

    expect(requestFullscreen).not.toHaveBeenCalled();
  });

  it("triggers regardless of keyboard layout (physical key position)", () => {
    const requestFullscreen = vi.fn().mockResolvedValue(undefined);
    setFullscreenApi({
      requestFullscreen,
      fullscreenElement: null,
    });

    render(<Harness />);
    // AZERTY layout: the physical F key produces "a" but has code "KeyF".
    fireKey("a", "KeyF");

    expect(requestFullscreen).toHaveBeenCalledTimes(1);
  });

  it("ignores a different physical key even if it produces 'f'", () => {
    const requestFullscreen = vi.fn().mockResolvedValue(undefined);
    setFullscreenApi({
      requestFullscreen,
      fullscreenElement: null,
    });

    render(<Harness />);
    // A different physical key (e.g. KeyA) producing "f" must not trigger.
    fireKey("f", "KeyA");

    expect(requestFullscreen).not.toHaveBeenCalled();
  });

  it("does nothing when fullscreen API is unsupported", () => {
    setFullscreenApi({
      requestFullscreen: undefined,
    });

    render(<Harness />);
    fireKey("f");
  });
});