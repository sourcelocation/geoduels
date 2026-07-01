import { afterEach, describe, expect, it, vi } from 'vitest';
import { createRuntimeConfigFixture } from '../../../test/runtime-config.fixture';
import { MatchRouteController } from './match-route-controller';

describe('MatchRouteController', () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    global.fetch = originalFetch;
    vi.restoreAllMocks();
  });

  it('bootstraps a cold route match and becomes idle after the first route snapshot arrives', async () => {
    const listeners = new Set<() => void>();
    let matchState = { snapshot: null as any };

    global.fetch = vi.fn(async (input) => {
      const url = String(input);
      if (url.includes('/route')) {
        return {
          ok: true,
          json: async () => ({
            status: 'live_auth_required',
            matchId: 'solo-123'
          })
        } as Response;
      }
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

    const sessionController = {
      getSessionSnapshot: vi.fn(() => null),
      applySessionSnapshot: vi.fn(),
      ensureFreshSession: vi.fn(async () => null)
    } as any;

    const matchController = {
      subscribe: (listener: () => void) => {
        listeners.add(listener);
        return () => listeners.delete(listener);
      },
      getState: () => matchState,
      resumeResolvedMatch: vi.fn(async () => true)
    } as any;

    const controller = new MatchRouteController({
      config: createRuntimeConfigFixture(),
      sessionController,
      matchController
    });
    controller.start();
    controller.setTargetMatch('solo-123');

    await vi.waitFor(() => expect(controller.getState().status).toBe('awaiting_first_snapshot'));
    expect(sessionController.applySessionSnapshot).toHaveBeenCalled();
    expect(matchController.resumeResolvedMatch).toHaveBeenCalledWith({
      matchId: 'solo-123',
      node: 'game-1',
      wsPath: '/ws/game-1',
      ticket: 'ticket-1'
    });

    matchState = {
      snapshot: {
        matchId: 'solo-123'
      }
    };
    listeners.forEach((listener) => listener());

    expect(controller.getState().status).toBe('idle');
    controller.destroy();
  });
});
