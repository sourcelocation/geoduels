import { afterEach, describe, expect, it, vi } from 'vitest';
import { createRuntimeConfigFixture } from '../../../test/runtime-config.fixture';
import { bootstrapMatchSession, fetchMatchSession, resolveMatchRoute } from './queue-client';

describe('queue-client match bootstrap', () => {
  const originalFetch = global.fetch;
  const runtimeConfig = createRuntimeConfigFixture();

  afterEach(() => {
    global.fetch = originalFetch;
    vi.restoreAllMocks();
  });

  it('parses a cookie-auth bootstrap response with a minted live ticket', async () => {
    global.fetch = vi.fn(async () => {
      return {
        ok: true,
        json: async () => ({
          auth: {
            accessToken: 'access-token',
            nicknameRequired: false,
            suggestedNickname: 'Player',
            user: {
              id: 'u_1',
              isGuest: false
            }
          },
          match: {
            status: 'live_connectable',
            matchId: 'solo-123',
            node: 'game-1',
            ticket: 'ticket-1',
            wsPath: '/ws/game-1'
          }
        })
      } as Response;
    }) as typeof fetch;

    const response = await bootstrapMatchSession(runtimeConfig, 'solo-123', new AbortController().signal);

    expect(response?.auth.accessToken).toBe('access-token');
    expect(response?.match).toEqual({
      status: 'live_connectable',
      matchId: 'solo-123',
      mode: '',
      node: 'game-1',
      ticket: 'ticket-1',
      wsPath: '/ws/game-1'
    });
  });

  it('normalizes replaced match responses from the bearer session endpoint', async () => {
    global.fetch = vi.fn(async () => {
      return {
        ok: true,
        json: async () => ({
          status: 'replaced',
          matchId: 'solo-old',
          replacementMatchId: 'solo-new',
          replacement: {
            matchId: 'solo-new',
            mode: 'singleplayer',
            node: 'game-1',
            ticket: 'ticket-2',
            wsPath: '/ws/game-1'
          }
        })
      } as Response;
    }) as typeof fetch;

    const response = await fetchMatchSession(runtimeConfig, 'access-token', 'solo-old', new AbortController().signal);

    expect(response).toEqual({
      status: 'replaced',
      matchId: 'solo-old',
      replacementMatchId: 'solo-new',
      replacement: {
        matchId: 'solo-new',
        mode: 'singleplayer',
        node: 'game-1',
        ticket: 'ticket-2',
        wsPath: '/ws/game-1'
      }
    });
  });

  it('resolves a public history route without an access token', async () => {
    global.fetch = vi.fn(async (_input, init) => {
      expect(init?.headers).toBeUndefined();
      return {
        ok: true,
        json: async () => ({
          status: 'history',
          matchId: 'match-1',
          snapshot: {
            matchId: 'match-1',
            state: 'ended',
            phase: 'ended',
            roundPhase: 'ended',
            phaseStartedAt: 0,
            phaseEndsAt: 0,
            roundMsLeft: 0,
            players: {},
            eventSequence: 1
          }
        })
      } as Response;
    }) as typeof fetch;

    const response = await resolveMatchRoute(runtimeConfig, 'match-1', new AbortController().signal);

    expect(response.status).toBe('history');
    expect(response.matchId).toBe('match-1');
  });
});
