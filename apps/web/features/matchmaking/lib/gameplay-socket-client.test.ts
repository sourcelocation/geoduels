import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createRuntimeConfigFixture } from '../../../test/runtime-config.fixture';
import { GameplaySocketClient } from './gameplay-socket-client';

class MockWebSocket {
  static instances: MockWebSocket[] = [];

  readyState = 0;
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  send = vi.fn();
  close = vi.fn();

  constructor(readonly url: string) {
    MockWebSocket.instances.push(this);
  }

  emitMessage(data: unknown) {
    this.onmessage?.({ data: JSON.stringify(data) } as MessageEvent);
  }
}

describe('GameplaySocketClient', () => {
  const originalWebSocket = global.WebSocket;

  beforeEach(() => {
    MockWebSocket.instances = [];
    global.WebSocket = MockWebSocket as unknown as typeof WebSocket;
    window.WebSocket = MockWebSocket as unknown as typeof WebSocket;
  });

  afterEach(() => {
    global.WebSocket = originalWebSocket;
    window.WebSocket = originalWebSocket;
    vi.restoreAllMocks();
  });

  it('uses the websocket event serverTs as the snapshot clock sync timestamp', () => {
    const onSnapshot = vi.fn();
    const onTeamPing = vi.fn();
    const client = new GameplaySocketClient(createRuntimeConfigFixture(), {
      onOpen: vi.fn(),
      onClose: vi.fn(),
      onError: vi.fn(),
      onActivity: vi.fn(),
      onSnapshot,
      onTeamPing,
      onAckError: vi.fn(),
      onProtocolError: vi.fn()
    });

    client.connect(
      { userId: 'self', accessToken: 'access', nicknameRequired: false, nicknameInput: 'Self' },
      'node-1',
      '/ws/node-1',
      'ticket-1'
    );

    MockWebSocket.instances[0]?.emitMessage({
      kind: 'event',
      type: 'match.state',
      serverTs: 2_000,
      payload: {
        matchId: 'match-1',
        state: 'active',
        phase: 'live',
        roundPhase: 'round_live',
        phaseStartedAt: 1_000,
        phaseEndsAt: 10_000,
        roundMsLeft: 9_000,
        players: {},
        eventSequence: 1,
        serverUnixMs: 1_500
      }
    });

    expect(onSnapshot).toHaveBeenCalledWith(expect.objectContaining({ serverUnixMs: 2_000 }));
  });

  it('delivers team ping events separately from snapshots', () => {
    const onTeamPing = vi.fn();
    const client = new GameplaySocketClient(createRuntimeConfigFixture(), {
      onOpen: vi.fn(), onClose: vi.fn(), onError: vi.fn(), onActivity: vi.fn(),
      onSnapshot: vi.fn(), onTeamPing, onAckError: vi.fn(), onProtocolError: vi.fn()
    });
    client.connect({ userId: 'self', accessToken: 'access', nicknameRequired: false, nicknameInput: 'Self' }, 'node-1', '/ws/node-1', 'ticket-1');
    MockWebSocket.instances[0]?.emitMessage({ kind: 'event', type: 'team.ping', payload: { id: 'ping-1', roundId: 'round-1', senderUserId: 'mate', lat: 1, lng: 2, expiresAt: 7_000 } });
    expect(onTeamPing).toHaveBeenCalledWith(expect.objectContaining({ id: 'ping-1', senderUserId: 'mate' }));
  });

});
