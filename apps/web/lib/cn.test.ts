import { describe, expect, it } from "vitest";

import { cn } from "./cn";

describe("cn", () => {
  it("allows standard radii to override semantic radius tokens", () => {
    expect(cn("rounded-control", "rounded-2xl")).toBe("rounded-2xl");
  });

  it("allows semantic radius tokens to override standard radii", () => {
    expect(cn("rounded-2xl", "rounded-control")).toBe("rounded-control");
  });
});
