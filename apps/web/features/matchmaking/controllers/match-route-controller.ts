import type { Snapshot } from '../../../components/ui/types';
import { ObservableStore } from '../../../lib/observable-store';
import type { AuthSessionSnapshot } from '../../auth/session';
import type { SessionController } from '../../auth/controllers/session-controller';
import type { MatchController } from './match-controller';
import {
  bootstrapMatchSession,
  resolveMatchRoute,
  type MatchSessionResponse
} from '../lib/queue-client';
import type { RuntimeConfig } from '../../../lib/runtime-config';

export type MatchRouteStatus =
  | 'idle'
  | 'bootstrapping_auth'
  | 'resolving'
  | 'awaiting_first_snapshot'
  | 'history'
  | 'replaced'
  | 'missing'
  | 'forbidden';

export type MatchRouteState = {
  targetMatchId: string;
  status: MatchRouteStatus;
  historySnapshot: Snapshot | null;
  replacement: MatchSessionResponse | null;
};

const initialState: MatchRouteState = {
  targetMatchId: '',
  status: 'idle',
  historySnapshot: null,
  replacement: null
};

export class MatchRouteController extends ObservableStore<MatchRouteState> {
  private readonly config: RuntimeConfig;
  private readonly sessionController: SessionController;
  private readonly matchController: MatchController;
  private state: MatchRouteState = initialState;
  private requestController: AbortController | null = null;
  private resolveSeq = 0;
  private started = false;
  private destroyed = false;
  private unsubscribeMatch: (() => void) | null = null;

  constructor(params: { config: RuntimeConfig; sessionController: SessionController; matchController: MatchController }) {
    super();
    this.config = params.config;
    this.sessionController = params.sessionController;
    this.matchController = params.matchController;
  }

  start() {
    if (this.started) return;
    this.started = true;
    this.destroyed = false;
    this.unsubscribeMatch = this.matchController.subscribe(() => {
      if (this.state.status !== 'awaiting_first_snapshot') return;
      const matchState = this.matchController.getState();
      const snapshot = matchState.snapshot;
      if (snapshot?.matchId && snapshot.matchId === this.state.targetMatchId) {
        this.patchState({ status: 'idle' });
        return;
      }
      if (matchState.matchmaking.status === 'abandoned' && matchState.activeMatchId === this.state.targetMatchId) {
        this.patchState({ status: 'missing' });
      }
    });
  }

  destroy() {
    this.destroyed = true;
    this.started = false;
    this.clearPendingWork();
    this.unsubscribeMatch?.();
    this.unsubscribeMatch = null;
  }

  getState() {
    return this.state;
  }

  setTargetMatch = (matchId: string | null | undefined) => {
    const nextMatchId = typeof matchId === 'string' ? matchId.trim() : '';
    if (!nextMatchId) {
      this.clearPendingWork();
      this.patchState({
        targetMatchId: '',
        status: 'missing',
        historySnapshot: null,
        replacement: null
      });
      return;
    }
    if (this.state.targetMatchId === nextMatchId && this.state.status !== 'missing') {
      return;
    }
    this.clearPendingWork();
    this.patchState({
      targetMatchId: nextMatchId,
      status: 'idle',
      historySnapshot: null,
      replacement: null
    });
    void this.resolve(nextMatchId);
  };

  reset = () => {
    this.clearPendingWork();
    this.patchState(initialState);
  };

  private patchState(patch: Partial<MatchRouteState>) {
    this.state = { ...this.state, ...patch };
    if (!this.destroyed) {
      this.emit();
    }
  }

  private clearPendingWork() {
    this.requestController?.abort();
    this.requestController = null;
  }

  private buildSessionSnapshot(auth: {
    accessToken?: string;
    nicknameRequired?: boolean;
    suggestedNickname?: string;
    user?: { id?: string };
  }): AuthSessionSnapshot {
    return {
      userId: typeof auth.user?.id === 'string' ? auth.user.id : '',
      accessToken: auth.accessToken || '',
      nicknameRequired: !!auth.nicknameRequired,
      nicknameInput: auth.suggestedNickname || ''
    };
  }

  private applyBootstrappedAuth(auth: {
    accessToken?: string;
    nicknameRequired?: boolean;
    suggestedNickname?: string;
    user?: {
      id?: string;
      isGuest?: boolean;
    };
  }) {
    const sessionSnapshot = this.buildSessionSnapshot(auth);
    this.sessionController.applySessionSnapshot(sessionSnapshot, {
      isGuest: typeof auth.user?.isGuest === 'boolean' ? auth.user.isGuest : false,
      leaderboard: null,
      authLoading: false,
      authError: ''
    });
  }

  private async handleResolvedMatch(matchId: string, resolved: MatchSessionResponse, seq: number) {
    if (seq !== this.resolveSeq || this.state.targetMatchId !== matchId) return;
    switch (resolved.status) {
      case 'live_connectable': {
        const ok = await this.matchController.resumeResolvedMatch({
          matchId: resolved.matchId,
          node: resolved.node,
          wsPath: resolved.wsPath,
          ticket: resolved.ticket,
          ...(resolved.sourcePartyInviteCode
            ? {
                sourcePartyId: resolved.sourcePartyId,
                sourcePartyInviteCode: resolved.sourcePartyInviteCode
              }
            : {})
        });
        if (seq !== this.resolveSeq || this.state.targetMatchId !== matchId) return;
        this.patchState({ status: ok ? 'awaiting_first_snapshot' : 'missing' });
        return;
      }
      case 'history':
        this.patchState({ status: 'history', historySnapshot: resolved.snapshot, replacement: resolved });
        return;
      case 'replaced':
        this.patchState({ status: 'replaced', replacement: resolved, historySnapshot: null });
        return;
      case 'live_auth_required':
      case 'forbidden':
        this.patchState({ status: 'forbidden', historySnapshot: null, replacement: null });
        return;
      default:
        this.patchState({ status: 'missing', historySnapshot: null, replacement: null });
    }
  }

  private async resolve(matchId: string) {
    this.clearPendingWork();
    const seq = ++this.resolveSeq;
    const requestController = new AbortController();
    this.requestController = requestController;
    this.patchState({ historySnapshot: null, replacement: null });

    try {
      const existingSession = this.sessionController.getSessionSnapshot();
      this.patchState({ status: 'resolving' });
      const publicResolved = await resolveMatchRoute(this.config, matchId, requestController.signal, existingSession?.accessToken);
      if (publicResolved.status !== 'live_auth_required') {
        await this.handleResolvedMatch(matchId, publicResolved, seq);
        return;
      }

      if (!this.sessionController.getSessionSnapshot()) {
        this.patchState({ status: 'bootstrapping_auth' });
        const bootstrapped = await bootstrapMatchSession(this.config, matchId, requestController.signal);
        if (!bootstrapped) {
          if (seq === this.resolveSeq && this.state.targetMatchId === matchId) {
            this.patchState({ status: 'forbidden' });
          }
          return;
        }
        this.applyBootstrappedAuth(bootstrapped.auth);
        await this.handleResolvedMatch(matchId, bootstrapped.match, seq);
        return;
      }

      const session = await this.sessionController.ensureFreshSession(60_000, { allowNicknameRequired: false });
      if (!session) {
        if (seq === this.resolveSeq && this.state.targetMatchId === matchId) {
          this.patchState({ status: 'forbidden' });
        }
        return;
      }

      this.patchState({ status: 'resolving' });
      const resolved = await resolveMatchRoute(this.config, matchId, requestController.signal, session.accessToken);
      await this.handleResolvedMatch(matchId, resolved, seq);
    } catch (error: any) {
      if (error?.name === 'AbortError') return;
      if (seq === this.resolveSeq && this.state.targetMatchId === matchId) {
        this.patchState({ status: 'missing' });
      }
    } finally {
      if (this.requestController === requestController) {
        this.requestController = null;
      }
    }
  }
}
