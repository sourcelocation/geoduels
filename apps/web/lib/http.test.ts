import { afterEach, describe, expect, it, vi } from "vitest";
import { apiFetchPath, apiPath } from "./http";

describe("api transport helpers", () => {
  afterEach(() => vi.unstubAllEnvs());

  it("uses same-origin relative paths when no public API origin is configured", () => {
    expect(apiPath({ apiURL: "" }, "/v1/maps")).toBe("/v1/maps");
  });

  it("keeps explicit public API origins for tests and external deployments", () => {
    expect(apiPath({ apiURL: "https://api.example.test" }, "/v1/maps")).toBe("https://api.example.test/v1/maps");
  });

  it("uses a server-only base for SSR fetches when browser API origin is same-origin", () => {
    vi.stubEnv("API_PROXY_URL", "http://api.internal:8080");
    expect(apiFetchPath({ apiURL: "" }, "/v1/maps")).toBe("http://api.internal:8080/v1/maps");
  });
});
