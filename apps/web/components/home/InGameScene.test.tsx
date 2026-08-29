import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import InGameScene from './InGameScene';
import type { InGameSceneProps } from './InGameScene';

function createProps(overrides: Partial<InGameSceneProps> = {}): InGameSceneProps {
  return {
    uiPhase: 'live_round',
    streetViewSrc: 'https://www.google.com/maps/embed/v1/streetview?key=test&pano=pano-1',
    streetViewInteractive: true,
    ruleset: 'moving',
    streetNames: 'shown',
    showResultStage: false,
    isSingleplayer: false,
    isPointsMode: false,
    resultOverlay: undefined,
    sides: {
      self: {
        id: 'self',
        participant: {
          kind: 'player',
          id: 'self',
          name: 'Self',
          avatarFallback: 'S',
          rating: 1200,
        },
        hp: 5000,
        connection: 'connected',
      },
      opponent: {
        id: 'opp',
        participant: {
          kind: 'player',
          id: 'opp',
          name: 'Opponent',
          avatarFallback: 'O',
          rating: 1200,
        },
        hp: 5000,
        connection: 'connected',
      },
    },
    hpPct: (hp) => `${hp}%`,
    mm: '01',
    ss: '00',
    isRoundTimerRunning: true,
    timerProgressPct: 50,
    isTimerCritical: false,
    isTimerPulseActive: false,
    resultMode: false,
    selfHP: 5000,
    oppHP: 5000,
    totalScore: 0,
    currentRoundScore: 0,
    currentRoundDistanceKm: 0,
    onForfeit: vi.fn(() => true),
    onAdvanceRound: vi.fn(() => true),
    onLeaveGame: vi.fn(),
    canFinalizeGuess: false,
    canAdvanceRound: false,
    onFinalizeGuess: vi.fn(),
    guessMapNode: null,
    selfUserId: 'self',
    damageMultiplier: 1,
    guessSubmitted: false,
    opponentGuessAlert: false,
    connectionIssue: '',
    ...overrides,
  };
}

describe('InGameScene', () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    const values = new Map<string, string>();
    Object.defineProperty(window, 'localStorage', {
      configurable: true,
      value: {
        clear: () => values.clear(),
        getItem: (key: string) => values.get(key) ?? null,
        removeItem: (key: string) => values.delete(key),
        setItem: (key: string, value: string) => values.set(key, value),
      },
    });
    window.localStorage.clear();
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });
  });

  it('keeps interactive Street View iframes in keyboard tab navigation', () => {
    render(<InGameScene {...createProps()} />);

    const streetViewFrame = screen.getByTitle('Street View');

    expect(streetViewFrame).not.toHaveAttribute('tabindex');
  });

  it('shows individual multipliers beside both player names instead of in the shared HUD', () => {
    render(
      <InGameScene
        {...createProps({
          multiplierMode: 'individual',
          selfDamageMultiplier: 1.5,
          oppDamageMultiplier: 1,
        })}
      />,
    );

    expect(screen.getByTestId('self-multiplier')).toHaveTextContent('1.5x');
    expect(screen.getByTestId('opponent-multiplier')).toHaveTextContent('1x');
    expect(screen.queryByTestId('multiplier-badge')).not.toBeInTheDocument();
  });

  it('embeds extension rules in the Street View iframe hash', () => {
    render(
      <InGameScene
        {...createProps({ ruleset: 'no_move', streetNames: 'hidden' })}
      />,
    );

    const streetViewFrame = screen.getByTitle('Street View') as HTMLIFrameElement;
    const url = new URL(streetViewFrame.src);
    const config = JSON.parse(
      new URLSearchParams(url.hash.slice(1)).get('geoduels') || '{}',
    );

    expect(config).toEqual({
      version: 1,
      ruleset: 'no_move',
      streetNames: 'hidden',
    });
  });

  it('starts the extension handshake only after the Street View frame loads', async () => {
    render(
      <InGameScene
        {...createProps({ ruleset: 'no_move', streetNames: 'hidden' })}
      />,
    );

    const streetViewFrame = screen.getByTitle('Street View') as HTMLIFrameElement;
    const postMessage = vi.spyOn(streetViewFrame.contentWindow!, 'postMessage');

    expect(postMessage).not.toHaveBeenCalled();

    fireEvent.load(streetViewFrame);

    await waitFor(() => {
      expect(postMessage).toHaveBeenCalledWith(
        expect.objectContaining({
          source: 'geoduels-app',
          type: 'configure',
          ruleset: 'no_move',
          streetNames: 'hidden',
        }),
        'https://www.google.com',
      );
    });
  });

  it('restarts the extension handshake after Retry remounts the Street View frame', async () => {
    vi.useFakeTimers();
    try {
      render(
        <InGameScene
          {...createProps({ ruleset: 'no_move', streetNames: 'hidden' })}
        />,
      );

      fireEvent.load(screen.getByTitle('Street View'));
      await act(async () => {
        await vi.advanceTimersByTimeAsync(12_000);
      });

      expect(
        screen.getByText("Couldn't reach the official extension."),
      ).toBeInTheDocument();

      fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
      const remountedFrame = screen.getByTitle('Street View') as HTMLIFrameElement;
      const postMessage = vi.spyOn(remountedFrame.contentWindow!, 'postMessage');
      fireEvent.load(remountedFrame);

      await act(async () => {
        await vi.advanceTimersByTimeAsync(0);
      });

      expect(postMessage).toHaveBeenCalledWith(
        expect.objectContaining({
          source: 'geoduels-app',
          type: 'configure',
          ruleset: 'no_move',
          streetNames: 'hidden',
        }),
        'https://www.google.com',
      );
    } finally {
      vi.useRealTimers();
    }
  });

  it('drops the minimap Street View control gutter once the extension is available', async () => {
    render(<InGameScene {...createProps()} />);

    const panel = screen.getByTestId('minimap-panel');
    expect(panel).toHaveStyle({ right: '80px' });

    const streetViewFrame = screen.getByTitle('Street View') as HTMLIFrameElement;
    fireEvent.load(streetViewFrame);
    window.dispatchEvent(
      new MessageEvent('message', {
        origin: 'https://www.google.com',
        source: streetViewFrame.contentWindow,
        data: {
          source: 'geoduels-extension',
          version: 1,
          extensionVersion: '0.1.4',
          type: 'ready',
          capabilities: { heading: true, roadLabels: true },
        },
      }),
    );

    await waitFor(() => {
      expect(panel).not.toHaveStyle({ right: '80px' });
    });
    expect(panel).toHaveClass('p-3', 'right-0');
  });

  it('keeps NMPZ Street View iframes out of keyboard tab navigation', () => {
    render(<InGameScene {...createProps({ streetViewInteractive: false })} />);

    const streetViewFrame = screen.getByTitle('Street View');

    expect(streetViewFrame).toHaveAttribute('tabindex', '-1');
  });

  it('allows interactive Street View iframes to keep focus', () => {
    render(<InGameScene {...createProps()} />);

    const streetViewFrame = screen.getByTitle('Street View');

    streetViewFrame.focus();

    expect(document.activeElement).toBe(streetViewFrame);
  });

  it('releases focus if the NMPZ Street View iframe captures it', () => {
    render(<InGameScene {...createProps({ streetViewInteractive: false })} />);

    const streetViewFrame = screen.getByTitle('Street View');

    streetViewFrame.focus();

    expect(document.activeElement).not.toBe(streetViewFrame);
    expect(document.activeElement?.tagName).toBe('SECTION');
  });

  it('renders an immediate wrap-safe app-owned compass when the extension bridge is ready', async () => {
    render(<InGameScene {...createProps()} />);

    const streetViewFrame = screen.getByTitle('Street View') as HTMLIFrameElement;
    window.dispatchEvent(
      new MessageEvent('message', {
        origin: 'https://www.google.com',
        source: streetViewFrame.contentWindow,
        data: {
          source: 'geoduels-extension',
          version: 1,
          extensionVersion: '0.1.3',
          type: 'ready',
          capabilities: { heading: true, roadLabels: true },
        },
      }),
    );
    window.dispatchEvent(
      new MessageEvent('message', {
        origin: 'https://www.google.com',
        source: streetViewFrame.contentWindow,
        data: {
          source: 'geoduels-extension',
          version: 1,
          extensionVersion: '0.1.3',
          type: 'pov',
          heading: 359,
        },
      }),
    );

    expect(await screen.findByTestId('extension-compass')).toHaveAttribute(
      'aria-label',
      'Compass heading 359 degrees',
    );
    expect(
      (screen.getByTestId('extension-compass').firstElementChild as HTMLElement)
        .style.transform,
    ).toBe('translateX(132px)');

    window.dispatchEvent(
      new MessageEvent('message', {
        origin: 'https://www.google.com',
        source: streetViewFrame.contentWindow,
        data: {
          source: 'geoduels-extension',
          version: 1,
          extensionVersion: '0.1.3',
          type: 'pov',
          heading: 1,
        },
      }),
    );

    await waitFor(() => {
      expect(screen.getByTestId('extension-compass')).toHaveAttribute(
        'aria-label',
        'Compass heading 1 degrees',
      );
      expect(
        (screen.getByTestId('extension-compass').firstElementChild as HTMLElement)
          .style.transform,
      ).toBe('translateX(128px)');
    });
    expect(
      screen.getByTestId('extension-compass').firstElementChild,
    ).not.toHaveClass('transition-transform');
    expect(
      screen.queryByRole('button', {
        name: 'Street View enhancement settings',
      }),
    ).not.toBeInTheDocument();

    for (const heading of [90, 180, 270, 0]) {
      window.dispatchEvent(
        new MessageEvent('message', {
          origin: 'https://www.google.com',
          source: streetViewFrame.contentWindow,
          data: {
            source: 'geoduels-extension',
            version: 1,
            extensionVersion: '0.1.3',
            type: 'pov',
            heading,
          },
        }),
      );
    }
    await waitFor(() => {
      expect(
        (screen.getByTestId('extension-compass').firstElementChild as HTMLElement)
          .style.transform,
      ).toBe('translateX(-590px)');
    });
  });

  it('covers extension-required Street View until the requested configuration is active', async () => {
    render(
      <InGameScene
        {...createProps({ ruleset: 'no_move', streetNames: 'hidden' })}
      />,
    );
    const streetViewFrame = screen.getByTitle('Street View') as HTMLIFrameElement;

    expect(screen.getByText('Preparing official extension…')).toBeInTheDocument();

    window.dispatchEvent(
      new MessageEvent('message', {
        origin: 'https://www.google.com',
        source: streetViewFrame.contentWindow,
        data: {
          source: 'geoduels-extension',
          version: 1,
          extensionVersion: '0.1.3',
          type: 'ready',
          capabilities: { heading: true, roadLabels: true },
        },
      }),
    );
    window.dispatchEvent(
      new MessageEvent('message', {
        origin: 'https://www.google.com',
        source: streetViewFrame.contentWindow,
        data: {
          source: 'geoduels-extension',
          version: 1,
          extensionVersion: '0.1.3',
          type: 'configured',
          ruleset: 'no_move',
          streetNames: 'hidden',
        },
      }),
    );

    await waitFor(() => {
      expect(
        screen.queryByText('Preparing official extension…'),
      ).not.toBeInTheDocument();
    });
  });

  it('keeps extension-required Street View covered for outdated extension releases', async () => {
    render(
      <InGameScene
        {...createProps({ ruleset: 'no_move', streetNames: 'hidden' })}
      />,
    );
    const streetViewFrame = screen.getByTitle('Street View') as HTMLIFrameElement;

    window.dispatchEvent(
      new MessageEvent('message', {
        origin: 'https://www.google.com',
        source: streetViewFrame.contentWindow,
        data: {
          source: 'geoduels-extension',
          version: 1,
          extensionVersion: '0.1.2',
          type: 'ready',
          capabilities: { heading: true, roadLabels: true },
        },
      }),
    );
    window.dispatchEvent(
      new MessageEvent('message', {
        origin: 'https://www.google.com',
        source: streetViewFrame.contentWindow,
        data: {
          source: 'geoduels-extension',
          version: 1,
          extensionVersion: '0.1.2',
          type: 'configured',
          ruleset: 'no_move',
          streetNames: 'hidden',
        },
      }),
    );

    await waitFor(() => {
      expect(
        screen.getByText('Update the official extension to keep playing this mode.'),
      ).toBeInTheDocument();
    });
  });

  it('renders team identity without promoting member avatars, badges, or ratings', () => {
    const memberBadge = {
      id: 'badge-1',
      kind: 'rank',
      label: 'Member Badge',
      description: 'Player-only badge',
      imageUrl: '/badge.png',
    };
    render(
      <InGameScene
        {...createProps({
          partyMode: 'team_duel',
          sides: {
            self: {
              id: 'a',
              participant: {
                kind: 'team',
                id: 'a',
                name: 'Team Red',
                avatarFallback: 'R',
                avatarColor: '#dc2626',
                members: [
                  {
                    kind: 'player',
                    id: 'self',
                    name: 'Red Member',
                    avatarFallback: 'M',
                    rating: 2100,
                    selectedBadge: memberBadge,
                  },
                ],
              },
              hp: 6000,
              connection: 'connected',
            },
            opponent: {
              id: 'b',
              participant: {
                kind: 'team',
                id: 'b',
                name: 'Team Blue',
                avatarFallback: 'B',
                avatarColor: '#2563eb',
                members: [],
              },
              hp: 3200,
              connection: 'connected',
            },
          },
          selfHP: 6000,
          oppHP: 3200,
        })}
      />,
    );

    expect(screen.getByText('Team Red')).toBeInTheDocument();
    expect(screen.getByText('Team Blue')).toBeInTheDocument();
    expect(screen.queryByText('Red Member')).not.toBeInTheDocument();
    expect(screen.queryByText('(2100)')).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/Member Badge/)).not.toBeInTheDocument();
  });
});
