import { afterEach, describe, expect, it, vi } from "vitest";
import { createRuntimeConfigFixture } from "../../../test/runtime-config.fixture";
import { createParty, fetchParty, streamParty, updatePartySettings } from "./party-client";

class MockWebSocket {
  static instances: MockWebSocket[] = [];
  onerror: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  closed = false;

  constructor(public url: string) {
    MockWebSocket.instances.push(this);
  }

  close() {
    this.closed = true;
  }

  emitMessage(payload: unknown) {
    this.onmessage?.({ data: JSON.stringify(payload) });
  }
}

describe("party-client", () => {
  const originalFetch = global.fetch;
  const originalWebSocket = global.WebSocket;
  const runtimeConfig = createRuntimeConfigFixture({
    apiURL: "http://api.example.test",
    queueURL: "http://coordinator.example.test/",
  });

  afterEach(() => {
    global.fetch = originalFetch;
    global.WebSocket = originalWebSocket;
    MockWebSocket.instances = [];
    vi.restoreAllMocks();
  });

  it("sends party commands to the match coordinator", async () => {
    global.fetch = vi.fn(async () => ({
      ok: true,
      json: async () => ({
        id: "party-1",
        inviteCode: "ABC123",
        ownerUserId: "u1",
        state: "open",
        mode: "duel",
        mapScope: "world",
        members: [],
      }),
    }) as Response) as typeof fetch;

    await createParty(runtimeConfig, "access-token");
    await updatePartySettings(
      runtimeConfig,
      "party-1",
      "access-token",
      { ruleset: "moving", roundTimerMode: "none" },
      "team_duel",
    );

    expect(global.fetch).toHaveBeenNthCalledWith(
      1,
      "http://coordinator.example.test/parties",
      expect.objectContaining({ method: "POST" }),
    );
    expect(global.fetch).toHaveBeenNthCalledWith(
      2,
      "http://coordinator.example.test/parties/party-1/settings",
      expect.objectContaining({ method: "PATCH" }),
    );
  });

  it("fetches invite snapshots from the match coordinator", async () => {
    global.fetch = vi.fn(async () => ({
      ok: true,
      json: async () => ({
        id: "party-1",
        inviteCode: "ABC123",
        ownerUserId: "u1",
        state: "open",
        mode: "duel",
        mapScope: "world",
        members: [],
      }),
    }) as Response) as typeof fetch;

    await fetchParty(runtimeConfig, "ABC123");

    expect(global.fetch).toHaveBeenCalledWith(
      "http://coordinator.example.test/parties/ABC123",
    );
  });

  it("streams party websocket snapshots", async () => {
    global.WebSocket = MockWebSocket as unknown as typeof WebSocket;
    const controller = new AbortController();
    const onEvent = vi.fn();
    const ready = streamParty(
      runtimeConfig,
      {
        userId: "u1",
        accessToken: "access-token",
        nicknameRequired: false,
        nicknameInput: "",
      },
      "party-1",
      controller.signal,
      onEvent,
    );

    expect(MockWebSocket.instances).toHaveLength(1);
    expect(MockWebSocket.instances[0]?.url).toBe(
      "ws://coordinator.example.test/parties/party-1/ws?accessToken=access-token",
    );
    MockWebSocket.instances[0]?.emitMessage({
      type: "party_snapshot",
      payload: {
        id: "party-1",
        inviteCode: "ABC123",
        ownerUserId: "u1",
        state: "open",
        mode: "duel",
        mapScope: "world",
        members: [{ userId: "u1", displayName: "Player", role: "owner", connected: true }],
      },
    });

    expect(onEvent).toHaveBeenCalledWith({
      type: "party_snapshot",
      party: expect.objectContaining({
        id: "party-1",
        inviteCode: "ABC123",
        members: [expect.objectContaining({ connected: true })],
      }),
    });
    controller.abort();
    await expect(ready).rejects.toMatchObject({ name: "AbortError" });
    expect(MockWebSocket.instances[0]?.closed).toBe(true);
  });
});
