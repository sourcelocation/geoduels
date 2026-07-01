import { describe, expect, it } from "vitest";
import { formatSeasonResetCountdown } from "../lib/season-countdown";
import { formatSeasonName } from "./LeaderboardPanel";

describe("formatSeasonResetCountdown", () => {
  it("formats days through seconds", () => {
    expect(formatSeasonResetCountdown((((2 * 24 + 3) * 60 + 4) * 60 + 5) * 1000)).toBe(
      "2d 3h 4m 5s",
    );
  });

  it("omits empty leading units", () => {
    expect(formatSeasonResetCountdown((4 * 60 + 5) * 1000)).toBe("4m 5s");
  });
});

describe("formatSeasonName", () => {
  it("formats season IDs for display", () => {
    expect(formatSeasonName("s2")).toBe("Season 2");
    expect(formatSeasonName("s3")).toBe("Season 3");
  });
});
