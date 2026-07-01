import { MapContainer, Marker, Polyline, TileLayer, useMap, useMapEvents } from 'react-leaflet';
import { useEffect, useMemo, useRef, useState } from 'react';
import L from 'leaflet';

type RoundPlayerResult = {
  userId: string;
  lat: number;
  lng: number;
  score: number;
  distanceKm: number;
};

type RoundResult = {
  roundId?: string;
  actualLocation: { lat: number; lng: number };
  players: Record<string, RoundPlayerResult>;
};

type Props = {
  onGuess?: (lat: number, lng: number) => void;
  guess?: { lat: number; lng: number };
  mode?: 'guess' | 'result';
  result?: RoundResult;
  results?: RoundResult[];
  interactiveInResult?: boolean;
  guessAvatarUrl?: string;
  guessAvatarFallback?: string;
  resultPlayerAvatars?: Record<string, string | undefined>;
  resultPlayerFallbacks?: Record<string, string | undefined>;
  resultPlayerBorderColors?: Record<string, string | undefined>;
};

function ClickCapture({ onGuess }: { onGuess: (lat: number, lng: number) => void }) {
  useMapEvents({
    click(e) {
      onGuess(e.latlng.lat, e.latlng.lng);
    }
  });
  return null;
}

function normalizeLng(lng: number) {
  return ((((lng + 180) % 360) + 360) % 360) - 180;
}

function closestWrappedLng(lng: number, referenceLng: number) {
  const normalized = normalizeLng(lng);
  return normalized + 360 * Math.round((referenceLng - normalized) / 360);
}

type WrappedCandidate = {
  latlng: L.LatLng;
  point: L.Point;
};

function getWrappedCandidates(map: L.Map, lat: number, lng: number): WrappedCandidate[] {
  const zoom = map.getZoom();
  const worldBounds = map.getPixelWorldBounds(zoom);
  if (!worldBounds) {
    const latlng = L.latLng(lat, lng);
    return [{ latlng, point: map.latLngToContainerPoint(latlng) }];
  }

  const worldWidth = worldBounds.getSize().x;
  const projected = map.project(L.latLng(lat, normalizeLng(lng)), zoom);
  const container = map.getContainer();
  const copies = Math.max(1, Math.ceil(container.clientWidth / worldWidth) + 2);
  const candidates: WrappedCandidate[] = [];

  for (let shift = -copies; shift <= copies; shift += 1) {
    const latlng = map.unproject(L.point(projected.x + shift * worldWidth, projected.y), zoom);
    candidates.push({ latlng, point: map.latLngToContainerPoint(latlng) });
  }

  return candidates;
}

function isPointVisible(point: L.Point, map: L.Map) {
  const { clientWidth, clientHeight } = map.getContainer();
  return point.x >= 0 && point.x <= clientWidth && point.y >= 0 && point.y <= clientHeight;
}

function distanceBetweenPoints(a: L.Point, b: L.Point) {
  return Math.hypot(a.x - b.x, a.y - b.y);
}

function pickBestWrappedCandidate(map: L.Map, lat: number, lng: number, anchor: L.Point) {
  const candidates = getWrappedCandidates(map, lat, lng);
  const visible = candidates.filter((candidate) => isPointVisible(candidate.point, map));
  const pool = visible.length > 0 ? visible : candidates;

  return pool.reduce((best, candidate) => {
    if (!best) return candidate;
    return distanceBetweenPoints(candidate.point, anchor) < distanceBetweenPoints(best.point, anchor) ? candidate : best;
  }, pool[0]);
}

function FitToResult({ result }: { result: RoundResult }) {
  const map = useMap();
  const fittedRoundKeyRef = useRef<string>('');

  const fitKey = useMemo(() => {
    if (result.roundId) return result.roundId;
    const visiblePoints = Object.values(result.players)
      .filter((p) => hasVisibleGuess(p))
      .map((p) => `${p.userId}:${p.lat.toFixed(4)},${p.lng.toFixed(4)}`)
      .sort()
      .join('|');
    return `${result.actualLocation.lat.toFixed(4)},${result.actualLocation.lng.toFixed(4)}::${visiblePoints}`;
  }, [result]);

  useEffect(() => {
    if (fittedRoundKeyRef.current === fitKey) return;

    const actualLng = normalizeLng(result.actualLocation.lng);
    const points: [number, number][] = [[result.actualLocation.lat, actualLng]];
    Object.values(result.players).forEach((p) => {
      if (hasVisibleGuess(p)) {
        points.push([p.lat, closestWrappedLng(p.lng, actualLng)]);
      }
    });
    let cancelled = false;
    const timer = window.setTimeout(() => {
      if (cancelled) return;
      const container = map.getContainer();
      if (!container || !container.isConnected) return;
      try {
        map.invalidateSize(false);
        map.fitBounds(points, { padding: [40, 40], maxZoom: 8, animate: false });
        fittedRoundKeyRef.current = fitKey;
      } catch {
        // Ignore transient map lifecycle races during UI transitions.
      }
    }, 0);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [fitKey, map, result]);
  return null;
}

function FitToResults({ results }: { results: RoundResult[] }) {
  const map = useMap();
  const fittedKeyRef = useRef<string>('');

  const fitKey = useMemo(() => {
    return results
      .map((round) => {
        const guesses = Object.values(round.players)
          .filter((player) => hasVisibleGuess(player))
          .map((player) => `${player.userId}:${player.lat.toFixed(2)},${player.lng.toFixed(2)}`)
          .sort()
          .join('|');
        return `${round.roundId || round.actualLocation.lat}:${round.actualLocation.lat.toFixed(2)},${round.actualLocation.lng.toFixed(2)}::${guesses}`;
      })
      .join('||');
  }, [results]);

  useEffect(() => {
    if (!results.length || fittedKeyRef.current === fitKey) return;

    const points: [number, number][] = [];
    results.forEach((round) => {
      const actualLng = normalizeLng(round.actualLocation.lng);
      points.push([round.actualLocation.lat, actualLng]);
      Object.values(round.players).forEach((player) => {
        if (hasVisibleGuess(player)) {
          points.push([player.lat, closestWrappedLng(player.lng, actualLng)]);
        }
      });
    });

    if (!points.length) return;

    let cancelled = false;
    const timer = window.setTimeout(() => {
      if (cancelled) return;
      const container = map.getContainer();
      if (!container || !container.isConnected) return;
      try {
        map.invalidateSize(false);
        map.fitBounds(points, { padding: [18, 18], maxZoom: 5, animate: false });
        fittedKeyRef.current = fitKey;
      } catch {
        // Ignore transient map lifecycle races during UI transitions.
      }
    }, 0);

    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [fitKey, map, results]);

  return null;
}

type WrappedResultLayerProps = {
  result: RoundResult;
  resultPlayerAvatars?: Record<string, string | undefined>;
  resultPlayerFallbacks?: Record<string, string | undefined>;
  resultPlayerBorderColors?: Record<string, string | undefined>;
};

function WrappedResultLayer({
  result,
  resultPlayerAvatars,
  resultPlayerFallbacks,
  resultPlayerBorderColors
}: WrappedResultLayerProps) {
  const map = useMap();
  const [viewportVersion, setViewportVersion] = useState(0);
  const actualLocationIcon = useMemo(
    () => createActualLocationIcon(result.actualLocation.lat, result.actualLocation.lng),
    [result.actualLocation.lat, result.actualLocation.lng]
  );

  useMapEvents({
    move() {
      setViewportVersion((value) => value + 1);
    },
    zoom() {
      setViewportVersion((value) => value + 1);
    },
    resize() {
      setViewportVersion((value) => value + 1);
    }
  });

  const layout = useMemo(() => {
    const container = map.getContainer();
    const centerPoint = L.point(container.clientWidth / 2, container.clientHeight / 2);
    const actual = pickBestWrappedCandidate(map, result.actualLocation.lat, result.actualLocation.lng, centerPoint);
    const players = Object.entries(result.players)
      .filter(([, p]) => hasVisibleGuess(p))
      .map(([id, p]) => ({
        id,
        result: p,
        displayedLatLng: pickBestWrappedCandidate(map, p.lat, p.lng, actual.point).latlng
      }));

    return {
      actualLatLng: actual.latlng,
      players
    };
  }, [map, result, viewportVersion]);

  return (
    <>
      <Marker
        position={[layout.actualLatLng.lat, layout.actualLatLng.lng]}
        icon={actualLocationIcon}
        zIndexOffset={5000}
        title="Open actual location in Google Maps"
      />
      {layout.players.map(({ id, result: player, displayedLatLng }) => (
        <Polyline
          key={`${id}-line`}
          positions={[
            [displayedLatLng.lat, displayedLatLng.lng],
            [layout.actualLatLng.lat, layout.actualLatLng.lng]
          ]}
          pathOptions={{ color: '#1f2933', dashArray: '2 6', weight: 1.5, opacity: 0.96 }}
        />
      ))}
      {layout.players.map(({ id, displayedLatLng }) => (
        <Marker
          key={`${id}-guess`}
          position={[displayedLatLng.lat, displayedLatLng.lng]}
          icon={createAvatarMarkerIcon({
            avatarUrl: resultPlayerAvatars?.[id],
            fallback: resultPlayerFallbacks?.[id],
            borderColor: resultPlayerBorderColors?.[id],
            size: 38
          })}
        />
      ))}
    </>
  );
}

type WrappedResultsLayerProps = {
  results: RoundResult[];
  resultPlayerAvatars?: Record<string, string | undefined>;
  resultPlayerFallbacks?: Record<string, string | undefined>;
  resultPlayerBorderColors?: Record<string, string | undefined>;
};

function WrappedResultsLayer({
  results,
  resultPlayerAvatars,
  resultPlayerFallbacks,
  resultPlayerBorderColors
}: WrappedResultsLayerProps) {
  const map = useMap();
  const [viewportVersion, setViewportVersion] = useState(0);

  useMapEvents({
    move() {
      setViewportVersion((value) => value + 1);
    },
    zoom() {
      setViewportVersion((value) => value + 1);
    },
    resize() {
      setViewportVersion((value) => value + 1);
    }
  });

  const layout = useMemo(() => {
    const container = map.getContainer();
    const centerPoint = L.point(container.clientWidth / 2, container.clientHeight / 2);

    return results.map((round, roundIndex) => {
      const actual = pickBestWrappedCandidate(map, round.actualLocation.lat, round.actualLocation.lng, centerPoint);
      const players = Object.entries(round.players)
        .filter(([, player]) => hasVisibleGuess(player))
        .map(([id, player]) => ({
          id,
          roundIndex,
          displayedLatLng: pickBestWrappedCandidate(map, player.lat, player.lng, actual.point).latlng
        }));

      return {
        roundIndex,
        actualLocation: round.actualLocation,
        actualLatLng: actual.latlng,
        players
      };
    });
  }, [map, results, viewportVersion]);

  return (
    <>
      {layout.map((round) => (
        <Marker
          key={`actual-${round.roundIndex}`}
          position={[round.actualLatLng.lat, round.actualLatLng.lng]}
          icon={createActualLocationIcon(
            round.actualLocation.lat,
            round.actualLocation.lng,
            round.roundIndex + 1
          )}
          zIndexOffset={4000}
          title={`Round ${round.roundIndex + 1}: Open actual location in Google Maps`}
        />
      ))}
      {layout.flatMap((round) =>
        round.players.map((player) => (
          <Polyline
            key={`${player.id}-${player.roundIndex}-line`}
            positions={[
              [player.displayedLatLng.lat, player.displayedLatLng.lng],
              [round.actualLatLng.lat, round.actualLatLng.lng]
            ]}
            pathOptions={{ color: '#1f2933', dashArray: '2 6', weight: 1.5, opacity: 0.96 }}
          />
        ))
      )}
      {layout.flatMap((round) =>
        round.players.map((player) => (
          <Marker
            key={`${player.id}-${player.roundIndex}`}
            position={[player.displayedLatLng.lat, player.displayedLatLng.lng]}
            icon={createAvatarMarkerIcon({
              avatarUrl: resultPlayerAvatars?.[player.id],
              fallback: resultPlayerFallbacks?.[player.id],
              borderColor: resultPlayerBorderColors?.[player.id],
              size: 30
            })}
          />
        ))
      )}
    </>
  );
}

function InvalidateOnResize() {
  const map = useMap();
  const savedViewRef = useRef<{ center: L.LatLng; zoom: number } | null>(null);
  const lastSizeRef = useRef({ width: 0, height: 0 });

  useEffect(() => {
    const container = map.getContainer();
    if (!container || typeof ResizeObserver === 'undefined') return;

    let frame = 0;
    const observer = new ResizeObserver(() => {
      const width = container.clientWidth;
      const height = container.clientHeight;
      const lastSize = lastSizeRef.current;

      if (width === 0 || height === 0) {
        savedViewRef.current = { center: map.getCenter(), zoom: map.getZoom() };
        lastSizeRef.current = { width, height };
        return;
      }

      const wasHidden = lastSize.width === 0 || lastSize.height === 0;
      lastSizeRef.current = { width, height };

      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        try {
          map.invalidateSize({ animate: false });
          if (wasHidden && savedViewRef.current) {
            const { center, zoom } = savedViewRef.current;
            savedViewRef.current = null;
            map.setView(center, zoom, { animate: false });
          }
        } catch {
          // Ignore transient lifecycle races while view transitions.
        }
      });
    });

    observer.observe(container);
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
    };
  }, [map]);

  return null;
}

function SafeMapUnmount() {
  const map = useMap();

  useEffect(() => {
    return () => {
      try {
        map.stop();
        const maybeStop = (map as L.Map & { _stop?: () => void })._stop;
        if (typeof maybeStop === 'function') maybeStop.call(map);
      } catch {
        // Best-effort shutdown to avoid teardown races with queued zoom frames.
      }
    };
  }, [map]);

  return null;
}

function hasVisibleGuess(p: RoundPlayerResult) {
  return !(p.lat === 0 && p.lng === 0 && p.score === 0);
}

export function buildGoogleMapsLocationUrl(lat: number, lng: number) {
  const params = new URLSearchParams({
    api: '1',
    map_action: 'pano',
    viewpoint: `${lat},${normalizeLng(lng)}`
  });
  return `https://www.google.com/maps/@?${params.toString()}`;
}

function createAvatarMarkerIcon({
  avatarUrl,
  fallback,
  borderColor,
  size
}: {
  avatarUrl?: string;
  fallback?: string;
  borderColor?: string;
  size: number;
}) {
  const normalizedFallback = (fallback || 'P').slice(0, 1).toUpperCase();
  const pinClass = avatarUrl ? 'guessAvatarPin' : 'guessAvatarPin fallback';
  const safeUrl = avatarUrl ? avatarUrl.replace(/&/g, '&amp;').replace(/"/g, '&quot;') : '';
  const safeBorderColor = normalizePinBorderColor(borderColor);
  const borderStyle = safeBorderColor ? `;--pin-border:${safeBorderColor}` : '';
  const avatarHtml = avatarUrl
    ? `<img src="${safeUrl}" alt="Player avatar" />`
    : normalizedFallback;

  return L.divIcon({
    className: 'guess-avatar-marker',
    html: `<div class="${pinClass}" style="--pin-size:${size}px${borderStyle}">${avatarHtml}</div>`,
    iconSize: [size, size],
    iconAnchor: [size / 2, size / 2]
  });
}

function normalizePinBorderColor(color?: string) {
  if (!color) return '';
  const trimmed = color.trim();
  return /^#[0-9a-fA-F]{3}([0-9a-fA-F]{3})?$/.test(trimmed) ? trimmed : '';
}

export function createActualLocationIcon(lat: number, lng: number, roundNumber?: number) {
  const label = typeof roundNumber === 'number' ? String(roundNumber) : '';
  const content = label
    ? `<span class="actualLocationNumber">${label}</span>`
    : '<span class="actualLocationFlag"></span>';
  const mapsUrl = buildGoogleMapsLocationUrl(lat, lng).replace(/&/g, '&amp;');
  const accessibleLabel = label
    ? `Open round ${label} actual location in Google Maps`
    : 'Open actual location in Google Maps';

  return L.divIcon({
    className: 'actual-location-marker',
    html: `<a class="actualLocationLink" href="${mapsUrl}" target="_blank" rel="noopener noreferrer" aria-label="${accessibleLabel}"><span class="actualLocationPin" aria-hidden="true">${content}</span></a>`,
    iconSize: [30, 30],
    iconAnchor: [15, 15]
  });
}

export default function GuessMap({
  onGuess,
  guess,
  mode = 'guess',
  result,
  results,
  interactiveInResult = false,
  guessAvatarUrl,
  guessAvatarFallback,
  resultPlayerAvatars,
  resultPlayerFallbacks,
  resultPlayerBorderColors
}: Props) {
  const center = useMemo<[number, number]>(() => [0, 0], []);
  const interactive = mode === 'guess' || (mode === 'result' && interactiveInResult);
  const mapClassName = [
    interactive ? 'minimap-interactive' : 'minimap-static',
    mode === 'result' ? 'minimap-result' : '',
  ].filter(Boolean).join(' ');
  const guessMarkerIcon = useMemo(() => {
    return createAvatarMarkerIcon({
      avatarUrl: guessAvatarUrl,
      fallback: guessAvatarFallback,
      size: 38 / 1.25
    });
  }, [guessAvatarUrl, guessAvatarFallback]);

  return (
    <MapContainer
      className={mapClassName}
      center={center}
      zoom={1}
      minZoom={1}
      worldCopyJump
      style={{ height: '100%', width: '100%' }}
      attributionControl={false}
      zoomControl={interactive}
      dragging={interactive}
      scrollWheelZoom={interactive}
      doubleClickZoom={interactive}
      touchZoom={interactive}
      boxZoom={interactive}
      keyboard={interactive}
    >
      <TileLayer
        url="https://mt{s}.google.com/vt/lyrs=m&x={x}&y={y}&z={z}&hl=en&scale=2"
        subdomains={['0', '1', '2', '3']}
      />
      <SafeMapUnmount />
      <InvalidateOnResize />
      {mode === 'guess' && onGuess ? <ClickCapture onGuess={onGuess} /> : null}
      {mode === 'guess' && guess ? <Marker position={[guess.lat, guess.lng]} icon={guessMarkerIcon} /> : null}
      {mode === 'result' && result ? <FitToResult result={result} /> : null}
      {mode === 'result' && !result && results?.length ? <FitToResults results={results} /> : null}
      {mode === 'result' && result ? (
        <WrappedResultLayer
          result={result}
          resultPlayerAvatars={resultPlayerAvatars}
          resultPlayerFallbacks={resultPlayerFallbacks}
          resultPlayerBorderColors={resultPlayerBorderColors}
        />
      ) : null}
      {mode === 'result' && !result && results?.length ? (
        <WrappedResultsLayer
          results={results}
          resultPlayerAvatars={resultPlayerAvatars}
          resultPlayerFallbacks={resultPlayerFallbacks}
          resultPlayerBorderColors={resultPlayerBorderColors}
        />
      ) : null}
    </MapContainer>
  );
}
