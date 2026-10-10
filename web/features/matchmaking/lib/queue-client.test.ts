import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createRuntimeConfigFixture } from '../../../test/runtime-config.fixture';
import type { AuthSessionSnapshot } from '../../auth/session';
import { streamQueue, type QueueEvent } from './queue-client';

// Each socket the client opens, scripted by the test: what the server sends, then whether it closes.
type Script = (ws: FakeSocket) => void;

class FakeSocket {
  static scripts: Script[] = [];
  static opened = 0;
  onerror: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onmessage: ((evt: { data: string }) => void) | null = null;

  constructor(public url: string) {
    FakeSocket.opened++;
    const script = FakeSocket.scripts.shift();
    if (!script) throw new Error('no script for this socket');
    queueMicrotask(() => script(this));
  }

  send(type: string, payload: object = {}) {
    this.onmessage?.({ data: JSON.stringify({ type, payload }) });
  }

  close() {
    this.onclose?.();
  }

  fail() {
    this.onerror?.();
    this.onclose?.();
  }
}

const session = { accessToken: 'token' } as AuthSessionSnapshot;
const found: Script = (ws) => {
  ws.send('queue_status', { status: 'queueing' });
  ws.send('match_found', { matchId: 'm1', kind: 'ranked_duel', status: 'starting' });
  ws.close();
};
const restarting: Script = (ws) => {
  ws.send('queue_status', { status: 'queueing' });
  ws.send('queue_reconnect');
  ws.close();
};
const unreachable: Script = (ws) => ws.fail();

describe('streamQueue', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    FakeSocket.scripts = [];
    FakeSocket.opened = 0;
    vi.stubGlobal('WebSocket', FakeSocket);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  async function run(scripts: Script[]) {
    FakeSocket.scripts = scripts;
    const events: QueueEvent[] = [];
    const done = streamQueue(createRuntimeConfigFixture(), session, new AbortController().signal, ['moving'], (e) => events.push(e));
    const settled = done.then(
      () => 'resolved',
      (err: Error) => err.message
    );
    await vi.runAllTimersAsync();
    return { result: await settled, events };
  }

  it('reconnects when a restarting server asks, and keeps queueing', async () => {
    const { result, events } = await run([restarting, unreachable, found]);
    expect(result).toBe('resolved');
    expect(FakeSocket.opened).toBe(3);
    expect(events.filter((e) => e.type === 'queue_error')).toHaveLength(0);
    expect(events.at(-1)).toMatchObject({ type: 'match_found', matchId: 'm1' });
  });

  it('gives up after six tries that reach no server', async () => {
    const { result } = await run(Array(6).fill(unreachable));
    expect(result).toBe('Queue unavailable');
    expect(FakeSocket.opened).toBe(6);
  });

  it('still cancels when a server closes without being asked to', async () => {
    const { result } = await run([
      (ws) => {
        ws.send('queue_status', { status: 'queueing' });
        ws.close();
      }
    ]);
    expect(result).toBe('Search cancelled');
  });
});
