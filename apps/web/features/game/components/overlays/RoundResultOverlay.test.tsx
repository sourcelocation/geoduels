import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import RoundResultOverlay from './RoundResultOverlay';
import type { RoundResultOverlayProps } from '../../model/types';

function createProps(overrides: Partial<RoundResultOverlayProps> = {}): RoundResultOverlayProps {
  return {
    roundNumber: 3,
    mapNode: <div>Map Node</div>,
    phase: 'scores',
    showScoreReveal: true,
    winner: 'self',
    damage: 123,
    damageMultiplier: 1.5,
    sides: {
      self: {
        id: 'self',
        participant: { kind: 'player', id: 'self', name: 'You', avatarFallback: 'Y' },
        hp: 4000,
        score: 4321,
        connection: 'connected',
      },
      opponent: {
        id: 'opp',
        participant: { kind: 'player', id: 'opp', name: 'Opp', avatarFallback: 'O' },
        hp: 3200,
        score: 1111,
        connection: 'connected',
      }
    },
    hpPct: (hp) => `${Math.max(0, Math.min(100, (hp / 6000) * 100))}%`,
    ...overrides
  };
}

describe('RoundResultOverlay component', () => {
  afterEach(() => {
    cleanup();
  });

  it('does not render score travel token on tie', () => {
    render(
      <RoundResultOverlay
        {...createProps({
          phase: 'crush',
          winner: 'tie',
          damage: 0,
          sides: {
            ...createProps().sides,
            self: { ...createProps().sides.self, score: 2500 },
            opponent: { ...createProps().sides.opponent, score: 2500 },
          }
        })}
      />
    );

    expect(screen.queryByTestId('score-travel-token')).not.toBeInTheDocument();
  });

  it('renders score travel token in crush phase for non-tie with damage', async () => {
    render(<RoundResultOverlay {...createProps({ phase: 'crush', winner: 'self', damage: 321 })} />);

    await waitFor(() => {
      expect(screen.getByTestId('score-travel-token')).toBeInTheDocument();
    });
  });

  it('reveals the damage multiplier and updates the shown damage when the multiplier phase starts', () => {
    render(
      <RoundResultOverlay
        {...createProps({
          phase: 'damage_multiplier',
          winner: 'self',
          damage: 123,
          damageMultiplier: 1.5
        })}
      />
    );

    expect(screen.getByTestId('damage-multiplier-label')).toHaveTextContent('1.5x');
    expect(screen.getByText('185')).toBeInTheDocument();
  });

  it('does not show a baseline damage multiplier label', () => {
    render(
      <RoundResultOverlay
        {...createProps({
          phase: 'damage_multiplier',
          winner: 'self',
          damage: 123,
          damageMultiplier: 1
        })}
      />
    );

    expect(screen.queryByTestId('damage-multiplier-label')).not.toBeInTheDocument();
    expect(screen.getAllByText('123').length).toBeGreaterThan(0);
  });

  it('formats whole-number damage multipliers without decimals', () => {
    render(
      <RoundResultOverlay
        {...createProps({
          phase: 'damage_multiplier',
          winner: 'self',
          damage: 123,
          damageMultiplier: 2
        })}
      />
    );

    expect(screen.getByTestId('damage-multiplier-label')).toHaveTextContent('2x');
    expect(screen.getByText('246')).toBeInTheDocument();
  });
});
