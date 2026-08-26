import { afterEach, describe, expect, it, vi } from "vitest";
import { createRuntimeConfigFixture } from "../../test/runtime-config.fixture";
import { AuthGateway } from "./auth-gateway";
import { requestGuestSession, requestRefreshSession, requestSession } from "./lib/auth-client";

vi.mock("./lib/auth-client", () => ({ requestSession: vi.fn(), requestRefreshSession: vi.fn(), requestGuestSession: vi.fn(), requestLogout: vi.fn() }));
const deferred = <T,>() => { let resolve!: (value: T) => void; const promise = new Promise<T>((r) => { resolve = r; }); return { promise, resolve }; };
const registered = (id: string, nicknameRequired = false) => ({ accessToken: `token-${id}`, nicknameRequired, user: { id } });
const originalBroadcastChannel = globalThis.BroadcastChannel;
const originalLocks = navigator.locks;

class FakeBroadcastChannel {
  static channels = new Set<FakeBroadcastChannel>();
  onmessage: ((event: MessageEvent) => void) | null = null;
  closed = false;
  constructor(public readonly name: string) { FakeBroadcastChannel.channels.add(this); }
  postMessage(data: unknown) {
    FakeBroadcastChannel.channels.forEach((channel) => {
      if (channel !== this && !channel.closed) channel.onmessage?.({ data } as MessageEvent);
    });
  }
  close() { this.closed = true; FakeBroadcastChannel.channels.delete(this); }
}

afterEach(() => {
  vi.clearAllMocks();
  FakeBroadcastChannel.channels.clear();
  if (originalBroadcastChannel) globalThis.BroadcastChannel = originalBroadcastChannel;
  else Reflect.deleteProperty(globalThis, "BroadcastChannel");
  Object.defineProperty(navigator, "locks", { configurable: true, value: originalLocks });
});

describe("AuthGateway transitions", () => {
  it("deduplicates overlapping restores", async () => {
    const pending = deferred<ReturnType<typeof registered> | null>(); vi.mocked(requestSession).mockReturnValueOnce(pending.promise);
    const gateway = new AuthGateway(createRuntimeConfigFixture()); const one = gateway.bootstrap(); const two = gateway.bootstrap();
    expect(requestSession).toHaveBeenCalledTimes(1); pending.resolve(registered("one")); await expect(one).resolves.toEqual(await two);
  });
  it("discards an anonymous restore that finishes after OAuth", async () => {
    const old = deferred<ReturnType<typeof registered> | null>(); const fresh = deferred<ReturnType<typeof registered> | null>();
    vi.mocked(requestSession).mockReturnValueOnce(old.promise).mockReturnValueOnce(fresh.promise);
    const gateway = new AuthGateway(createRuntimeConfigFixture()); void gateway.bootstrap(); const oauth = gateway.oauthCompleted(); fresh.resolve(registered("google", true)); await oauth; old.resolve(null);
    await Promise.resolve(); expect(gateway.getSnapshot()?.userId).toBe("google"); expect(gateway.getSnapshot()?.nicknameRequired).toBe(true);
  });
  it("discards a guest result that finishes after OAuth", async () => {
    const guest = deferred<ReturnType<typeof registered>>(); vi.mocked(requestGuestSession).mockReturnValueOnce(guest.promise); vi.mocked(requestSession).mockResolvedValueOnce(null).mockResolvedValueOnce(registered("google"));
    const gateway = new AuthGateway(createRuntimeConfigFixture()); const pendingGuest = gateway.ensurePlayableSession(); await gateway.oauthCompleted(); guest.resolve({ ...registered("guest"), user: { id: "guest", isGuest: true } }); await pendingGuest;
    expect(gateway.getSnapshot()?.userId).toBe("google");
  });
  it("discards refresh completion after logout", async () => {
    const refresh = deferred<ReturnType<typeof registered> | null>(); vi.mocked(requestRefreshSession).mockReturnValueOnce(refresh.promise);
    const gateway = new AuthGateway(createRuntimeConfigFixture()); gateway.applyPayload(registered("one")); const pending = gateway.refresh(); await gateway.logout(); refresh.resolve(registered("one")); await pending;
    expect(gateway.getSnapshot()).toBeNull();
  });

  it("invalidates another tab and restores its new cookie state after an auth change", async () => {
    globalThis.BroadcastChannel = FakeBroadcastChannel as unknown as typeof BroadcastChannel;
    const first = new AuthGateway(createRuntimeConfigFixture());
    const second = new AuthGateway(createRuntimeConfigFixture());
    second.applyPayload(registered("old"));
    vi.mocked(requestSession).mockResolvedValueOnce(registered("new")).mockResolvedValueOnce(registered("new"));

    await first.oauthCompleted();
    await Promise.resolve();
    await Promise.resolve();

    expect(second.getSnapshot()?.userId).toBe("new");
    first.dispose();
    second.dispose();
  });

  it("broadcasts logout and closes its channel on disposal", async () => {
    globalThis.BroadcastChannel = FakeBroadcastChannel as unknown as typeof BroadcastChannel;
    const first = new AuthGateway(createRuntimeConfigFixture());
    const second = new AuthGateway(createRuntimeConfigFixture());
    second.applyPayload(registered("old"));

    await first.logout();
    expect(second.getSnapshot()).toBeNull();
    first.dispose();
    expect(FakeBroadcastChannel.channels.size).toBe(1);
    second.dispose();
    expect(FakeBroadcastChannel.channels.size).toBe(0);
  });

  it("settles a remote logout as anonymous instead of leaving it bootstrapping", () => {
    globalThis.BroadcastChannel = FakeBroadcastChannel as unknown as typeof BroadcastChannel;
    const first = new AuthGateway(createRuntimeConfigFixture());
    const second = new AuthGateway(createRuntimeConfigFixture());
    second.applyPayload(registered("old"));

    (first as any).channel.postMessage({ source: "remote", type: "logout" });

    expect(second.getSnapshot()).toBeNull();
    expect(second.isRestored()).toBe(true);
    first.dispose();
    second.dispose();
  });

  it("keeps local logout anonymous when a remote change arrives while it waits for the cookie lock", async () => {
    globalThis.BroadcastChannel = FakeBroadcastChannel as unknown as typeof BroadcastChannel;
    const gate = deferred<void>();
    Object.defineProperty(navigator, "locks", {
      configurable: true,
      value: { request: vi.fn(async (_name: string, work: () => Promise<unknown>) => { await gate.promise; return work(); }) },
    });
    const local = new AuthGateway(createRuntimeConfigFixture());
    const remote = new AuthGateway(createRuntimeConfigFixture());
    local.applyPayload(registered("old"));
    vi.mocked(requestSession).mockResolvedValueOnce(registered("remote"));

    const logout = local.logout();
    (remote as any).channel.postMessage({ source: "remote", type: "changed" });
    await Promise.resolve();
    await Promise.resolve();
    expect(local.getSnapshot()?.userId).toBe("remote");

    gate.resolve();
    await logout;
    expect(local.getSnapshot()).toBeNull();
    expect(local.isRestored()).toBe(true);
    local.dispose();
    remote.dispose();
  });
});
