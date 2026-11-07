
import { apiFetch, apiSocketURL, authHeaders, mergeHeaders, readError } from '../../../lib/http';
import type { MatchKind, Snapshot } from '../../game/model/types';

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

export type MaintenancePhase = 'normal' | 'warning' | 'active';

export type MaintenanceStatus = {
  phase: MaintenancePhase;
  startsAt?: string;
  endsAt?: string;
  queuePaused: boolean;
  playPaused: boolean;
  message: string;
};

export async function enterQueue(token: string, queues: QueueVariant[]) {
  const response = await apiFetch('/api/v2/queue', {
    method: 'POST',
    headers: mergeHeaders(authHeaders(token), { 'Content-Type': 'application/json' }),
    body: JSON.stringify({ queues: queues.join(',') }),
  });
  if (!response.ok) throw new Error(await readError(response, 'Queue unavailable'));
}

export async function leaveQueue(token: string) {
  await apiFetch('/api/v2/queue', { method: 'DELETE', headers: authHeaders(token) });
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
  matchId: string,
  signal: AbortSignal,
  accessToken?: string
): Promise<MatchView> {
  const resp = await apiFetch(`/api/v2/matches/${encodeURIComponent(matchId)}`, {
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
  matchId: string,
  signal: AbortSignal
): Promise<MatchBootstrapResponse | null> {
  const resp = await apiFetch(`/api/v2/matches/${encodeURIComponent(matchId)}/bootstrap`, {
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
  accessToken: string,
  signal: AbortSignal,
  matchConfig?: MatchConfig,
  returnTarget?: MatchReturnTarget,
): Promise<MatchView> {
  const resp = await apiFetch('/api/v2/matches', {
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

// matchSocketURL is where a seated player connects to play a match. The session cookie says who they are.
export function matchSocketURL(matchId: string) {
  return apiSocketURL(`/api/live/matches/${encodeURIComponent(matchId)}`);
}
