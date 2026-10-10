import type { RuntimeConfig } from '../../../lib/runtime-config';
import { apiFetch, apiSocketURL, authHeaders, mergeHeaders } from '../../../lib/http';
import type { MatchKind, Snapshot } from '../../game/model/types';
import type { AuthSessionSnapshot } from '../../auth/session';

export type GameRuleset = 'moving' | 'no_move' | 'nmpz';
export type StreetNamesVisibility = 'shown' | 'hidden';
export type MultiplierMode = 'shared' | 'individual';
export type QueueVariant =
  | 'moving'
  | 'no_move'
  | 'nmpz'
  | 'moving_hidden'
  | 'no_move_hidden'
  | 'nmpz_hidden';
export type MatchConfig = {
  ruleset?: GameRuleset;
  streetNames?: StreetNamesVisibility;
  mapId?: string;
  mapName?: string;
  mapKey?: string;
  roundTimerMode?: 'none' | 'pressure' | 'fixed';
  roundTimeLimitMs?: number;
  pressureTimeLimitMs?: number;
  multiplierMode?: MultiplierMode;
};

export type MatchReturnTarget =
  | { kind: 'home' }
  | { kind: 'map'; mapId: string }
  | { kind: 'party'; partyId: string; partyInviteCode?: string };

export type QueueEvent =
  | { type: 'queue_status'; status: string; queuedAt?: number }
  | { type: 'match_found'; matchId: string }
  | { type: 'queue_error'; message: string };

export type MaintenancePhase = 'normal' | 'warning' | 'active';

export type MaintenanceStatus = {
  phase: MaintenancePhase;
  startsAt?: string;
  endsAt?: string;
  queuePaused: boolean;
  playPaused: boolean;
  message: string;
};

// How the queue's one connection ended: matched ('done'), asked to reconnect by a server that is
// restarting, or never reached a server at all, as while a restarting server's address moves over.
type QueueOutcome = 'done' | 'reconnect' | 'unreachable';

// A queue server that restarts asks its clients to reconnect, and another one serves them. Opening a
// connection that reaches no server is tried a few more times before the queue counts as unavailable.
const QUEUE_ATTEMPTS = 6;
const QUEUE_RETRY_MS = 750;

export async function streamQueue(
  config: RuntimeConfig,
  session: AuthSessionSnapshot,
  signal: AbortSignal,
  queues: QueueVariant[],
  onEvent: (event: QueueEvent) => void
) {
  let unreachable = 0;
  for (;;) {
    const outcome = await openQueue(config, session, signal, queues, onEvent);
    if (outcome === 'done') return;
    if (outcome === 'reconnect') {
      unreachable = 0;
    } else if (++unreachable >= QUEUE_ATTEMPTS) {
      throw new Error('Queue unavailable');
    }
    await waitOrAbort(QUEUE_RETRY_MS, signal);
  }
}

function waitOrAbort(ms: number, signal: AbortSignal) {
  return new Promise<void>((resolve, reject) => {
    const onAbort = () => {
      clearTimeout(timer);
      reject(new DOMException('Aborted', 'AbortError'));
    };
    const timer = setTimeout(() => {
      signal.removeEventListener('abort', onAbort);
      resolve();
    }, ms);
    if (signal.aborted) {
      onAbort();
      return;
    }
    signal.addEventListener('abort', onAbort, { once: true });
  });
}

function openQueue(
  config: RuntimeConfig,
  session: AuthSessionSnapshot,
  signal: AbortSignal,
  queues: QueueVariant[],
  onEvent: (event: QueueEvent) => void
): Promise<QueueOutcome> {
  const selectedQueues = (queues.length ? queues : ['moving']).join(',');
  const target = `${apiSocketURL(config, '/api/v2/queue/ws')}?accessToken=${encodeURIComponent(session.accessToken)}&queues=${encodeURIComponent(selectedQueues)}`;

  return new Promise<QueueOutcome>((resolve, reject) => {
    let settled = false;
    let found = false;
    let heard = false;
    let reconnect = false;
    const ws = new WebSocket(target);

    const cleanup = () => {
      signal.removeEventListener('abort', abort);
    };

    const settleResolve = (outcome: QueueOutcome) => {
      if (settled) return;
      settled = true;
      cleanup();
      resolve(outcome);
    };

    const settleReject = (error: Error) => {
      if (settled) return;
      settled = true;
      cleanup();
      reject(error);
    };

    const abort = () => {
      ws.close();
      settleReject(new DOMException('Aborted', 'AbortError'));
    };

    signal.addEventListener('abort', abort, { once: true });

    ws.onerror = () => {
      if (!heard) {
        settleResolve('unreachable');
        return;
      }
      settleReject(new Error('Queue unavailable'));
    };

    ws.onclose = () => {
      if (signal.aborted) {
        settleReject(new DOMException('Aborted', 'AbortError'));
        return;
      }
      if (found) {
        settleResolve('done');
        return;
      }
      if (reconnect) {
        settleResolve('reconnect');
        return;
      }
      if (!heard) {
        settleResolve('unreachable');
        return;
      }
      settleReject(new Error('Search cancelled'));
    };

    ws.onmessage = (evt) => {
      heard = true;
      let msg: any;
      try {
        msg = JSON.parse(String(evt.data));
      } catch {
        settleReject(new Error('Queue unavailable'));
        return;
      }

      const eventName = typeof msg?.type === 'string' ? msg.type : '';
      const payload = msg?.payload ?? {};

      try {
        if (eventName === 'queue_status') {
          const queuedAt = typeof payload?.queuedAt === 'number' && Number.isFinite(payload.queuedAt) ? payload.queuedAt : undefined;
          onEvent({ type: 'queue_status', status: payload?.status || 'queueing', queuedAt });
          return;
        }
        if (eventName === 'match_found') {
          const matchId = typeof payload?.matchId === 'string' ? payload.matchId : '';
          if (!matchId) return;
          found = true;
          onEvent({ type: 'match_found', matchId });
          return;
        }
        if (eventName === 'queue_reconnect') {
          reconnect = true;
          return;
        }
        if (eventName === 'queue_error') {
          onEvent({ type: 'queue_error', message: payload?.message || 'Queue failed' });
        }
      } catch (error: any) {
        settleReject(error instanceof Error ? error : new Error(error?.message || 'Queue failed'));
      }
    };
  });
}

export type MatchViewStatus = 'starting' | 'live' | 'ended' | 'interrupted' | 'missing' | 'forbidden';

export type MatchViewSeat = { userId: string; displayName: string; avatarUrl?: string; teamId?: string };

// MatchView is everything the client needs about a match: what it is, whether it is being played
// and whether the viewer plays in it, where to go after it, and how it ended.
export type MatchView = {
  matchId: string;
  kind?: MatchKind;
  mode?: string;
  status: MatchViewStatus;
  config?: MatchConfig;
  players: MatchViewSeat[];
  playing: boolean;
  signInRequired: boolean;
  currentMatchId?: string;
  party?: { id: string; inviteCode?: string };
  returnTarget?: MatchReturnTarget;
  result?: Snapshot;
};

const VIEW_STATUSES: MatchViewStatus[] = ['starting', 'live', 'ended', 'interrupted', 'missing', 'forbidden'];

export function isOpenMatch(view: MatchView) {
  return view.status === 'starting' || view.status === 'live';
}

export function normalizeMatchReturnTarget(value: any): MatchReturnTarget | undefined {
  if (!value || typeof value !== 'object') return undefined;
  if (value.kind === 'home') return { kind: 'home' };
  if (value.kind === 'map' && typeof value.mapId === 'string' && value.mapId.trim()) {
    return { kind: 'map', mapId: value.mapId };
  }
  if (value.kind === 'party' && typeof value.partyId === 'string' && value.partyId.trim()) {
    return {
      kind: 'party',
      partyId: value.partyId,
      ...(typeof value.partyInviteCode === 'string' && value.partyInviteCode ? { partyInviteCode: value.partyInviteCode } : {})
    };
  }
  return undefined;
}

export function normalizeMatchView(data: any, fallbackMatchId: string): MatchView {
  const status = VIEW_STATUSES.includes(data?.status) ? (data.status as MatchViewStatus) : 'missing';
  const party =
    data?.party && typeof data.party === 'object' && typeof data.party.id === 'string' && data.party.id
      ? { id: data.party.id, ...(typeof data.party.inviteCode === 'string' && data.party.inviteCode ? { inviteCode: data.party.inviteCode } : {}) }
      : undefined;
  const returnTarget = normalizeMatchReturnTarget(data?.returnTarget);
  return {
    matchId: typeof data?.matchId === 'string' && data.matchId ? data.matchId : fallbackMatchId,
    kind: typeof data?.kind === 'string' && data.kind ? (data.kind as MatchKind) : undefined,
    mode: typeof data?.mode === 'string' ? data.mode : undefined,
    status,
    config: typeof data?.config === 'object' && data.config ? (data.config as MatchConfig) : undefined,
    players: Array.isArray(data?.players) ? (data.players as MatchViewSeat[]) : [],
    playing: !!data?.playing,
    signInRequired: !!data?.signInRequired,
    ...(typeof data?.currentMatchId === 'string' && data.currentMatchId ? { currentMatchId: data.currentMatchId } : {}),
    ...(party ? { party } : {}),
    ...(returnTarget ? { returnTarget } : {}),
    ...(data?.result && typeof data.result === 'object' ? { result: data.result as Snapshot } : {})
  };
}

// fetchMatchView reads what the viewer (anonymous without a token) sees of a match.
export async function fetchMatchView(
  config: RuntimeConfig,
  matchId: string,
  signal: AbortSignal,
  accessToken?: string
): Promise<MatchView> {
  const resp = await apiFetch(config, `/api/v2/matches/${encodeURIComponent(matchId)}`, {
    headers: authHeaders(accessToken),
    signal
  });
  if (!resp.ok) {
    if (resp.status === 404) return normalizeMatchView({ status: 'missing' }, matchId);
    throw new Error('Match temporarily unavailable');
  }
  return normalizeMatchView(await resp.json(), matchId);
}

export type MatchBootstrapResponse = {
  auth: {
    accessToken?: string;
    nicknameRequired?: boolean;
    suggestedNickname?: string;
    user?: {
      id?: string;
      isGuest?: boolean;
    };
  };
  match: MatchView;
};

// bootstrapMatchView restores the session from the refresh cookie and reads the match as that player.
export async function bootstrapMatchView(
  config: RuntimeConfig,
  matchId: string,
  signal: AbortSignal
): Promise<MatchBootstrapResponse | null> {
  const resp = await apiFetch(config, `/api/v2/matches/${encodeURIComponent(matchId)}/bootstrap`, {
    credentials: 'include',
    signal
  });
  if (!resp.ok) {
    return null;
  }
  const data = await resp.json();
  return {
    auth: {
      accessToken: typeof data?.auth?.accessToken === 'string' ? data.auth.accessToken : '',
      nicknameRequired: !!data?.auth?.nicknameRequired,
      suggestedNickname: typeof data?.auth?.suggestedNickname === 'string' ? data.auth.suggestedNickname : '',
      user:
        data?.auth?.user && typeof data.auth.user === 'object'
          ? {
              id: typeof data.auth.user.id === 'string' ? data.auth.user.id : '',
              isGuest: typeof data.auth.user.isGuest === 'boolean' ? data.auth.user.isGuest : false
            }
          : undefined
    },
    match: normalizeMatchView(data?.match, matchId)
  };
}

// startSoloMatch starts a match the player plays alone and returns its view once a node runs it.
export async function startSoloMatch(
  config: RuntimeConfig,
  accessToken: string,
  signal: AbortSignal,
  matchConfig?: MatchConfig,
  returnTarget?: MatchReturnTarget,
): Promise<MatchView> {
  const resp = await apiFetch(config, '/api/v2/matches', {
    method: 'POST',
    headers: mergeHeaders(authHeaders(accessToken), { 'Content-Type': 'application/json' }),
    body: JSON.stringify({ kind: 'solo', config: matchConfig || {}, ...(returnTarget ? { returnTarget } : {}) }),
    signal,
  });
  if (!resp.ok) {
    let message = 'Singleplayer unavailable';
    try {
      const data = await resp.json();
      if (typeof data?.message === 'string' && data.message) message = data.message;
      else if (typeof data?.error === 'string' && data.error) message = data.error;
    } catch {
      // Keep the generic message.
    }
    throw new Error(message);
  }
  return normalizeMatchView(await resp.json(), '');
}

// matchSocketURL is where a seated player connects to play a match.
export function matchSocketURL(config: RuntimeConfig, matchId: string, accessToken: string) {
  return `${apiSocketURL(config, `/api/v2/matches/${encodeURIComponent(matchId)}/ws`)}?accessToken=${encodeURIComponent(accessToken)}`;
}
