import { afterEach, describe, expect, it, vi } from "vitest";
import { createMap, listMaps, setMapCommentLike, updateMap, validateMapFile } from "./maps-client";
import type { RuntimeConfig } from "../../../lib/runtime-config";

function jsonFile(value: unknown) {
  const text = JSON.stringify(value);
  return {
    size: text.length,
    text: async () => text,
  } as File;
}

function uploadFile(value: unknown) {
  const text = JSON.stringify(value);
  return new Blob([text], { type: "application/json" }) as File;
}

const config = { apiURL: "https://api.test" } as RuntimeConfig;
const countryThumbnailRef = ["countries", "japan"].join("/");

afterEach(() => {
  vi.restoreAllMocks();
});

describe("validateMapFile", () => {
  it("accepts map-making.app exports with nested pano ids", async () => {
    const locations = [
      ["3P5a5OtPyfh9ByBzuHANrg", -0.15796175210211638, 37.7503211982208],
      ["Jd2sr079nrj3lhQ9Ab_kEw", 6.428595321124186, -1.4132091930831705],
      ["b184OQ0GzootvmVPP6CuAg", 6.5462559963431595, 0.25167556026892035],
      ["cQ5UcYcmtkqA3oVIvOR8jA", 6.2225031210101065, -1.383655971662562],
      ["exGCRe5MBhjvC1PBBUzWLg", 8.555462932718319, -2.2134015863200154],
    ].map(([panoId, lat, lng]) => ({
      lat,
      lng,
      heading: 88,
      pitch: 0,
      zoom: 0,
      panoId: null,
      countryCode: null,
      stateCode: null,
      extra: { panoId, panoDate: "2025-02" },
    }));

    await expect(validateMapFile(jsonFile({ name: "test", customCoordinates: locations }))).resolves.toBe(5);
  });

  it("uses the creator tier location cap", async () => {
    const locations = Array.from({ length: 6 }, (_, index) => ({ lat: index, lng: index }));
    await expect(validateMapFile(jsonFile(locations), 5)).rejects.toThrow("Map exceeds 5 locations for your tier");
  });
});

describe("listMaps", () => {
  it("sends trimmed search and trending sort", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: async () => [],
    } as Response);

    await expect(listMaps(config, undefined, { scope: "community", sort: "trending", search: "  source world  " })).resolves.toEqual([]);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const url = new URL(String(fetchMock.mock.calls[0][0]));
    expect(url.origin + url.pathname).toBe("https://api.test/v1/maps");
    expect(url.searchParams.get("scope")).toBe("community");
    expect(url.searchParams.get("sort")).toBe("trending");
    expect(url.searchParams.get("search")).toBe("source world");
  });
});

describe("createMap", () => {
  it("sends the selected visibility with uploads", async () => {
    const map = {
      id: "map-1",
      displayName: "Hidden Corners",
      visibility: "unlisted",
      status: "ready",
      difficulty: "normal",
      thumbnailVariant: 1,
      thumbnailKey: "generic/variant-1",
      locationCount: 5,
      system: false,
      playCount: 0,
      favoriteCount: 0,
      commentCount: 0,
      trendingScore: 0,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: async () => map,
    } as Response);

    await expect(createMap(config, "token", {
      file: uploadFile([{ lat: 1, lng: 1 }]),
      displayName: "Hidden Corners",
      description: "",
      visibility: "unlisted",
      difficulty: "normal",
      thumbnailKey: "generic/variant-1",
    })).resolves.toEqual(map);

    const body = fetchMock.mock.calls[0][1]?.body;
    expect(body).toBeInstanceOf(FormData);
    expect((body as FormData).get("visibility")).toBe("unlisted");
  });
});

describe("updateMap", () => {
  it("patches map metadata as JSON", async () => {
    const map = {
      id: "map-1",
      displayName: "Better Corners",
      description: "Updated route notes",
      visibility: "public",
      status: "ready",
      difficulty: "hard",
      thumbnailVariant: 1,
      thumbnailKey: countryThumbnailRef,
      locationCount: 5,
      system: false,
      playCount: 0,
      favoriteCount: 0,
      commentCount: 0,
      trendingScore: 0,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: async () => map,
    } as Response);

    await expect(updateMap(config, "token", "map-1", {
      displayName: "Better Corners",
      description: "Updated route notes",
      visibility: "public",
      difficulty: "hard",
      thumbnailKey: countryThumbnailRef,
      thumbnailVariant: 1,
    })).resolves.toEqual(map);

    expect(fetchMock).toHaveBeenCalledWith(
      "https://api.test/v1/maps/map-1",
      expect.objectContaining({
        method: "PATCH",
        body: JSON.stringify({
          displayName: "Better Corners",
          description: "Updated route notes",
          visibility: "public",
          difficulty: "hard",
          thumbnailKey: countryThumbnailRef,
          thumbnailVariant: 1,
          autoZoomPlayRegion: false,
        }),
      }),
    );
  });
});

describe("setMapCommentLike", () => {
  it("persists a like with the comment endpoint", async () => {
    const comment = {
      id: "comment-1",
      mapId: "map-1",
      userId: "user-1",
      userDisplayName: "Player",
      body: "Nice map",
      status: "visible",
      likeCount: 1,
      liked: true,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: async () => comment,
    } as Response);

    await expect(setMapCommentLike(config, "token", "map-1", "comment-1", true)).resolves.toEqual(comment);

    expect(fetchMock).toHaveBeenCalledWith(
      "https://api.test/v1/maps/map-1/comments/comment-1/like",
      expect.objectContaining({ method: "POST" }),
    );
  });
});
