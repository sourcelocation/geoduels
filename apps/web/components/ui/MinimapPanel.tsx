import type { MouseEvent as ReactMouseEvent, PointerEvent as ReactPointerEvent, ReactNode } from 'react';
import { useEffect, useRef, useState } from 'react';
import { ChevronLeft, MapPin, MoveDiagonal2 } from 'lucide-react';

type Props = {
  children: ReactNode;
  onFinalize: () => void;
  canFinalizeGuess: boolean;
  guessSubmitted: boolean;
  roundKey?: string;
};

type MinimapSize = {
  width: number;
  height: number;
};

const DESKTOP_BREAKPOINT_PX = 768;
const RIGHT_GUTTER_PX = 80;
const MIN_EXPANDED_SIZE: MinimapSize = { width: 460, height: 360 };
const EXPANDED_SIZE_STORAGE_KEY = 'geoduels.minimapExpandedSize';
const GESTURE_MOVE_THRESHOLD_PX = 6;

const finalizeButtonClassName =
  'font-hud relative z-10 min-h-11 w-full rounded-pill border border-emerald-200/35 bg-cta-gradient px-6 py-2 text-center text-sm uppercase tracking-[0.15em] text-white shadow-elev-3 transition hover:brightness-110 disabled:cursor-not-allowed';

function clampExpandedSize(size: MinimapSize): MinimapSize {
  if (typeof window === 'undefined') return size;

  return {
    width: Math.round(
      Math.min(Math.max(size.width, MIN_EXPANDED_SIZE.width), window.innerWidth - RIGHT_GUTTER_PX - 16)
    ),
    height: Math.round(
      Math.min(Math.max(size.height, MIN_EXPANDED_SIZE.height), window.innerHeight - 32)
    ),
  };
}

function loadExpandedSize(): MinimapSize | null {
  if (typeof window === 'undefined') return null;

  try {
    const stored = window.localStorage.getItem(EXPANDED_SIZE_STORAGE_KEY);
    if (!stored) return null;
    const parsed = JSON.parse(stored) as Partial<MinimapSize>;
    if (!Number.isFinite(parsed.width) || !Number.isFinite(parsed.height)) return null;
    return clampExpandedSize({
      width: Number(parsed.width),
      height: Number(parsed.height),
    });
  } catch {
    return null;
  }
}

export default function MinimapPanel({
  children,
  onFinalize,
  canFinalizeGuess,
  guessSubmitted,
  roundKey,
}: Props) {
  const panelRef = useRef<HTMLDivElement | null>(null);
  const activePointerRef = useRef<{ id: number; x: number; y: number } | null>(null);
  const activeResizeRef = useRef<({ id: number; x: number; y: number } & MinimapSize) | null>(null);
  const resizeLockedRef = useRef(false);
  const suppressNextPanelClickRef = useRef(false);
  const [desktopHovered, setDesktopHovered] = useState(false);
  const [mapViewOpen, setMapViewOpen] = useState(false);
  const [isDesktop, setIsDesktop] = useState(false);
  const [expandedSize, setExpandedSize] = useState<MinimapSize | null>(null);

  useEffect(() => {
    if (typeof window === 'undefined') return;

    const mediaQuery = window.matchMedia(`(min-width: ${DESKTOP_BREAKPOINT_PX}px)`);
    const syncViewport = () => {
      const desktop = mediaQuery.matches;
      setIsDesktop(desktop);
      if (desktop) setMapViewOpen(false);
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
    setMapViewOpen(false);
  }, [roundKey]);

  useEffect(() => {
    if (typeof window === 'undefined') return;
    setExpandedSize(loadExpandedSize());
  }, []);

  useEffect(() => {
    if (typeof window === 'undefined') return;

    const releaseResizeLock = () => {
      resizeLockedRef.current = false;
      activeResizeRef.current = null;
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

  const expanded = desktopHovered;
  const mobileExpanded = !isDesktop && mapViewOpen;
  const reserveRightGutter = isDesktop;
  const finalizeLabel = guessSubmitted ? 'Waiting for opponent...' : canFinalizeGuess ? 'Guess' : 'Place Pin';
  const finalizeDisabledClassName = guessSubmitted ? 'opacity-45' : 'disabled:opacity-70';

  const beginPointerGesture = (event: ReactPointerEvent) => {
    activePointerRef.current = {
      id: event.pointerId,
      x: event.clientX,
      y: event.clientY,
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

  const handleFinalizeClick = (event: ReactMouseEvent<HTMLButtonElement>) => {
    event.stopPropagation();
    onFinalize();
  };

  const handlePanelClick = () => {
    if (suppressNextPanelClickRef.current) {
      suppressNextPanelClickRef.current = false;
      return;
    }

    if (!isDesktop) {
      setMapViewOpen(true);
    }
  };

  const handleBackdropClick = () => {
    if (suppressNextPanelClickRef.current) {
      suppressNextPanelClickRef.current = false;
      return;
    }

    setMapViewOpen(false);
  };

  const finalizeButton = (
    <button
      type="button"
      className={`${finalizeButtonClassName} ${finalizeDisabledClassName}`}
      onClick={handleFinalizeClick}
      disabled={!canFinalizeGuess || guessSubmitted}
    >
      {finalizeLabel}
    </button>
  );

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
      height: rect.height,
    };
    resizeLockedRef.current = true;
    setDesktopHovered(true);
  };

  const resize = (event: ReactPointerEvent<HTMLButtonElement>) => {
    const activeResize = activeResizeRef.current;
    if (!activeResize || activeResize.id !== event.pointerId) return;

    setExpandedSize(
      clampExpandedSize({
        width: activeResize.width + activeResize.x - event.clientX,
        height: activeResize.height + activeResize.y - event.clientY,
      })
    );
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

  const desktopExpandedStyle =
    isDesktop && expanded && expandedSize
      ? {
          width: `${expandedSize.width}px`,
          height: `${expandedSize.height}px`,
        }
      : {};

  if (!isDesktop && !mapViewOpen) {
    return (
      <button
        type="button"
        aria-label="Open map to place guess"
        onClick={() => setMapViewOpen(true)}
        className="font-hud absolute bottom-4 right-3 z-40 flex min-h-11 items-center gap-2 rounded-pill border border-white/15 bg-hudBg px-4 text-sm uppercase tracking-[0.12em] text-white shadow-elev-2 backdrop-blur-hud transition hover:bg-white/10"
      >
        <MapPin size={16} strokeWidth={2.4} aria-hidden="true" />
        Guess
      </button>
    );
  }

  return (
    <>
      {!isDesktop && mobileExpanded ? (
        <button
          type="button"
          aria-label="Collapse minimap"
          className="absolute inset-0 z-20 cursor-default bg-transparent"
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
        className={`absolute bottom-0 right-0 z-30 flex w-full flex-col gap-2 p-3 transition-[width,height] duration-150 ease-out md:bottom-4 md:right-4 md:w-[min(34vw,460px)] md:h-[min(33vh,360px)] md:p-0 ${
          expanded ? 'md:w-[min(90vw,800px)] md:h-[min(52vh,560px)]' : ''
        }`}
        style={{
          right: reserveRightGutter ? `${RIGHT_GUTTER_PX}px` : '0px',
          width: isDesktop ? undefined : reserveRightGutter ? `calc(100% - ${RIGHT_GUTTER_PX}px)` : '100%',
          ...desktopExpandedStyle,
        }}
      >
        {isDesktop && expanded ? <div aria-hidden="true" className="absolute -inset-24 z-0" /> : null}

        <div
          onClick={handlePanelClick}
          onPointerDown={beginPointerGesture}
          onPointerMove={trackPointerGesture}
          onPointerUp={endPointerGesture}
          onPointerCancel={endPointerGesture}
          className={`group relative z-10 min-h-0 w-full origin-bottom-right overflow-hidden rounded-panel border border-white/20 bg-slate-900/70 shadow-elev-4 transition-[height,opacity,box-shadow] duration-150 ease-out ${
            expanded
              ? 'h-[50vh] min-h-[280px] opacity-100 sm:h-[55vh] sm:min-h-[320px]'
              : 'h-[22vh] min-h-[150px] opacity-70 sm:h-[27vh] sm:min-h-[190px]'
          } md:h-auto md:min-h-0 md:flex-1 md:opacity-85 md:hover:opacity-100`}
        >
          {children}
        </div>

        {finalizeButton}

        {isDesktop && expanded ? (
          <button
            type="button"
            aria-label="Resize minimap"
            title="Resize minimap"
            className="absolute -left-3 -top-3 z-20 flex h-8 w-8 touch-none items-center justify-center rounded-lg border border-white/20 bg-slate-950/90 text-emerald-100 shadow-elev-3 transition-colors hover:border-emerald-200/50 hover:bg-slate-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-300"
            onPointerDown={beginResize}
            onPointerMove={resize}
            onPointerUp={finishResize}
            onPointerCancel={finishResize}
          >
            <MoveDiagonal2 size={16} strokeWidth={2.5} />
          </button>
        ) : null}

        {!isDesktop ? (
          <button
            type="button"
            aria-label="Back to Street View"
            onClick={() => setMapViewOpen(false)}
            className="font-hud absolute bottom-[calc(1rem+3.25rem)] right-3 z-40 flex min-h-11 items-center gap-2 rounded-pill border border-white/15 bg-hudBg px-4 text-sm uppercase tracking-[0.12em] text-white shadow-elev-2 backdrop-blur-hud transition hover:bg-white/10"
          >
            <ChevronLeft size={16} strokeWidth={2.4} aria-hidden="true" />
            Back
          </button>
        ) : null}
      </div>
    </>
  );
}