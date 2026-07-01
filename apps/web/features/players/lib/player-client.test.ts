import { afterEach, describe, expect, it, vi } from "vitest";
import { createRuntimeConfig } from "../../../lib/runtime-config";
import { requestPlayerMatches } from "./player-client";

describe("player client", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("passes cursor pagination parameters", async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({
      matches: [],
      nextCursor: "",
    }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    await requestPlayerMatches(createRuntimeConfig(), "Explorer", 20, "cursor-value");

    expect(fetchMock).toHaveBeenCalledWith(
      "/v1/players/Explorer/matches?limit=20&cursor=cursor-value",
      undefined,
    );
  });

  it("passes the ranked match filter", async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({
      matches: [],
      nextCursor: "",
    }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    await requestPlayerMatches(createRuntimeConfig(), "Explorer", 20, "", "ranked");

    expect(fetchMock).toHaveBeenCalledWith(
      "/v1/players/Explorer/matches?limit=20&filter=ranked",
      undefined,
    );
  });
});
