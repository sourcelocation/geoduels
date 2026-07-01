import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createRuntimeConfigFixture } from '../../../test/runtime-config.fixture';
import type { AuthSessionSnapshot } from '../../auth/session';
import { MatchController } from './match-controller';

class MockWebSocket {
  static instances: MockWebSocket[] = [];

  url: string;
  readyState = 0;
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  send = vi.fn();
  close = vi.fn(() => {
    this.readyState = 3;
    this.onclose?.();
  });

  constructor(url: string) {
    this.url = url;
    MockWebSocket.instances.push(this);
  }

  emitMessage(data: unknown) {
    this.onmessage?.({ data: JSON.stringify(data) } as MessageEvent);
  }

  emitOpen() {
    this.readyState = 1;
    this.onopen?.();
  }
}

describe('MatchController', () => {
  const originalFetch = global.fetch;
  const originalWebSocket = global.WebSocket;
  const runtimeConfig = createRuntimeConfigFixture({ queueHeartbeatIntervalMs: 5 });

  beforeEach(() => {
    MockWebSocket.instances = [];
    global.WebSocket = MockWebSocket as unknown as typeof WebSocket;
    window.WebSocket = MockWebSocket as unknown as typeof WebSocket;
  });

  afterEach(() => {
    global.fetch = originalFetch;
    global.WebSocket = originalWebSocket;
    window.WebSocket = originalWebSocket;
    vi.restoreAllMocks();
  });

  it('opens the websocket after guest bootstrap before queue assignment is parsed', async () => {
    const session: AuthSessionSnapshot = {
      userId: 'u_guest',
      accessToken: 'guest-access-token',
      nicknameRequired: false,
      nicknameInput: 'Guest'
    };

    global.fetch = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith('/queue/online')) {
        return {
          ok: true,
          json: async () => ({ online: 0 })
        } as Response;
      }
      throw new Error(`unexpected fetch: ${url}`);
    }) as typeof fetch;

    const sessionController = {
      getPlayableSession: vi.fn(async () => session),
      ensureFreshSession: vi.fn(async () => session),
      getSessionSnapshot: vi.fn(() => session),
      refreshSession: vi.fn(async () => null),
      clearAuthSession: vi.fn()
    } as any;
    const sfxController = {
      start: vi.fn(),
      destroy: vi.fn(),
      play: vi.fn(),
      playManaged: vi.fn(),
      playLoop: vi.fn(),
      stop: vi.fn()
    };

    const controller = new MatchController({ config: runtimeConfig, sessionController, sfxController });
    controller.start();

    controller.joinQueue();
    await vi.waitFor(() => expect(MockWebSocket.instances).toHaveLength(1));
    expect(MockWebSocket.instances[0]?.url).toBe('ws://localhost:8090/queue?accessToken=guest-access-token&queues=moving');
    MockWebSocket.instances[0]?.emitMessage({
      type: 'queue_status',
      payload: { status: 'queued', queuedAt: 1773355276730 }
    });
    MockWebSocket.instances[0]?.emitMessage({
      type: 'match_assigned',
      payload: {
        matchId: 'm-1773355279675',
        node: 'gameplay-node-0',
        ticket: 'guest-ticket',
        wsPath: '/ws/gameplay-node-0'
      }
    });
    await vi.waitFor(() => expect(MockWebSocket.instances).toHaveLength(2));
    expect(MockWebSocket.instances[1]?.url).toBe('ws://localhost:8092/ws/gameplay-node-0?ticket=guest-ticket');
    expect(sfxController.play).toHaveBeenCalledWith('duel-game-start');

    controller.destroy();
  });

  it('does not perform startup recovery on home load', async () => {
    const session: AuthSessionSnapshot = {
      userId: 'u_recover',
      accessToken: 'recover-access-token',
      nicknameRequired: false,
      nicknameInput: 'Recover'
    };

    global.fetch = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith('/queue/online')) {
        return Promise.resolve({
          ok: true,
          json: async () => ({ online: 0 })
        } as Response);
      }
      throw new Error(`unexpected fetch: ${url}`);
    }) as typeof fetch;

    const sessionController = {
      getPlayableSession: vi.fn(async () => session),
      ensureFreshSession: vi.fn(async () => session),
      getSessionSnapshot: vi.fn(() => session),
      refreshSession: vi.fn(async () => null),
      clearAuthSession: vi.fn()
    } as any;

    const controller = new MatchController({ config: runtimeConfig, sessionController });
    controller.start();

    expect(sessionController.ensureFreshSession).not.toHaveBeenCalled();
    expect(MockWebSocket.instances).toHaveLength(0);

    controller.destroy();
  });

  it('returns to ready when the queue expires', async () => {
    const session: AuthSessionSnapshot = {
      userId: 'u_expired',
      accessToken: 'expired-access-token',
      nicknameRequired: false,
      nicknameInput: 'Expired'
    };

    global.fetch = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith('/queue/online')) {
        return {
          ok: true,
          json: async () => ({ online: 0 })
        } as Response;
      }
      throw new Error(`unexpected fetch: ${url}`);
    }) as typeof fetch;

    const sessionController = {
      getPlayableSession: vi.fn(async () => session),
      ensureFreshSession: vi.fn(async () => session),
      getSessionSnapshot: vi.fn(() => session),
      refreshSession: vi.fn(async () => null),
      clearAuthSession: vi.fn()
    } as any;

    const controller = new MatchController({ config: runtimeConfig, sessionController });
    controller.start();
    controller.joinQueue();
    await vi.waitFor(() => expect(MockWebSocket.instances).toHaveLength(1));
    MockWebSocket.instances[0]?.emitMessage({
      type: 'queue_status',
      payload: { status: 'queued', queuedAt: 1773355276730 }
    });
    MockWebSocket.instances[0]?.emitMessage({
      type: 'queue_error',
      payload: { message: 'Queue expired. Please re-queue.' }
    });

    await vi.waitFor(() => expect(controller.getState().matchmaking.status).toBe('ready'));
    expect(controller.getState().queueError).toBe('Queue expired. Please re-queue.');

    controller.destroy();
  });

  it('connects a resolved live match immediately when given a minted ticket', async () => {
    const session: AuthSessionSnapshot = {
      userId: 'u_match',
      accessToken: 'match-access-token',
      nicknameRequired: false,
      nicknameInput: 'Match'
    };

    const sessionController = {
      getPlayableSession: vi.fn(async () => session),
      ensureFreshSession: vi.fn(async () => session),
      getSessionSnapshot: vi.fn(() => session),
      refreshSession: vi.fn(async () => null),
      clearAuthSession: vi.fn()
    } as any;

    const controller = new MatchController({ config: runtimeConfig, sessionController });
    controller.start();
    const resumed = await controller.resumeResolvedMatch({
      matchId: 'solo-123',
      node: 'gameplay-node-0',
      wsPath: '/ws/gameplay-node-0',
      ticket: 'resume-ticket'
    });

    expect(resumed).toBe(true);
    await vi.waitFor(() => expect(MockWebSocket.instances).toHaveLength(1));
    expect(MockWebSocket.instances[0]?.url).toBe('ws://localhost:8092/ws/gameplay-node-0?ticket=resume-ticket');
    expect(controller.getState().activeMatchId).toBe('solo-123');

    controller.destroy();
  });

  it('can play match-found sfx for explicit resolved assignments', async () => {
    const session: AuthSessionSnapshot = {
      userId: 'u_lobby',
      accessToken: 'lobby-access-token',
      nicknameRequired: false,
      nicknameInput: 'Lobby'
    };

    const sessionController = {
      getPlayableSession: vi.fn(async () => session),
      ensureFreshSession: vi.fn(async () => session),
      getSessionSnapshot: vi.fn(() => session),
      refreshSession: vi.fn(async () => null),
      clearAuthSession: vi.fn()
    } as any;
    const sfxController = {
      start: vi.fn(),
      destroy: vi.fn(),
      play: vi.fn(),
      playManaged: vi.fn(),
      playLoop: vi.fn(),
      stop: vi.fn()
    };

    const controller = new MatchController({ config: runtimeConfig, sessionController, sfxController });
    controller.start();
    const resumed = await controller.resumeResolvedMatch(
      {
        matchId: 'lobby-123',
        node: 'gameplay-node-0',
        wsPath: '/ws/gameplay-node-0',
        ticket: 'lobby-ticket'
      },
      { playMatchFoundSfx: true }
    );

    expect(resumed).toBe(true);
    expect(sfxController.play).toHaveBeenCalledWith('duel-game-start');

    controller.destroy();
  });

  it('accepts a lower event sequence when the snapshot belongs to a new match', async () => {
    const session: AuthSessionSnapshot = {
      userId: 'u_single',
      accessToken: 'single-access-token',
      nicknameRequired: false,
      nicknameInput: 'Single'
    };

    const sessionController = {
      getPlayableSession: vi.fn(async () => session),
      ensureFreshSession: vi.fn(async () => session),
      getSessionSnapshot: vi.fn(() => session),
      refreshSession: vi.fn(async () => null),
      clearAuthSession: vi.fn()
    } as any;

    const controller = new MatchController({ config: runtimeConfig, sessionController });
    controller.start();
    const resumed = await controller.resumeResolvedMatch({
      matchId: 'solo-new',
      node: 'gameplay-node-0',
      wsPath: '/ws/gameplay-node-0',
      ticket: 'single-ticket'
    });

    expect(resumed).toBe(true);
    await vi.waitFor(() => expect(MockWebSocket.instances).toHaveLength(1));
    const socket = MockWebSocket.instances[0]!;
    socket.emitOpen();
    socket.emitMessage({
      kind: 'event',
      type: 'match.lifecycle.v2.snapshot',
      payload: { matchId: 'solo-old', eventSequence: 3 }
    });
    socket.emitMessage({
      kind: 'event',
      type: 'match.lifecycle.v2.snapshot',
      payload: { matchId: 'solo-new', eventSequence: 2 }
    });

    expect(controller.getState().snapshot?.matchId).toBe('solo-new');
    expect(controller.getState().snapshot?.eventSequence).toBe(2);

    controller.destroy();
  });

  it('does not emit match state again for heartbeat acks after the socket is already healthy', async () => {
    const session: AuthSessionSnapshot = {
      userId: 'u_ping',
      accessToken: 'ping-access-token',
      nicknameRequired: false,
      nicknameInput: 'Ping'
    };
    const sessionController = {
      getPlayableSession: vi.fn(async () => session),
      ensureFreshSession: vi.fn(async () => session),
      getSessionSnapshot: vi.fn(() => session),
      refreshSession: vi.fn(async () => null),
      clearAuthSession: vi.fn()
    } as any;

    const controller = new MatchController({ config: runtimeConfig, sessionController });
    const listener = vi.fn();
    controller.subscribe(listener);
    controller.start();
    await controller.resumeResolvedMatch({
      matchId: 'm-ping',
      node: 'gameplay-node-0',
      wsPath: '/ws/gameplay-node-0',
      ticket: 'ping-ticket'
    });

    await vi.waitFor(() => expect(MockWebSocket.instances).toHaveLength(1));
    const socket = MockWebSocket.instances[0]!;
    socket.emitOpen();
    socket.emitMessage({
      kind: 'event',
      type: 'match.state',
      payload: { matchId: 'm-ping', eventSequence: 1 }
    });
    const callsAfterSnapshot = listener.mock.calls.length;

    socket.emitMessage({
      kind: 'ack',
      commandId: 'heartbeat-1',
      status: 'ok',
      serverTs: Date.now()
    });

    expect(listener).toHaveBeenCalledTimes(callsAfterSnapshot);
    controller.destroy();
  });

  it('applies the committed rating from an ended snapshot', async () => {
    const session: AuthSessionSnapshot = {
      userId: 'self',
      accessToken: 'rating-access-token',
      nicknameRequired: false,
      nicknameInput: 'Self'
    };
    const sessionController = {
      getPlayableSession: vi.fn(async () => session),
      ensureFreshSession: vi.fn(async () => session),
      getSessionSnapshot: vi.fn(() => session),
      getState: vi.fn(() => ({ userId: 'self' })),
      applyCommittedRating: vi.fn(),
      refreshSession: vi.fn(async () => null),
      clearAuthSession: vi.fn()
    } as any;

    const controller = new MatchController({ config: runtimeConfig, sessionController });
    controller.start();
    await controller.resumeResolvedMatch({
      matchId: 'ranked-1',
      node: 'gameplay-node-0',
      wsPath: '/ws/gameplay-node-0',
      ticket: 'rating-ticket'
    });
    await vi.waitFor(() => expect(MockWebSocket.instances).toHaveLength(1));

    MockWebSocket.instances[0]?.emitMessage({
      kind: 'event',
      type: 'match.state',
      payload: {
        matchId: 'ranked-1',
        state: 'ended',
        players: {
          self: {
            userId: 'self',
            mmr: 1025,
            ratingRd: 180,
            isGuest: false
          }
        },
        eventSequence: 10
      }
    });

    expect(sessionController.applyCommittedRating).toHaveBeenCalledWith(1025, 180);
    expect(controller.getState().lastFinalizedMatchId).toBe('ranked-1');
    controller.destroy();
  });

  it('ignores late snapshots from a socket after a replacement connection opens', async () => {
    const session: AuthSessionSnapshot = {
      userId: 'u_replace',
      accessToken: 'replace-access-token',
      nicknameRequired: false,
      nicknameInput: 'Replace'
    };

    const sessionController = {
      getPlayableSession: vi.fn(async () => session),
      ensureFreshSession: vi.fn(async () => session),
      getSessionSnapshot: vi.fn(() => session),
      refreshSession: vi.fn(async () => null),
      clearAuthSession: vi.fn()
    } as any;

    const controller = new MatchController({ config: runtimeConfig, sessionController });
    controller.start();
    await controller.resumeResolvedMatch({
      matchId: 'solo-old',
      node: 'gameplay-node-0',
      wsPath: '/ws/gameplay-node-0',
      ticket: 'old-ticket'
    });
    await vi.waitFor(() => expect(MockWebSocket.instances).toHaveLength(1));
    const oldSocket = MockWebSocket.instances[0]!;
    oldSocket.emitOpen();
    oldSocket.emitMessage({
      kind: 'event',
      type: 'match.lifecycle.v2.snapshot',
      payload: { matchId: 'solo-old', eventSequence: 8 }
    });

    await controller.resumeResolvedMatch({
      matchId: 'm-new',
      node: 'gameplay-node-0',
      wsPath: '/ws/gameplay-node-0',
      ticket: 'new-ticket'
    });
    await vi.waitFor(() => expect(MockWebSocket.instances).toHaveLength(2));
    const newSocket = MockWebSocket.instances[1]!;
    newSocket.emitOpen();
    newSocket.emitMessage({
      kind: 'event',
      type: 'match.lifecycle.v2.snapshot',
      payload: { matchId: 'm-new', eventSequence: 1 }
    });

    oldSocket.emitMessage({
      kind: 'event',
      type: 'match.lifecycle.v2.snapshot',
      payload: { matchId: 'solo-old', eventSequence: 9 }
    });

    expect(controller.getState().snapshot?.matchId).toBe('m-new');
    expect(controller.getState().snapshot?.eventSequence).toBe(1);

    controller.destroy();
  });

  it('reuses an already-bootstrapped playable session when resuming a cold route match', async () => {
    const session: AuthSessionSnapshot = {
      userId: 'u_guest',
      accessToken: 'fresh-bootstrap-token',
      nicknameRequired: false,
      nicknameInput: 'Guest'
    };

    const sessionController = {
      getPlayableSession: vi.fn(async () => session),
      ensureFreshSession: vi.fn(async () => null),
      getSessionSnapshot: vi.fn(() => session),
      refreshSession: vi.fn(async () => null),
      clearAuthSession: vi.fn()
    } as any;

    const controller = new MatchController({ config: runtimeConfig, sessionController });
    controller.start();
    const ensureCallsBeforeResume = sessionController.ensureFreshSession.mock.calls.length;
    const resumed = await controller.resumeResolvedMatch({
      matchId: 'solo-guest',
      node: 'gameplay-node-0',
      wsPath: '/ws/gameplay-node-0',
      ticket: 'guest-route-ticket'
    });

    expect(resumed).toBe(true);
    expect(sessionController.ensureFreshSession.mock.calls.length).toBe(ensureCallsBeforeResume);
    await vi.waitFor(() => expect(MockWebSocket.instances).toHaveLength(1));
    expect(MockWebSocket.instances[0]?.url).toBe('ws://localhost:8092/ws/gameplay-node-0?ticket=guest-route-ticket');

    controller.destroy();
  });

  it('ignores repeated singleplayer starts while the first start is bootstrapping a session', async () => {
    const session: AuthSessionSnapshot = {
      userId: 'u_guest',
      accessToken: 'guest-access-token',
      nicknameRequired: false,
      nicknameInput: 'Guest'
    };
    let resolveSession: (value: AuthSessionSnapshot) => void = () => {};
    const sessionPromise = new Promise<AuthSessionSnapshot>((resolve) => {
      resolveSession = resolve;
    });

    global.fetch = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith('/v1/singleplayer/session')) {
        return {
          ok: true,
          json: async () => ({
            matchId: 'solo-1',
            node: 'gameplay-node-0',
            wsPath: '/ws/gameplay-node-0',
            ticket: 'solo-ticket'
          })
        } as Response;
      }
      throw new Error(`unexpected fetch: ${url}`);
    }) as typeof fetch;

    const sessionController = {
      getPlayableSession: vi.fn(() => sessionPromise),
      ensureFreshSession: vi.fn(async () => session),
      getSessionSnapshot: vi.fn(() => session),
      refreshSession: vi.fn(async () => null),
      clearAuthSession: vi.fn()
    } as any;

    const controller = new MatchController({ config: runtimeConfig, sessionController });
    const firstStart = controller.startSingleplayer();
    const secondStart = await controller.startSingleplayer();

    expect(secondStart).toBe('');
    expect(sessionController.getPlayableSession).toHaveBeenCalledTimes(1);
    expect(controller.getState().matchmaking.status).toBe('matched_connecting');

    resolveSession(session);
    await expect(firstStart).resolves.toBe('solo-1');
    expect(global.fetch).toHaveBeenCalledTimes(1);

    controller.destroy();
  });

  it('does not perform startup recovery while route-owned reconnect is active', async () => {
    const session: AuthSessionSnapshot = {
      userId: 'u_match_route',
      accessToken: 'route-access-token',
      nicknameRequired: false,
      nicknameInput: 'Route'
    };

    global.fetch = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith('/queue/online')) {
        return {
          ok: true,
          json: async () => ({ online: 0 })
        } as Response;
      }
      throw new Error(`unexpected fetch: ${url}`);
    }) as typeof fetch;

    const sessionController = {
      getPlayableSession: vi.fn(async () => session),
      ensureFreshSession: vi.fn(async () => session),
      getSessionSnapshot: vi.fn(() => session),
      refreshSession: vi.fn(async () => null),
      clearAuthSession: vi.fn()
    } as any;

    const controller = new MatchController({ config: runtimeConfig, sessionController });
    controller.setAutoRecoverEnabled(false);
    controller.start();

    expect(sessionController.ensureFreshSession).not.toHaveBeenCalled();

    controller.destroy();
  });
});
