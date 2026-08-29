import { afterEach, describe, expect, it, vi } from "vitest";
import { createRuntimeConfigFixture } from "../../test/runtime-config.fixture";
import { AuthGateway } from "./auth-gateway";
import { requestBootstrap, requestGuestSession, requestRefreshSession, type AppBootstrapPayload, type AuthSessionPayload } from "./lib/auth-client";

vi.mock("./lib/auth-client", () => ({ requestBootstrap: vi.fn(), requestRefreshSession: vi.fn(), requestGuestSession: vi.fn(), requestLogout: vi.fn() }));
const deferred = <T,>() => { let resolve!: (value: T) => void; const promise = new Promise<T>((r) => { resolve = r; }); return { promise, resolve }; };
const registered = (id: string, nicknameRequired = false): AuthSessionPayload => ({ accessToken: `token-${id}`, nicknameRequired, user: { id } });
const boot = (auth: AuthSessionPayload | null): AppBootstrapPayload => ({
  version: 1,
  auth,
  viewer: auth ? { id: auth.user?.id || "", displayName: auth.user?.id || "", accountType: auth.user?.isGuest ? "guest" : "registered", mmr: 0, gamesPlayed: 0, wins: 0, rankedGamesPlayed: 0, rankedWins: 0 } : null,
  preferences: null,
  activity: { activeMatch: null, notifications: [] },
  global: { onlinePlayers: 0, maintenance: { phase: "normal" } },
});
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
  it("applies live notification deltas onto bootstrap activity", async () => {
    vi.mocked(requestBootstrap).mockResolvedValueOnce(boot(registered("one")));
    const gateway = new AuthGateway(createRuntimeConfigFixture());
    await gateway.bootstrap();
    gateway.applyNotification({ id: 7, type: "friend_request_received", payload: {}, createdAt: "2026-08-28T00:00:00Z" });
    expect(gateway.getBootstrapPayload()?.activity.notifications[0]?.id).toBe(7);
    gateway.applyNotificationRead(7);
    expect(gateway.getBootstrapPayload()?.activity.notifications[0]?.readAt).toBeTruthy();
  });

  it("retains anonymous shell bootstrap state", async () => {
    vi.mocked(requestBootstrap).mockResolvedValueOnce(boot(null));
    const gateway = new AuthGateway(createRuntimeConfigFixture());
    await gateway.bootstrap();
    expect(gateway.getSnapshot()).toBeNull();
    expect(gateway.getBootstrapPayload()?.version).toBe(1);
  });
  it("deduplicates overlapping restores", async () => {
    const pending = deferred<AppBootstrapPayload>(); vi.mocked(requestBootstrap).mockReturnValueOnce(pending.promise);
    const gateway = new AuthGateway(createRuntimeConfigFixture()); const one = gateway.bootstrap(); const two = gateway.bootstrap();
    expect(requestBootstrap).toHaveBeenCalledTimes(1); pending.resolve(boot(registered("one"))); await expect(one).resolves.toEqual(await two);
  });
  it("discards an anonymous restore that finishes after OAuth", async () => {
    const old = deferred<AppBootstrapPayload>(); const fresh = deferred<AppBootstrapPayload>();
    vi.mocked(requestBootstrap).mockReturnValueOnce(old.promise).mockReturnValueOnce(fresh.promise);
    const gateway = new AuthGateway(createRuntimeConfigFixture()); void gateway.bootstrap(); const oauth = gateway.oauthCompleted(); fresh.resolve(boot(registered("google", true))); await oauth; old.resolve(boot(null));
    await Promise.resolve(); expect(gateway.getSnapshot()?.userId).toBe("google"); expect(gateway.getSnapshot()?.nicknameRequired).toBe(true);
  });
  it("discards a guest result that finishes after OAuth", async () => {
    const guest = deferred<AuthSessionPayload>(); vi.mocked(requestGuestSession).mockReturnValueOnce(guest.promise); vi.mocked(requestBootstrap).mockResolvedValueOnce(boot(null)).mockResolvedValueOnce(boot(registered("google")));
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
    vi.mocked(requestBootstrap).mockResolvedValueOnce(boot(registered("new"))).mockResolvedValueOnce(boot(registered("new")));

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
    vi.mocked(requestBootstrap).mockResolvedValueOnce(boot(registered("remote")));

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
