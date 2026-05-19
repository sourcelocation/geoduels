import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import InGameScene from './InGameScene';
import type { InGameSceneProps } from './InGameScene';

function createProps(overrides: Partial<InGameSceneProps> = {}): InGameSceneProps {
  return {
    uiPhase: 'live_round',
    streetViewSrc: 'https://www.google.com/maps/embed/v1/streetview?key=test&pano=pano-1&heading=93.5',
    streetViewInteractive: true,
    showResultStage: false,
    isSingleplayer: false,
    isPointsMode: false,
    resultOverlay: undefined,
    selfName: 'Self',
    selfFallback: 'S',
    selfIsAdmin: false,
    opponentName: 'Opponent',
    opponentIsAdmin: false,
    opponentDisconnected: false,
    oppFallback: 'O',
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
    selfElo: 1200,
    opponentElo: 1200,
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

  it('keeps the Street View iframe out of keyboard tab navigation', () => {
    render(<InGameScene {...createProps()} />);

    const streetViewFrame = screen.getByTitle('Street View');

    expect(streetViewFrame).toHaveAttribute('tabindex', '-1');
  });

  it('shows a top-centered direction HUD from the Street View heading', () => {
    render(<InGameScene {...createProps()} />);

    expect(screen.getByTestId('direction-hud')).toBeInTheDocument();
    expect(screen.getByTestId('direction-hud-heading')).toHaveTextContent('E 94°');
  });

  it('updates the direction HUD when the Street View source heading changes', () => {
    const { rerender } = render(<InGameScene {...createProps()} />);

    rerender(
      <InGameScene
        {...createProps({
          streetViewSrc: 'https://www.google.com/maps/embed/v1/streetview?key=test&pano=pano-2&heading=181',
        })}
      />,
    );

    expect(screen.getByTestId('direction-hud-heading')).toHaveTextContent('S 181°');
  });

  it('releases focus if the Street View iframe captures it', () => {
    render(<InGameScene {...createProps()} />);

    const streetViewFrame = screen.getByTitle('Street View');

    streetViewFrame.focus();

    expect(document.activeElement).not.toBe(streetViewFrame);
    expect(document.activeElement?.tagName).toBe('SECTION');
  });
});
