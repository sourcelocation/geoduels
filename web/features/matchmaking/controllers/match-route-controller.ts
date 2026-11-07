import type { Snapshot } from '../../game/model/types';
import { Listeners } from "../../../lib/listeners";
import type { AuthSessionSnapshot } from '../../auth/session';
import type { SessionController } from '../../auth/controllers/session-controller';
import type { MatchController } from './match-controller';
import {
  bootstrapMatchView,
  fetchMatchView,
  isOpenMatch,
  type MatchView
} from '../lib/queue-client';

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
  // view is the target match as the viewer last read it.
  view: MatchView | null;
};

const initialState: MatchRouteState = {
  targetMatchId: '',
  status: 'idle',
  historySnapshot: null,
  view: null
};

export class MatchRouteController {
  private readonly listeners = new Listeners();
  readonly subscribe = this.listeners.subscribe;
  private readonly sessionController: SessionController;
  private readonly matchController: MatchController;
  private state: MatchRouteState = initialState;
  private requestController: AbortController | null = null;
  private resolveSeq = 0;
  private started = false;
  private destroyed = false;
  private unsubscribeMatch: (() => void) | null = null;

  constructor(params: { sessionController: SessionController; matchController: MatchController }) {
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
      this.patchState({ targetMatchId: '', status: 'missing', historySnapshot: null, view: null });
      return;
    }
    if (this.state.targetMatchId === nextMatchId && this.state.status !== 'missing') {
      return;
    }
    this.clearPendingWork();
    this.patchState({ targetMatchId: nextMatchId, status: 'idle', historySnapshot: null, view: null });
    void this.resolve(nextMatchId);
  };

  // acceptStartedMatch goes to a match the server just seated this player in, the same way opening
  // its page would.
  acceptStartedMatch = (matchId: string) => {
    if (!this.matchController.matchStarted(matchId)) return;
    this.patchState({ targetMatchId: matchId, status: 'awaiting_first_snapshot', historySnapshot: null, view: null });
    void this.resolve(matchId);
  };

  reset = () => {
    this.clearPendingWork();
    this.patchState(initialState);
  };

  private patchState(patch: Partial<MatchRouteState>) {
    this.state = { ...this.state, ...patch };
    if (!this.destroyed) {
      this.listeners.emit();
    }
  }

  private clearPendingWork() {
    this.resolveSeq += 1;
    this.requestController?.abort();
    this.requestController = null;
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
    const sessionSnapshot: AuthSessionSnapshot = {
      userId: typeof auth.user?.id === 'string' ? auth.user.id : '',
      accessToken: auth.accessToken || '',
      nicknameRequired: !!auth.nicknameRequired,
      nicknameInput: auth.suggestedNickname || ''
    };
    this.sessionController.applySessionSnapshot(sessionSnapshot, {
      isGuest: typeof auth.user?.isGuest === 'boolean' ? auth.user.isGuest : false,
      authLoading: false,
      authError: ''
    });
  }

  private async handleView(matchId: string, view: MatchView, seq: number) {
    if (seq !== this.resolveSeq || this.state.targetMatchId !== matchId) return;
    if (isOpenMatch(view) && view.playing) {
      const ok = await this.matchController.resumeMatch(view.matchId, {
        sourcePartyId: view.party?.id,
        sourcePartyInviteCode: view.party?.inviteCode,
        returnTarget: view.returnTarget
      });
      if (seq !== this.resolveSeq || this.state.targetMatchId !== matchId) return;
      this.patchState({ status: ok ? 'awaiting_first_snapshot' : 'missing', view });
      return;
    }
    if (view.status === 'ended' && view.result) {
      this.patchState({ status: 'history', historySnapshot: view.result, view });
      return;
    }
    if (view.status === 'forbidden' || isOpenMatch(view)) {
      this.patchState({ status: 'forbidden', historySnapshot: null, view });
      return;
    }
    this.patchState({ status: view.currentMatchId ? 'replaced' : 'missing', historySnapshot: null, view });
  }

  private async resolve(matchId: string) {
    this.clearPendingWork();
    const seq = ++this.resolveSeq;
    const requestController = new AbortController();
    this.requestController = requestController;
    this.patchState({ historySnapshot: null, view: null });

    try {
      const existingSession = this.sessionController.getSessionSnapshot();
      this.patchState({ status: 'resolving' });
      const publicView = await fetchMatchView(matchId, requestController.signal, existingSession?.accessToken);
      if (!publicView.signInRequired) {
        await this.handleView(matchId, publicView, seq);
        return;
      }

      if (!this.sessionController.getSessionSnapshot()) {
        this.patchState({ status: 'bootstrapping_auth' });
        const bootstrapped = await bootstrapMatchView(matchId, requestController.signal);
        if (!bootstrapped) {
          if (seq === this.resolveSeq && this.state.targetMatchId === matchId) {
            this.patchState({ status: 'forbidden' });
          }
          return;
        }
        this.applyBootstrappedAuth(bootstrapped.auth);
        await this.handleView(matchId, bootstrapped.match, seq);
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
      const view = await fetchMatchView(matchId, requestController.signal, session.accessToken);
      await this.handleView(matchId, view, seq);
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
