import type { RuntimeConfig } from '../../../lib/runtime-config';
import { normalizeHTTPBase, normalizeWSBase } from '../../../lib/runtime-config';
import { apiFetch, authHeaders, mergeHeaders } from '../../../lib/http';
import type { Snapshot } from '../../game/model/types';
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
  | { type: 'match_assigned'; matchId: string; mode?: string; config?: MatchConfig; node: string; ticket: string; wsPath: string; sourcePartyId?: string; sourcePartyInviteCode?: string; returnTarget?: MatchReturnTarget }
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

export async function heartbeatQueue(
  config: RuntimeConfig,
  session: AuthSessionSnapshot,
  signal: AbortSignal
): Promise<'queueing' | 'matched' | 'missing'> {
  const resp = await fetch(`${normalizeHTTPBase(config.queueURL).replace(/\/$/, '')}/queue/heartbeat`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${session.accessToken}` },
    signal
  });
  if (!resp.ok) {
    throw new Error('Queue unavailable');
  }
  const data = await resp.json();
  const status = typeof data?.status === 'string' ? data.status : '';
  if (status === 'queueing' || status === 'matched' || status === 'missing') {
    return status;
  }
  throw new Error('Queue unavailable');
}

export async function streamQueue(
  config: RuntimeConfig,
  session: AuthSessionSnapshot,
  signal: AbortSignal,
  queues: QueueVariant[],
  onEvent: (event: QueueEvent) => void
) {
  const base = normalizeWSBase(config.queueURL).replace(/\/$/, '');
  const selectedQueues = (queues.length ? queues : ['moving']).join(',');
  const target = `${base}/queue?accessToken=${encodeURIComponent(session.accessToken)}&queues=${encodeURIComponent(selectedQueues)}`;

  await new Promise<void>((resolve, reject) => {
    let settled = false;
    let assigned = false;
    const ws = new WebSocket(target);

    const cleanup = () => {
      signal.removeEventListener('abort', abort);
    };

    const settleResolve = () => {
      if (settled) return;
      settled = true;
      cleanup();
      resolve();
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
      settleReject(new Error('Queue unavailable'));
    };

    ws.onclose = () => {
      if (signal.aborted) {
        settleReject(new DOMException('Aborted', 'AbortError'));
        return;
      }
      if (assigned) {
        settleResolve();
        return;
      }
      settleReject(new Error('Search cancelled'));
    };

    ws.onmessage = (evt) => {
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
        if (eventName === 'match_assigned') {
          assigned = true;
          onEvent({
            type: 'match_assigned',
            matchId: typeof payload?.matchId === 'string' ? payload.matchId : '',
            mode: typeof payload?.mode === 'string' ? payload.mode : '',
            config: typeof payload?.config === 'object' && payload.config ? (payload.config as MatchConfig) : undefined,
            node: typeof payload?.node === 'string' ? payload.node : '',
            ticket: typeof payload?.ticket === 'string' ? payload.ticket : '',
            wsPath: typeof payload?.wsPath === 'string' ? payload.wsPath : '',
            sourcePartyId: typeof payload?.sourcePartyId === 'string' ? payload.sourcePartyId : '',
            sourcePartyInviteCode: typeof payload?.sourcePartyInviteCode === 'string' ? payload.sourcePartyInviteCode : '',
            returnTarget: normalizeMatchReturnTarget(payload?.returnTarget)
          });
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

export type ResumableSessionResponse = { status: 'none'; matchId?: string; mode?: string } | { status: 'match'; matchId: string; mode?: string };

export async function fetchResumableSession(
  config: RuntimeConfig,
  accessToken: string,
  signal?: AbortSignal
): Promise<ResumableSessionResponse> {
  const resp = await apiFetch(config, '/v1/session/resumable', {
    headers: authHeaders(accessToken),
    signal
  });
  if (!resp.ok) {
    return { status: 'none' };
  }
  const data = await resp.json();
  const status = typeof data?.status === 'string' ? data.status : '';
  if (status === 'match') {
    return {
      status,
      matchId: typeof data?.matchId === 'string' ? data.matchId : '',
      mode: typeof data?.mode === 'string' ? data.mode : ''
    };
  }
  return { status: 'none' };
}

export type MatchSessionResponse =
  | { status: 'live_connectable'; matchId: string; mode?: string; config?: MatchConfig; ticket: string; node: string; wsPath: string; sourcePartyId?: string; sourcePartyInviteCode?: string; returnTarget?: MatchReturnTarget }
  | { status: 'live_auth_required'; matchId: string }
  | { status: 'history'; matchId: string; snapshot: Snapshot; replacementMatchId?: string; sourcePartyId?: string; sourcePartyInviteCode?: string; returnTarget?: MatchReturnTarget }
  | {
      status: 'replaced';
      matchId: string;
      replacementMatchId: string;
      replacement?: { matchId: string; mode?: string; config?: MatchConfig; ticket: string; node: string; wsPath: string; sourcePartyId?: string; sourcePartyInviteCode?: string; returnTarget?: MatchReturnTarget };
      sourcePartyId?: string;
      sourcePartyInviteCode?: string;
      returnTarget?: MatchReturnTarget;
    }
  | { status: 'missing' | 'forbidden'; matchId: string };

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
  match: MatchSessionResponse;
};

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

function normalizeMatchSessionResponse(data: any, fallbackMatchId: string): MatchSessionResponse {
  const status = typeof data?.status === 'string' ? data.status : 'missing';
  const sourceParty =
    typeof data?.sourcePartyInviteCode === 'string' && data.sourcePartyInviteCode
      ? {
          sourcePartyId: typeof data?.sourcePartyId === 'string' ? data.sourcePartyId : '',
          sourcePartyInviteCode: data.sourcePartyInviteCode
        }
      : {};
  const returnTarget = normalizeMatchReturnTarget(data?.returnTarget);
  if (status === 'live_connectable') {
    return {
      status,
      matchId: typeof data?.matchId === 'string' ? data.matchId : fallbackMatchId,
      mode: typeof data?.mode === 'string' ? data.mode : '',
      config: typeof data?.config === 'object' && data.config ? (data.config as MatchConfig) : undefined,
      ticket: typeof data?.ticket === 'string' ? data.ticket : '',
      node: typeof data?.node === 'string' ? data.node : '',
      wsPath: typeof data?.wsPath === 'string' ? data.wsPath : '',
      ...sourceParty,
      ...(returnTarget ? { returnTarget } : {})
    };
  }
  if (status === 'history') {
    return {
      status,
      matchId: typeof data?.matchId === 'string' ? data.matchId : fallbackMatchId,
      snapshot: (data?.snapshot || null) as Snapshot,
      replacementMatchId: typeof data?.replacementMatchId === 'string' ? data.replacementMatchId : '',
      ...sourceParty,
      ...(returnTarget ? { returnTarget } : {})
    };
  }
  if (status === 'replaced') {
    const replacementPayload =
      data?.replacement && typeof data.replacement === 'object'
        ? {
            matchId: typeof data.replacement.matchId === 'string' ? data.replacement.matchId : '',
            mode: typeof data.replacement.mode === 'string' ? data.replacement.mode : '',
            config: typeof data.replacement.config === 'object' && data.replacement.config ? (data.replacement.config as MatchConfig) : undefined,
            ticket: typeof data.replacement.ticket === 'string' ? data.replacement.ticket : '',
            node: typeof data.replacement.node === 'string' ? data.replacement.node : '',
            wsPath: typeof data.replacement.wsPath === 'string' ? data.replacement.wsPath : '',
            ...(typeof data.replacement.sourcePartyInviteCode === 'string' && data.replacement.sourcePartyInviteCode
              ? {
                  sourcePartyId: typeof data.replacement.sourcePartyId === 'string' ? data.replacement.sourcePartyId : '',
                  sourcePartyInviteCode: data.replacement.sourcePartyInviteCode
                }
                : {}),
            ...(normalizeMatchReturnTarget(data.replacement.returnTarget)
              ? { returnTarget: normalizeMatchReturnTarget(data.replacement.returnTarget) }
              : {})
          }
        : undefined;
    return {
      status,
      matchId: typeof data?.matchId === 'string' ? data.matchId : fallbackMatchId,
      replacementMatchId: typeof data?.replacementMatchId === 'string' ? data.replacementMatchId : '',
      replacement: replacementPayload,
      ...sourceParty,
      ...(returnTarget ? { returnTarget } : {})
    };
  }
  if (status === 'forbidden') {
    return { status, matchId: typeof data?.matchId === 'string' ? data.matchId : fallbackMatchId };
  }
  if (status === 'live_auth_required') {
    return { status, matchId: typeof data?.matchId === 'string' ? data.matchId : fallbackMatchId };
  }
  return { status: 'missing', matchId: typeof data?.matchId === 'string' ? data.matchId : fallbackMatchId };
}

export async function resolveMatchRoute(
  config: RuntimeConfig,
  matchId: string,
  signal: AbortSignal,
  accessToken?: string
): Promise<MatchSessionResponse> {
  const resp = await apiFetch(config, `/v1/matches/${encodeURIComponent(matchId)}/route`, {
    headers: authHeaders(accessToken),
    signal
  });
  if (!resp.ok) {
    return { status: 'missing', matchId };
  }
  return normalizeMatchSessionResponse(await resp.json(), matchId);
}

export async function fetchMatchSession(
  config: RuntimeConfig,
  accessToken: string,
  matchId: string,
  signal: AbortSignal
): Promise<MatchSessionResponse> {
  const resp = await apiFetch(config, `/v1/matches/${encodeURIComponent(matchId)}/session`, {
    headers: authHeaders(accessToken),
    signal
  });
  if (!resp.ok) {
    return { status: 'missing', matchId };
  }
  return normalizeMatchSessionResponse(await resp.json(), matchId);
}

export async function bootstrapMatchSession(
  config: RuntimeConfig,
  matchId: string,
  signal: AbortSignal
): Promise<MatchBootstrapResponse | null> {
  const resp = await apiFetch(config, `/v1/matches/${encodeURIComponent(matchId)}/bootstrap`, {
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
    match: normalizeMatchSessionResponse(data?.match, matchId)
  };
}

export async function startSingleplayerSession(
  config: RuntimeConfig,
  accessToken: string,
  signal: AbortSignal,
  matchConfig?: MatchConfig,
  returnTarget?: MatchReturnTarget,
): Promise<{ matchId: string; mode?: string; ticket: string; node: string; wsPath: string; returnTarget?: MatchReturnTarget }> {
  const body = returnTarget ? JSON.stringify({ config: matchConfig || {}, returnTarget }) : matchConfig ? JSON.stringify(matchConfig) : undefined;
  const resp = await apiFetch(config, '/v1/singleplayer/session', {
    method: 'POST',
    headers: mergeHeaders(authHeaders(accessToken), body ? { 'Content-Type': 'application/json' } : undefined),
    body,
    signal,
  });
  if (!resp.ok) {
    throw new Error('Singleplayer unavailable');
  }
  const data = await resp.json();
  return {
    matchId: typeof data?.matchId === 'string' ? data.matchId : '',
    mode: typeof data?.mode === 'string' ? data.mode : '',
    ticket: typeof data?.ticket === 'string' ? data.ticket : '',
    node: typeof data?.node === 'string' ? data.node : '',
    wsPath: typeof data?.wsPath === 'string' ? data.wsPath : '',
    returnTarget: normalizeMatchReturnTarget(data?.returnTarget)
  };
}
