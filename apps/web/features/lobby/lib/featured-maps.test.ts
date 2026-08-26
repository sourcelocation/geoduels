import { describe, expect, it } from "vitest";

import type { CustomMap } from "../../maps/lib/maps-client";
import { selectFeaturedOfficialMaps } from "./featured-maps";

function map(id: string, overrides: Partial<CustomMap> = {}): CustomMap {
  return {
    id,
    displayName: id,
    visibility: "public",
    status: "ready",
    difficulty: "normal",
    thumbnailVariant: 1,
    thumbnailKey: "generic/variant-1",
    locationCount: 100,
    system: true,
    official: true,
    playCount: 0,
    favoriteCount: 0,
    commentCount: 0,
    trendingScore: 0,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

describe("selectFeaturedOfficialMaps", () => {
  it("keeps only ready official maps that are not assigned to ranked gameplay", () => {
    const result = selectFeaturedOfficialMaps(
      [
        map("eligible"),
        map("community", { official: false, system: false }),
        map("processing", { status: "processing" }),
        map("moving", { modeMoving: true }),
        map("no-move", { modeNoMove: true }),
        map("nmpz", { modeNmpz: true }),
      ],
      10,
      () => 0.5,
    );

    expect(result.map((item) => item.id)).toEqual(["eligible"]);
  });

  it("samples without replacement and honors the requested limit", () => {
    const values = [0.2, 0.9, 0.5];
    let index = 0;
    const result = selectFeaturedOfficialMaps(
      [map("small"), map("large", { locationCount: 500_000 }), map("other")],
      2,
      () => values[index++],
    );

    expect(result).toHaveLength(2);
    expect(new Set(result.map((item) => item.id)).size).toBe(2);
    expect(result[0].id).toBe("large");
  });
});
