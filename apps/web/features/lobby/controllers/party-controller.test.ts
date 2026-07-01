import { describe, expect, it, vi } from "vitest";
import { jitteredPartyDelay } from "./party-controller";

describe("jitteredPartyDelay", () => {
  it("keeps party polling close to the base cadence with jitter", () => {
    const spy = vi.spyOn(Math, "random");
    spy.mockReturnValue(0);
    expect(jitteredPartyDelay(5000)).toBe(4000);
    spy.mockReturnValue(1);
    expect(jitteredPartyDelay(5000)).toBe(6000);
    spy.mockRestore();
  });
});
