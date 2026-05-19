import { act, cleanup, render, waitFor } from '@testing-library/react';
import { createRef } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import StreetViewPane from './StreetViewPane';

describe('StreetViewPane', () => {
  afterEach(() => {
    cleanup();
    delete window.google;
  });

  it('reports Google Street View pov changes as heading updates', async () => {
    let pov = { heading: 12, pitch: 0, zoom: 0 };
    let povChanged: (() => void) | undefined;
    const onHeadingChange = vi.fn();
    const setPov = vi.fn((nextPov: typeof pov) => {
      pov = nextPov;
    });

    window.google = {
      maps: {
        StreetViewPanorama: vi.fn(function StreetViewPanorama() {
          return {
            addListener: vi.fn((eventName: string, handler: () => void) => {
              if (eventName === 'pov_changed') povChanged = handler;
              return { remove: vi.fn() };
            }),
            getPov: vi.fn(() => pov),
            setPano: vi.fn(),
            setPov,
          };
        }),
      },
    };

    render(
      <StreetViewPane
        src="https://www.google.com/maps/embed/v1/streetview?key=test&pano=pano-1&heading=12"
        interactive
        resetCount={0}
        iframeRef={createRef<HTMLIFrameElement>()}
        onFrameFocus={vi.fn()}
        onHeadingChange={onHeadingChange}
      />,
    );

    await waitFor(() => expect(povChanged).toBeDefined());
    expect(onHeadingChange).toHaveBeenCalledWith(12);

    act(() => {
      pov = { heading: 91.8, pitch: 0, zoom: 0 };
      povChanged?.();
    });

    expect(onHeadingChange.mock.calls.at(-1)?.[0]).toBeCloseTo(91.8);
  });
});
