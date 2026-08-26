import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { Button } from "../button";

afterEach(cleanup);

describe("Button", () => {
  it("shows an accessible loading indicator and disables interaction", () => {
    const onClick = vi.fn();

    render(
      <Button loading loadingLabel="Saving changes" icon={<span>Icon</span>} onClick={onClick}>
        Save
      </Button>,
    );

    const button = screen.getByRole("button", { name: /save/i });
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute("aria-busy", "true");
    expect(screen.getByRole("status", { name: "Saving changes" })).toBeInTheDocument();
    expect(screen.queryByText("Icon")).not.toBeInTheDocument();
  });

  it("renders its icon and preserves an explicitly disabled state when idle", () => {
    render(
      <Button disabled icon={<span>Icon</span>}>
        Save
      </Button>,
    );

    expect(screen.getByRole("button", { name: /save/i })).toBeDisabled();
    expect(screen.getByText("Icon")).toBeInTheDocument();
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });
});
