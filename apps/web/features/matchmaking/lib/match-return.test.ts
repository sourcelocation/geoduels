import { describe, expect, it } from "vitest";
import { getMatchReturnDestination } from "./match-return";

describe("getMatchReturnDestination", () => {
  it("returns a map target to its map page", () => {
    expect(getMatchReturnDestination({ kind: "map", mapId: "map-custom" })).toEqual({
      href: "/maps/map-custom",
      label: "Back to map",
    });
  });

  it("returns home even when a world map was played", () => {
    expect(getMatchReturnDestination({ kind: "home" })).toEqual({
      href: "/",
      label: "Back to lobby",
    });
  });

  it("uses the server-resolved party invite code", () => {
    expect(
      getMatchReturnDestination({
        kind: "party",
        partyId: "party-1",
        partyInviteCode: "PARTY1",
      }),
    ).toEqual({ href: "/party/PARTY1", label: "Back to party" });
  });

  it("falls back safely when the party is no longer available", () => {
    expect(getMatchReturnDestination({ kind: "party", partyId: "party-1" })).toEqual({
      href: "/",
      label: "Back to lobby",
    });
  });

  it("falls back to the lobby for standard matchmaking", () => {
    expect(getMatchReturnDestination()).toEqual({
      href: "/",
      label: "Back to lobby",
    });
  });
});
