import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { DiscreteSlider } from "../DiscreteSlider";

const options = [
  { value: "none", label: "None" },
  { value: "15", label: "15s" },
  { value: "30", label: "30s" },
] as const;

afterEach(cleanup);

describe("DiscreteSlider", () => {
  it("exposes the selected label and emits the corresponding option", () => {
    const onValueChange = vi.fn();
    render(
      <DiscreteSlider
        aria-label="Guess pressure"
        options={options}
        value="15"
        onValueChange={onValueChange}
      />,
    );

    const slider = screen.getByRole("slider", { name: "Guess pressure" });
    expect(slider).toHaveAttribute("aria-valuetext", "15s");
    fireEvent.change(slider, { target: { value: "2" } });
    expect(onValueChange).toHaveBeenCalledWith("30");
  });

  it("allows a labeled mark to select its value", () => {
    const onValueChange = vi.fn();
    render(
      <DiscreteSlider
        aria-label="Guess pressure"
        options={options}
        value="none"
        onValueChange={onValueChange}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Set guess pressure to 15s" }));
    expect(onValueChange).toHaveBeenCalledWith("15");
  });
});
