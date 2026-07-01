import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import MinimapPanel from '../MinimapPanel';

function mockDesktop(matches: boolean) {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: vi.fn().mockReturnValue({
      matches,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn()
    })
  });
}

describe('MinimapPanel', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'PointerEvent', {
      configurable: true,
      value: MouseEvent
    });
    const values = new Map<string, string>();
    Object.defineProperty(window, 'localStorage', {
      configurable: true,
      value: {
        clear: () => values.clear(),
        getItem: (key: string) => values.get(key) ?? null,
        removeItem: (key: string) => values.delete(key),
        setItem: (key: string, value: string) => values.set(key, value)
      }
    });
    window.localStorage.clear();
  });

  afterEach(() => {
    cleanup();
    window.localStorage.clear();
  });

  it('shows the resize handle only while the desktop minimap is expanded', () => {
    mockDesktop(true);
    render(
      <MinimapPanel onFinalize={vi.fn()} canFinalizeGuess={false} guessSubmitted={false}>
        <div>Map</div>
      </MinimapPanel>
    );

    const panel = screen.getByTestId('minimap-panel');
    expect(screen.queryByRole('button', { name: 'Resize minimap' })).not.toBeInTheDocument();

    fireEvent.mouseEnter(panel);

    expect(screen.getByRole('button', { name: 'Resize minimap' })).toBeInTheDocument();
    expect(panel.firstElementChild).toHaveClass('-inset-24');
  });

  it('does not show the resize handle on mobile', () => {
    mockDesktop(false);
    render(
      <MinimapPanel onFinalize={vi.fn()} canFinalizeGuess={false} guessSubmitted={false}>
        <div>Map</div>
      </MinimapPanel>
    );

    fireEvent.click(screen.getByText('Map'));

    expect(screen.queryByRole('button', { name: 'Resize minimap' })).not.toBeInTheDocument();
  });

  it('applies a resized desktop size immediately and remembers it', () => {
    mockDesktop(true);
    render(
      <MinimapPanel onFinalize={vi.fn()} canFinalizeGuess={false} guessSubmitted={false}>
        <div>Map</div>
      </MinimapPanel>
    );

    const panel = screen.getByTestId('minimap-panel');
    fireEvent.mouseEnter(panel);
    vi.spyOn(panel, 'getBoundingClientRect').mockReturnValue({
      width: 800,
      height: 560,
      x: 0,
      y: 0,
      top: 0,
      right: 800,
      bottom: 560,
      left: 0,
      toJSON: () => ({})
    });

    const handle = screen.getByRole('button', { name: 'Resize minimap' });
    fireEvent.pointerDown(handle, { pointerId: 1, clientX: 100, clientY: 100 });
    fireEvent.pointerMove(handle, { pointerId: 1, clientX: 60, clientY: 70 });

    expect(panel).toHaveStyle({ width: '840px', height: '590px' });

    fireEvent.pointerUp(handle, { pointerId: 1, clientX: 60, clientY: 70 });
    expect(window.localStorage.getItem('geoduels.minimapExpandedSize')).toBe('{"width":840,"height":590}');
  });
});
