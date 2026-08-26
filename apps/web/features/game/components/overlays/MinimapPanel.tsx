import type { MouseEvent as ReactMouseEvent, PointerEvent as ReactPointerEvent, ReactNode } from 'react';
import { useEffect, useRef, useState } from 'react';
import { MoveDiagonal2 } from 'lucide-react';
import { Button, IconButton } from '../../../../components/ui/button';

type Props = {
  children: ReactNode;
  onFinalize: () => void;
  canFinalizeGuess: boolean;
  guessSubmitted: boolean;
};

type MinimapSize = {
  width: number;
  height: number;
};

const DESKTOP_BREAKPOINT_PX = 768;
const RIGHT_GUTTER_PX = 80;
const MIN_EXPANDED_SIZE: MinimapSize = { width: 460, height: 360 };
const EXPANDED_SIZE_STORAGE_KEY = 'geoduels.minimapExpandedSize';

function clampExpandedSize(size: MinimapSize): MinimapSize {
  if (typeof window === 'undefined') return size;

  return {
    width: Math.round(Math.min(Math.max(size.width, MIN_EXPANDED_SIZE.width), window.innerWidth - RIGHT_GUTTER_PX - 16)),
    height: Math.round(Math.min(Math.max(size.height, MIN_EXPANDED_SIZE.height), window.innerHeight - 32))
  };
}

function loadExpandedSize(): MinimapSize | null {
  try {
    const stored = window.localStorage.getItem(EXPANDED_SIZE_STORAGE_KEY);
    if (!stored) return null;
    const parsed = JSON.parse(stored) as Partial<MinimapSize>;
    if (!Number.isFinite(parsed.width) || !Number.isFinite(parsed.height)) return null;
    return clampExpandedSize({ width: Number(parsed.width), height: Number(parsed.height) });
  } catch {
    return null;
  }
}

export default function MinimapPanel({ children, onFinalize, canFinalizeGuess, guessSubmitted }: Props) {
  const GESTURE_MOVE_THRESHOLD_PX = 6;
  const panelRef = useRef<HTMLDivElement | null>(null);
  const activePointerRef = useRef<{ id: number; x: number; y: number } | null>(null);
  const activeResizeRef = useRef<({ id: number; x: number; y: number } & MinimapSize) | null>(null);
  const resizeLockedRef = useRef(false);
  const suppressNextPanelClickRef = useRef(false);
  const [desktopHovered, setDesktopHovered] = useState(false);
  const [mobileExpanded, setMobileExpanded] = useState(false);
  const [isDesktop, setIsDesktop] = useState(false);
  const [expandedSize, setExpandedSize] = useState<MinimapSize | null>(null);

  useEffect(() => {
    if (typeof window === 'undefined') return;

    const mediaQuery = window.matchMedia(`(min-width: ${DESKTOP_BREAKPOINT_PX}px)`);
    const syncViewport = () => {
      const desktop = mediaQuery.matches;
      setIsDesktop(desktop);
      if (desktop) setMobileExpanded(false);
    };

    syncViewport();

    if (typeof mediaQuery.addEventListener === 'function') {
      mediaQuery.addEventListener('change', syncViewport);
      return () => mediaQuery.removeEventListener('change', syncViewport);
    }

    mediaQuery.addListener(syncViewport);
    return () => mediaQuery.removeListener(syncViewport);
  }, []);

  useEffect(() => {
    if (typeof window === 'undefined') return;
    setExpandedSize(loadExpandedSize());
  }, []);

  useEffect(() => {
    if (typeof window === 'undefined') return;

    const releaseResizeLock = () => {
      resizeLockedRef.current = false;
      if (!isDesktop) return;

      const panel = panelRef.current;
      setDesktopHovered(Boolean(panel?.matches(':hover')));
    };

    const handleMouseMove = (event: MouseEvent) => {
      if (resizeLockedRef.current && event.buttons === 0) releaseResizeLock();
    };

    window.addEventListener('pointerup', releaseResizeLock, true);
    window.addEventListener('pointercancel', releaseResizeLock, true);
    window.addEventListener('mouseup', releaseResizeLock, true);
    window.addEventListener('mousemove', handleMouseMove, true);
    window.addEventListener('blur', releaseResizeLock);

    return () => {
      window.removeEventListener('pointerup', releaseResizeLock, true);
      window.removeEventListener('pointercancel', releaseResizeLock, true);
      window.removeEventListener('mouseup', releaseResizeLock, true);
      window.removeEventListener('mousemove', handleMouseMove, true);
      window.removeEventListener('blur', releaseResizeLock);
    };
  }, [isDesktop]);

  const expanded = isDesktop ? desktopHovered : mobileExpanded;
  const reserveRightGutter = isDesktop || !mobileExpanded;
  const finalizeLabel = guessSubmitted ? 'Waiting for opponent...' : canFinalizeGuess ? 'Guess' : 'Place Pin';

  const beginPointerGesture = (event: ReactPointerEvent) => {
    activePointerRef.current = {
      id: event.pointerId,
      x: event.clientX,
      y: event.clientY
    };
    resizeLockedRef.current = true;
    suppressNextPanelClickRef.current = false;
  };

  const trackPointerGesture = (event: ReactPointerEvent) => {
    const activePointer = activePointerRef.current;
    if (!activePointer || activePointer.id !== event.pointerId) return;

    const dx = event.clientX - activePointer.x;
    const dy = event.clientY - activePointer.y;
    if (Math.hypot(dx, dy) > GESTURE_MOVE_THRESHOLD_PX) {
      suppressNextPanelClickRef.current = true;
    }
  };

  const endPointerGesture = (event: ReactPointerEvent) => {
    if (activePointerRef.current?.id === event.pointerId) activePointerRef.current = null;
  };

  const handlePanelClick = () => {
    if (suppressNextPanelClickRef.current) {
      suppressNextPanelClickRef.current = false;
      return;
    }
    if (!isDesktop) setMobileExpanded(true);
  };

  const handleBackdropClick = () => {
    if (suppressNextPanelClickRef.current) {
      suppressNextPanelClickRef.current = false;
      return;
    }
    setMobileExpanded(false);
  };

  const handleFinalizeClick = (event: ReactMouseEvent<HTMLButtonElement>) => {
    event.stopPropagation();
    onFinalize();
  };

  const beginResize = (event: ReactPointerEvent<HTMLButtonElement>) => {
    const panel = panelRef.current;
    if (!isDesktop || !panel) return;

    event.preventDefault();
    event.stopPropagation();
    event.currentTarget.setPointerCapture?.(event.pointerId);
    const rect = panel.getBoundingClientRect();
    activeResizeRef.current = {
      id: event.pointerId,
      x: event.clientX,
      y: event.clientY,
      width: rect.width,
      height: rect.height
    };
    resizeLockedRef.current = true;
    setDesktopHovered(true);
  };

  const resize = (event: ReactPointerEvent<HTMLButtonElement>) => {
    const activeResize = activeResizeRef.current;
    if (!activeResize || activeResize.id !== event.pointerId) return;

    setExpandedSize(clampExpandedSize({
      width: activeResize.width + activeResize.x - event.clientX,
      height: activeResize.height + activeResize.y - event.clientY
    }));
  };

  const finishResize = (event: ReactPointerEvent<HTMLButtonElement>) => {
    if (activeResizeRef.current?.id !== event.pointerId) return;
    activeResizeRef.current = null;

    setExpandedSize((currentSize) => {
      if (!currentSize) return currentSize;
      try {
        window.localStorage.setItem(EXPANDED_SIZE_STORAGE_KEY, JSON.stringify(currentSize));
      } catch {
        // Resizing still works when browser storage is unavailable.
      }
      return currentSize;
    });
  };

  const desktopExpandedStyle = isDesktop && expanded && expandedSize
    ? {
      width: `${expandedSize.width}px`,
      height: `${expandedSize.height}px`
    }
    : {};

  return (
    <>
      {!isDesktop && mobileExpanded ? (
        <Button
          variant="ghost"
          type="button"
          aria-label="Collapse minimap"
          className="absolute inset-0 z-sticky cursor-default bg-transparent"
          onPointerDown={beginPointerGesture}
          onPointerMove={trackPointerGesture}
          onPointerUp={endPointerGesture}
          onPointerCancel={endPointerGesture}
          onClick={handleBackdropClick}
        />
      ) : null}
      <div
        ref={panelRef}
        data-testid="minimap-panel"
        onMouseEnter={(event) => {
          if (!isDesktop) return;
          if (resizeLockedRef.current || event.buttons !== 0) {
            resizeLockedRef.current = true;
            return;
          }
          setDesktopHovered(true);
        }}
        onMouseLeave={(event) => {
          if (!isDesktop) return;
          if (resizeLockedRef.current || event.buttons !== 0) {
            resizeLockedRef.current = true;
            return;
          }
          setDesktopHovered(false);
        }}
        className={`absolute bottom-0 right-0 z-game-controls flex w-full flex-col gap-2 p-3 transition-[width,height] duration-fast ease-standard md:bottom-4 md:right-4 md:p-0 md:w-[min(34vw,460px)] md:h-[min(33vh,360px)] ${expanded ? 'md:w-[min(90vw,800px)] md:h-[min(52vh,560px)]' : ''
          }`}
        style={{
          right: reserveRightGutter ? `${RIGHT_GUTTER_PX}px` : '0px',
          width: isDesktop ? undefined : reserveRightGutter ? `calc(100% - ${RIGHT_GUTTER_PX}px)` : '100%',
          ...desktopExpandedStyle
        }}
      >
        {isDesktop && expanded ? <div aria-hidden="true" className="absolute -inset-24 z-base" /> : null}
        <section
          aria-label="Minimap"
          onClick={handlePanelClick}
          onPointerDown={beginPointerGesture}
          onPointerMove={trackPointerGesture}
          onPointerUp={endPointerGesture}
          onPointerCancel={endPointerGesture}
          className={`group relative z-content min-h-0 w-full origin-bottom-right overflow-hidden rounded-xl border border-border-strong bg-surface-panel shadow-elev-4 transition-[height,opacity,box-shadow] duration-fast ease-standard ${expanded
            ? 'h-[50vh] min-h-[280px] opacity-100 sm:h-[55vh] sm:min-h-[320px]'
            : 'h-[22vh] min-h-[150px] opacity-70 sm:h-[27vh] sm:min-h-[190px]'
            } md:h-auto md:min-h-0 md:flex-1 md:opacity-90 md:hover:opacity-100`}
        >
          {children}
        </section>
        {isDesktop && expanded ? (
          <IconButton
            size="icon-md"
            type="button"
            aria-label="Resize minimap"
            title="Resize minimap"
            className="absolute -left-3 -top-3 z-sticky touch-none rounded-lg border border-border-strong bg-surface-inset text-status-success shadow-elev-3 transition-colors hover:border-status-success/50 hover:bg-surface-panel focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-border-focus"
            onPointerDown={beginResize}
            onPointerMove={resize}
            onPointerUp={finishResize}
            onPointerCancel={finishResize}
          >
            <MoveDiagonal2 size={16} strokeWidth={2.5} />
          </IconButton>
        ) : null}
        <Button
          variant="primary"
          type="button"
          className={`font-hud relative z-content min-h-11 w-full rounded-full border border-status-success/35 bg-action-primary px-6 py-2 text-center text-body-sm font-strong uppercase tracking-control-wide text-content-on-action shadow-elev-3 transition hover:bg-action-primary-hover disabled:cursor-not-allowed ${guessSubmitted ? 'opacity-50' : 'disabled:opacity-70'}`}
          onClick={handleFinalizeClick}
          disabled={!canFinalizeGuess}
        >
          {finalizeLabel}
        </Button>
      </div>
    </>
  );
}
