import React from 'react';
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

    it('shows only the Open Map FAB when the map is closed', () => {
      renderPanel();

      expect(screen.getByRole('button', { name: 'Open map' })).toBeInTheDocument();
      expect(screen.getByTestId('mobile-map-overlay')).toHaveClass('opacity-0');
      expect(screen.queryByRole('button', { name: 'Back to Street View' })).not.toBeInTheDocument();
    });

    it('opens the fullscreen map when the Open Map FAB is clicked', () => {
      renderPanel();

      fireEvent.click(screen.getByRole('button', { name: 'Open map' }));

      const overlay = screen.getByTestId('mobile-map-overlay');
      expect(overlay).toHaveClass('inset-0');
      expect(screen.getByTestId('guess-map')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Back to Street View' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Place Pin' })).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Open map' })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Collapse minimap' })).not.toBeInTheDocument();
      expect(screen.queryByTestId('minimap-panel')).not.toBeInTheDocument();
    });

    it('returns to Street View when Back is clicked', () => {
      renderPanel();

      fireEvent.click(screen.getByRole('button', { name: 'Open map' }));
      fireEvent.click(screen.getByRole('button', { name: 'Back to Street View' }));

      expect(screen.getByRole('button', { name: 'Open map' })).toBeInTheDocument();
      expect(screen.getByTestId('mobile-map-overlay')).toHaveClass('opacity-0');
    });

    it('keeps the map mounted when toggling Street View', () => {
      renderPanel();
      const map = screen.getByTestId('guess-map');

      fireEvent.click(screen.getByRole('button', { name: 'Open map' }));
      fireEvent.click(screen.getByRole('button', { name: 'Back to Street View' }));
      fireEvent.click(screen.getByRole('button', { name: 'Open map' }));

      expect(screen.getByTestId('guess-map')).toBe(map);
      expect(screen.getByTestId('mobile-map-overlay')).toHaveClass('inset-0');
      expect(screen.getByTestId('mobile-map-overlay')).not.toHaveClass('opacity-0');
    });

    it('closes the map when roundKey changes', () => {
      const { rerender, onFinalize } = renderPanel({ roundKey: 'round-1' });

      fireEvent.click(screen.getByRole('button', { name: 'Open map' }));
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

      expect(screen.getByRole('button', { name: 'Open map' })).toBeInTheDocument();
      expect(screen.getByTestId('mobile-map-overlay')).toHaveClass('opacity-0');
    });
  });

  describe('desktop', () => {
    beforeEach(() => {
      mockMatchMedia(true);
    });

    it('renders the minimap panel without the mobile Open Map FAB', () => {
      renderPanel();

      expect(screen.getByTestId('guess-map')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Place Pin' })).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Open map' })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Back to Street View' })).not.toBeInTheDocument();
    });
  });
});
