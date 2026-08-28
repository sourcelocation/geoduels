import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import AppModalShell from "../AppModalShell";
import { dismissAllModals } from "../modal-dismissal";

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

  it("lets the shared coordinator await each modal's independent exit", async () => {
    const firstClose = vi.fn();
    const secondClose = vi.fn();
    render(
      <>
        <AppModalShell title="First" onClose={firstClose}>First content</AppModalShell>
        <AppModalShell title="Second" onClose={secondClose}>Second content</AppModalShell>
      </>,
    );

    const dismissal = dismissAllModals();
    expect(firstClose).not.toHaveBeenCalled();
    expect(secondClose).not.toHaveBeenCalled();
    await dismissal;

    expect(firstClose).toHaveBeenCalledTimes(1);
    expect(secondClose).toHaveBeenCalledTimes(1);
  });

  it("can animate a non-dismissible modal during a coordinated shutdown", async () => {
    render(<AppModalShell title="Required">Required content</AppModalShell>);

    await dismissAllModals();

    expect(screen.getByRole("dialog", { name: "Required" }).parentElement).toHaveStyle({ pointerEvents: "none" });
  });
});
