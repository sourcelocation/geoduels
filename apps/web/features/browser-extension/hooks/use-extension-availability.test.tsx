import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useExtensionAvailability } from "./use-extension-availability";

function Harness() {
  const status = useExtensionAvailability();
  return <div>{status.state}</div>;
}

describe("useExtensionAvailability", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it("detects the official extension and expires a stale handshake", () => {
    render(<Harness />);
    expect(screen.getByText("checking")).toBeInTheDocument();

    act(() => {
      window.dispatchEvent(
        new MessageEvent("message", {
          source: window,
          origin: window.location.origin,
          data: {
            source: "geoduels-extension",
            version: 1,
            extensionVersion: "0.1.3",
            type: "extension_ready",
          },
        }),
      );
    });
    expect(screen.getByText("ready")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(6_100);
    });
    expect(screen.getByText("missing")).toBeInTheDocument();
  });

  it("marks older extension releases as outdated", () => {
    render(<Harness />);

    act(() => {
      window.dispatchEvent(
        new MessageEvent("message", {
          source: window,
          origin: window.location.origin,
          data: {
            source: "geoduels-extension",
            version: 1,
            extensionVersion: "0.1.2",
            type: "extension_ready",
          },
        }),
      );
    });

    expect(screen.getByText("outdated")).toBeInTheDocument();
  });
});
