import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import MinimapPanel from './MinimapPanel';

function mockMatchMedia(matches: boolean) {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
}

type MinimapPanelProps = {
  onFinalize?: () => void;
  canFinalizeGuess?: boolean;
  guessSubmitted?: boolean;
  roundKey?: string;
};

function renderPanel(overrides: MinimapPanelProps = {}) {
  const onFinalize = overrides.onFinalize ?? vi.fn();
  const props = {
    onFinalize,
    canFinalizeGuess: overrides.canFinalizeGuess ?? false,
    guessSubmitted: overrides.guessSubmitted ?? false,
    roundKey: overrides.roundKey,
  };

  const view = render(
    <MinimapPanel {...props}>
      <div data-testid="guess-map">Map</div>
    </MinimapPanel>,
  );

  return { ...view, onFinalize };
}

describe('MinimapPanel', () => {
  afterEach(() => {
    cleanup();
  });

  describe('mobile', () => {
    beforeEach(() => {
      mockMatchMedia(false);
    });

    it('shows only the Guess FAB when the map is closed', () => {
      renderPanel();

      expect(screen.getByRole('button', { name: 'Open map to place guess' })).toBeInTheDocument();
      expect(screen.queryByTestId('guess-map')).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Back to Street View' })).not.toBeInTheDocument();
    });

    it('opens the fullscreen map when the Guess FAB is clicked', () => {
      renderPanel();

      fireEvent.click(screen.getByRole('button', { name: 'Open map to place guess' }));

      expect(screen.getByTestId('guess-map')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Back to Street View' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Place Pin' })).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Open map to place guess' })).not.toBeInTheDocument();
    });

    it('returns to Street View when Back is clicked', () => {
      renderPanel();

      fireEvent.click(screen.getByRole('button', { name: 'Open map to place guess' }));
      fireEvent.click(screen.getByRole('button', { name: 'Back to Street View' }));

      expect(screen.getByRole('button', { name: 'Open map to place guess' })).toBeInTheDocument();
      expect(screen.queryByTestId('guess-map')).not.toBeInTheDocument();
    });

    it('closes the map when roundKey changes', () => {
      const { rerender, onFinalize } = renderPanel({ roundKey: 'round-1' });

      fireEvent.click(screen.getByRole('button', { name: 'Open map to place guess' }));
      expect(screen.getByTestId('guess-map')).toBeInTheDocument();

      rerender(
        <MinimapPanel
          onFinalize={onFinalize}
          canFinalizeGuess={false}
          guessSubmitted={false}
          roundKey="round-2"
        >
          <div data-testid="guess-map">Map</div>
        </MinimapPanel>,
      );

      expect(screen.getByRole('button', { name: 'Open map to place guess' })).toBeInTheDocument();
      expect(screen.queryByTestId('guess-map')).not.toBeInTheDocument();
    });
  });

  describe('desktop', () => {
    beforeEach(() => {
      mockMatchMedia(true);
    });

    it('renders the minimap panel without the mobile Guess FAB', () => {
      renderPanel();

      expect(screen.getByTestId('guess-map')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Place Pin' })).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Open map to place guess' })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Back to Street View' })).not.toBeInTheDocument();
    });
  });
});
