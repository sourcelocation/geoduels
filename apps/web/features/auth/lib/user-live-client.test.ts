import { describe, expect, it } from "vitest";
import { parseLiveEvent } from "./user-live-client";

describe("parseLiveEvent", () => {
  it("accepts the closed live event set", () => {
    expect(parseLiveEvent({ type: "hello" })).toEqual({ type: "hello" });
    expect(parseLiveEvent({ type: "notification.read_all" })).toEqual({ type: "notification.read_all" });
    expect(parseLiveEvent({ type: "notification.read", notificationId: 9 })).toEqual({
      type: "notification.read",
      notificationId: 9,
    });
    expect(parseLiveEvent({
      type: "presence.patch",
      presence: { userId: "u1", presenceStatus: "online" },
    })).toEqual({
      type: "presence.patch",
      presence: { userId: "u1", presenceStatus: "online" },
    });
    expect(parseLiveEvent({ type: "unknown" })).toBeNull();
  });
});
