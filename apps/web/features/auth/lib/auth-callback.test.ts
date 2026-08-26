import { describe, expect, it } from "vitest";
import { clearAuthCallbackParams, readAuthCallback } from "./auth-callback";

describe("auth callback helpers", () => {
  it("reads success and error callback kinds from oauth return params", () => {
    expect(readAuthCallback(new URL("http://localhost:3000/?auth=success")).kind).toBe(
      "success",
    );
    expect(
      readAuthCallback(new URL("http://localhost:3000/?googleAuth=success")).kind,
    ).toBe("success");
    expect(
      readAuthCallback(
        new URL("http://localhost:3000/?auth=error&authError=nope"),
      ),
    ).toEqual({ kind: "error", errorMessage: "nope" });
    expect(readAuthCallback(new URL("http://localhost:3000/")).kind).toBeNull();
  });

  it("clears oauth return params in place", () => {
    const url = new URL(
      "http://localhost:3000/party/ABC?auth=success&provider=google&keep=1",
    );
    clearAuthCallbackParams(url);
    expect(url.searchParams.get("auth")).toBeNull();
    expect(url.searchParams.get("provider")).toBeNull();
    expect(url.searchParams.get("keep")).toBe("1");
  });
});
