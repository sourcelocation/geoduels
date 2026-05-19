import { useEffect, useMemo, useRef, useState, type RefObject } from 'react';

type StreetViewPaneProps = {
  src: string;
  interactive: boolean;
  resetCount: number;
  iframeRef: RefObject<HTMLIFrameElement>;
  onFrameFocus: () => void;
  onHeadingChange: (heading: number) => void;
};

type StreetViewConfig = {
  key: string;
  pano: string;
  heading: number;
  pitch: number;
};

type GoogleMapsEventListener = {
  remove: () => void;
};

type GoogleStreetViewPov = {
  heading?: number;
  pitch?: number;
  zoom?: number;
};

type GoogleStreetViewPanorama = {
  addListener: (eventName: 'pov_changed', handler: () => void) => GoogleMapsEventListener;
  getPov: () => GoogleStreetViewPov;
  setPano: (pano: string) => void;
  setPov: (pov: GoogleStreetViewPov) => void;
};

type GoogleMapsNamespace = {
  StreetViewPanorama: new (
    element: HTMLElement,
    options: {
      pano: string;
      pov: Required<Pick<GoogleStreetViewPov, 'heading' | 'pitch'>> & Pick<GoogleStreetViewPov, 'zoom'>;
      visible: boolean;
      addressControl: boolean;
      fullscreenControl: boolean;
      motionTracking: boolean;
      motionTrackingControl: boolean;
      showRoadLabels: boolean;
    },
  ) => GoogleStreetViewPanorama;
};

declare global {
  interface Window {
    google?: {
      maps?: GoogleMapsNamespace;
    };
  }
}

let mapsScriptPromise: Promise<void> | null = null;
let mapsScriptKey = '';

function normalizeHeading(value: number) {
  if (!Number.isFinite(value)) return 0;
  return ((value % 360) + 360) % 360;
}

function readNumberParam(params: URLSearchParams, key: string, fallback: number) {
  const raw = params.get(key);
  if (raw === null) return fallback;
  const value = Number(raw);
  return Number.isFinite(value) ? value : fallback;
}

function getStreetViewConfig(src: string): StreetViewConfig | null {
  try {
    const url = new URL(src);
    const key = url.searchParams.get('key')?.trim() || '';
    const pano = url.searchParams.get('pano')?.trim() || '';
    if (!key || key === 'NO_KEY_DEFINED' || !pano) return null;

    return {
      key,
      pano,
      heading: normalizeHeading(readNumberParam(url.searchParams, 'heading', 0)),
      pitch: readNumberParam(url.searchParams, 'pitch', 0),
    };
  } catch {
    return null;
  }
}

export function getStreetViewInitialHeading(src: string) {
  try {
    const url = new URL(src);
    return normalizeHeading(readNumberParam(url.searchParams, 'heading', 0));
  } catch {
    return 0;
  }
}

function loadGoogleMapsScript(key: string) {
  if (typeof window === 'undefined') {
    return Promise.reject(new Error('Google Maps is only available in the browser'));
  }
  if (window.google?.maps?.StreetViewPanorama) {
    return Promise.resolve();
  }
  if (mapsScriptPromise && mapsScriptKey === key) {
    return mapsScriptPromise;
  }

  const existingScript = document.querySelector<HTMLScriptElement>('script[data-geoduels-google-maps="true"]');
  if (existingScript) {
    mapsScriptPromise = new Promise((resolve, reject) => {
      existingScript.addEventListener('load', () => resolve(), { once: true });
      existingScript.addEventListener('error', () => reject(new Error('Failed to load Google Maps JavaScript API')), {
        once: true,
      });
    });
    mapsScriptKey = key;
    return mapsScriptPromise;
  }

  mapsScriptPromise = new Promise((resolve, reject) => {
    const script = document.createElement('script');
    const params = new URLSearchParams({
      key,
      v: 'weekly',
    });
    script.src = `https://maps.googleapis.com/maps/api/js?${params.toString()}`;
    script.async = true;
    script.defer = true;
    script.dataset.geoduelsGoogleMaps = 'true';
    script.addEventListener('load', () => resolve(), { once: true });
    script.addEventListener('error', () => reject(new Error('Failed to load Google Maps JavaScript API')), {
      once: true,
    });
    document.head.appendChild(script);
  });
  mapsScriptKey = key;
  return mapsScriptPromise;
}

export default function StreetViewPane({
  src,
  interactive,
  resetCount,
  iframeRef,
  onFrameFocus,
  onHeadingChange,
}: StreetViewPaneProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const panoramaRef = useRef<GoogleStreetViewPanorama | null>(null);
  const povListenerRef = useRef<GoogleMapsEventListener | null>(null);
  const config = useMemo(() => getStreetViewConfig(src), [src]);
  const initialHeading = useMemo(() => getStreetViewInitialHeading(src), [src]);
  const [panoramaReady, setPanoramaReady] = useState(false);
  const [loadFailed, setLoadFailed] = useState(false);

  useEffect(() => {
    onHeadingChange(initialHeading);
  }, [initialHeading, onHeadingChange]);

  useEffect(() => {
    let cancelled = false;
    setPanoramaReady(false);
    setLoadFailed(false);
    panoramaRef.current = null;
    povListenerRef.current?.remove();
    povListenerRef.current = null;

    if (!config) return;

    loadGoogleMapsScript(config.key)
      .then(() => {
        if (cancelled || !containerRef.current || !window.google?.maps?.StreetViewPanorama) return;

        const panorama = new window.google.maps.StreetViewPanorama(containerRef.current, {
          pano: config.pano,
          pov: {
            heading: config.heading,
            pitch: config.pitch,
            zoom: 0,
          },
          visible: true,
          addressControl: false,
          fullscreenControl: false,
          motionTracking: false,
          motionTrackingControl: false,
          showRoadLabels: false,
        });

        const handlePovChange = () => {
          const pov = panorama.getPov();
          onHeadingChange(normalizeHeading(Number(pov.heading ?? config.heading)));
        };

        panoramaRef.current = panorama;
        povListenerRef.current = panorama.addListener('pov_changed', handlePovChange);
        handlePovChange();
        setPanoramaReady(true);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      });

    return () => {
      cancelled = true;
      povListenerRef.current?.remove();
      povListenerRef.current = null;
      panoramaRef.current = null;
    };
  }, [config, onHeadingChange]);

  useEffect(() => {
    const panorama = panoramaRef.current;
    if (!panorama || !config) return;

    panorama.setPano(config.pano);
    panorama.setPov({
      heading: config.heading,
      pitch: config.pitch,
      zoom: 0,
    });
    onHeadingChange(config.heading);
  }, [config, onHeadingChange, resetCount]);

  const showFallbackFrame = !config || loadFailed || !panoramaReady;

  return (
    <>
      <div
        ref={containerRef}
        aria-label="Street View"
        className={[
          'absolute inset-0',
          panoramaReady ? '' : 'pointer-events-none opacity-0',
          interactive ? '' : 'pointer-events-none',
        ].join(' ')}
      />
      {showFallbackFrame ? (
        <iframe
          key={`${src}-${resetCount}`}
          ref={iframeRef}
          title="Street View"
          src={src}
          tabIndex={-1}
          onFocus={onFrameFocus}
          className={`absolute left-0 top-[-75px] h-[calc(100%+75px)] w-full border-0 ${interactive ? '' : 'pointer-events-none'}`}
          allowFullScreen
          loading="eager"
        />
      ) : null}
    </>
  );
}
