import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import AppModalShell from "../AppModalShell";

afterEach(cleanup);

describe("AppModalShell", () => {
  it("finishes its hide animation before notifying the parent to unmount", async () => {
    const onClose = vi.fn();
    render(
      <AppModalShell title="Settings" onClose={onClose}>
        Settings content
      </AppModalShell>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Close Settings" }));

    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog", { name: "Settings" }).parentElement).toHaveStyle({ pointerEvents: "none" });
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
  });
});
