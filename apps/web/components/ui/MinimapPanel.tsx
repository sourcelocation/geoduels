import type { MouseEvent as ReactMouseEvent, PointerEvent as ReactPointerEvent, ReactNode } from 'react';
import { useEffect, useRef, useState } from 'react';
import { ChevronLeft, MapPin } from 'lucide-react';

type Props = {
  children: ReactNode;
  onFinalize: () => void;
  canFinalizeGuess: boolean;
  guessSubmitted: boolean;
  roundKey?: string;
};

const GESTURE_MOVE_THRESHOLD_PX = 6;
const finalizeButtonClassName =
  'font-hud relative z-10 min-h-11 w-full rounded-pill border border-emerald-200/35 bg-cta-gradient px-6 py-2 text-center text-sm uppercase tracking-[0.15em] text-white shadow-elev-3 transition hover:brightness-110 disabled:cursor-not-allowed';

export default function MinimapPanel({
  children,
  onFinalize,
  canFinalizeGuess,
  guessSubmitted,
  roundKey,
}: Props) {
  const panelRef = useRef<HTMLDivElement | null>(null);
  const activePointerRef = useRef<{ id: number; x: number; y: number } | null>(null);
  const resizeLockedRef = useRef(false);
  const suppressNextPanelClickRef = useRef(false);
  const [desktopHovered, setDesktopHovered] = useState(false);
  const [mapViewOpen, setMapViewOpen] = useState(false);
  const [isDesktop, setIsDesktop] = useState(false);

  useEffect(() => {
    if (typeof window === 'undefined') return;

    const mediaQuery = window.matchMedia('(min-width: 768px)');
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

  const expanded = desktopHovered;
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

  const finalizeButton = (
    <button
      type="button"
      className={`${finalizeButtonClassName} ${finalizeDisabledClassName}`}
      onClick={handleFinalizeClick}
      disabled={!canFinalizeGuess}
    >
      {finalizeLabel}
    </button>
  );

  if (!isDesktop) {
    if (!mapViewOpen) {
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
        <div className="absolute inset-0 z-30 flex flex-col gap-2 p-3">
          <div className="min-h-0 flex-1 overflow-hidden rounded-panel border border-white/20 bg-slate-900/70 shadow-elev-4">
            {children}
          </div>
          {finalizeButton}
        </div>
        <button
          type="button"
          aria-label="Back to Street View"
          onClick={() => setMapViewOpen(false)}
          className="font-hud absolute bottom-[calc(1rem+3.25rem)] right-3 z-40 flex min-h-11 items-center gap-2 rounded-pill border border-white/15 bg-hudBg px-4 text-sm uppercase tracking-[0.12em] text-white shadow-elev-2 backdrop-blur-hud transition hover:bg-white/10"
        >
          <ChevronLeft size={16} strokeWidth={2.4} aria-hidden="true" />
          Back
        </button>
      </>
    );
  }

  return (
    <div
      ref={panelRef}
      onMouseEnter={(event) => {
        if (resizeLockedRef.current || event.buttons !== 0) {
          resizeLockedRef.current = true;
          return;
        }
        setDesktopHovered(true);
      }}
      onMouseLeave={(event) => {
        if (resizeLockedRef.current || event.buttons !== 0) {
          resizeLockedRef.current = true;
          return;
        }
        setDesktopHovered(false);
      }}
      className={`absolute bottom-0 right-0 z-30 flex w-full flex-col gap-2 p-3 transition-[width,height] duration-150 ease-out md:bottom-4 md:right-4 md:h-[min(33vh,360px)] md:w-[min(34vw,460px)] md:p-0 ${
        expanded ? 'md:h-[min(52vh,560px)] md:w-[min(90vw,800px)]' : ''
      }`}
    >
      <div
        onPointerDown={beginPointerGesture}
        onPointerMove={trackPointerGesture}
        onPointerUp={endPointerGesture}
        onPointerCancel={endPointerGesture}
        className={`group relative min-h-0 w-full origin-bottom-right overflow-hidden rounded-panel border border-white/20 bg-slate-900/70 shadow-elev-4 transition-[height,opacity,box-shadow] duration-150 ease-out md:h-auto md:min-h-0 md:flex-1 md:opacity-85 md:hover:opacity-100 ${
          expanded ? 'opacity-100' : 'opacity-70'
        }`}
      >
        {children}
      </div>
      {finalizeButton}
    </div>
  );
}
