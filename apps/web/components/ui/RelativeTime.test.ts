import { describe, expect, it } from "vitest";
import { formatRelativeTime } from "./RelativeTime";

describe("formatRelativeTime", () => {
  const now = Date.parse("2026-06-23T12:00:00.000Z");

  it("formats useful compact relative dates", () => {
    expect(formatRelativeTime("2026-06-23T11:55:00.000Z", now)).toBe("5m ago");
    expect(formatRelativeTime("2026-06-23T09:00:00.000Z", now)).toBe("3h ago");
    expect(formatRelativeTime("2026-06-21T12:00:00.000Z", now)).toBe("2d ago");
  });
});
