import { afterEach, describe, expect, it, vi } from "vitest";

describe("authGateway", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.resetModules();
  });

  // Rendering a page imports this on the server, where a channel would never be closed.
  it("opens nothing until the app starts it, and then only once", async () => {
    const channels: unknown[] = [];
    vi.stubGlobal("BroadcastChannel", class {
      constructor() {
        channels.push(this);
      }
    });
    const { authGateway } = await import("./auth-gateway");
    expect(channels).toHaveLength(0);

    authGateway.start();
    authGateway.start();
    expect(channels).toHaveLength(1);
  });
});
