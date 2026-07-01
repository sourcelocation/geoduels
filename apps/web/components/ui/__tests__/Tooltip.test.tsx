import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Tooltip, TooltipProvider } from "../Tooltip";

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("Tooltip", () => {
  it("opens after the hover delay and renders through a portal", async () => {
    vi.useFakeTimers();
    render(
      <TooltipProvider>
        <Tooltip content="Official map">
          <span aria-label="Official map">O</span>
        </Tooltip>
      </TooltipProvider>,
    );

    fireEvent.mouseEnter(screen.getByLabelText("Official map"));
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(649);
    });
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(screen.getByRole("tooltip")).toHaveTextContent("Official map");
    expect(screen.getByRole("tooltip").parentElement).toHaveAttribute("data-floating-ui-portal");
    expect(screen.getByRole("tooltip").parentElement?.parentElement).toBe(document.body);
  });

  it("opens immediately for keyboard focus and links the description", () => {
    render(
      <TooltipProvider>
        <Tooltip content="Ranked map">
          <button type="button">Ranked</button>
        </Tooltip>
      </TooltipProvider>,
    );

    fireEvent.focus(screen.getByRole("button", { name: "Ranked" }));

    const tooltip = screen.getByRole("tooltip");
    expect(tooltip).toHaveTextContent("Ranked map");
    expect(screen.getByRole("button", { name: "Ranked" })).toHaveAttribute("aria-describedby", tooltip.id);
  });
});
