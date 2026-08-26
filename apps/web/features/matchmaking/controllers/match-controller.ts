import type { Snapshot, TeamPing } from '../../game/model/types';
import type { SfxController } from '../../../lib/audio/sfx';
import type { RuntimeConfig } from '../../../lib/runtime-config';
import { ObservableStore } from '../../../lib/observable-store';
import { initialMatchmakingState, matchmakingReducer, type MatchmakingAction, type MatchmakingState } from '../../../lib/matchmaking';
import type { AuthSessionSnapshot } from '../../auth/session';
import type { SessionController } from '../../auth/controllers/session-controller';
import { GameplaySocketClient } from '../lib/gameplay-socket-client';
import { fetchMatchSession, startSingleplayerSession, streamQueue, type MatchConfig, type MatchReturnTarget, type QueueVariant } from '../lib/queue-client';

type SendGameCommandOptions = {
  errorMessage?: string;
  forceReconnect?: boolean;
  silent?: boolean;
};

export type MatchState = {
  matchmaking: MatchmakingState;
  connected: boolean;
  snapshot: Snapshot | null;
  lastFinalizedMatchId?: string;
  activeMatchId: string;
  sourcePartyId: string;
  sourcePartyInviteCode: string;
  returnTarget?: MatchReturnTarget;
  queueError: string;
  singleplayerError: string;
  connectionIssue: string;
  onlinePlayers: number;
  teamPings: TeamPing[];
};

const initialState: MatchState = {
  matchmaking: initialMatchmakingState,
  connected: false,
  snapshot: null,
  lastFinalizedMatchId: '',
  activeMatchId: '',
  sourcePartyId: '',
  sourcePartyInviteCode: '',
  returnTarget: { kind: 'home' },
  queueError: '',
  singleplayerError: '',
  connectionIssue: '',
  onlinePlayers: 0,
  teamPings: []
};

export class MatchController extends ObservableStore<MatchState> {
  private readonly config: RuntimeConfig;
  private state: MatchState = initialState;
  private readonly sessionController: SessionController;
  private readonly sfxController?: SfxController;
  private readonly socketClient: GameplaySocketClient;
  private autoRecoverEnabled = true;
  private activeSession: AuthSessionSnapshot | null = null;
  private recoverInFlight = false;
  private recoverRequestId = 0;
  private recoverAbort: AbortController | null = null;
  private queueAbort: AbortController | null = null;
  private singleplayerStartInFlight = false;
  private lastSocketOpenedAt = 0;
  private lastServerSeenAt = 0;
  private heartbeatInterval: ReturnType<typeof setInterval> | null = null;
  private firstSnapshotTimeout: ReturnType<typeof setTimeout> | null = null;
  private awaitingFirstSnapshotMatchId = '';
  private firstSnapshotRecoverAttempts = 0;
  private destroyed = false;
  private started = false;

  constructor(params: { config: RuntimeConfig; sessionController: SessionController; sfxController?: SfxController }) {
    super();
    this.config = params.config;
    this.sessionController = params.sessionController;
    this.sfxController = params.sfxController;
    this.socketClient = new GameplaySocketClient(this.config, {
      onOpen: () => {
        this.lastSocketOpenedAt = Date.now();
        this.patchState({ connected: false, connectionIssue: '' });
        this.dispatchMatchmaking({ type: 'game_connected' });
        this.sendGameCommand('ping', {}, { forceReconnect: true, silent: true });
      },
      onClose: ({ expected }) => {
        this.patchState({ connected: false });
        if (expected) {
          return;
        }
        if (this.state.snapshot || this.state.activeMatchId) {
          this.patchState({ connectionIssue: this.config.gameConnectionErrorMessage });
          this.dispatchMatchmaking({ type: 'ws_closed' });
          const currentSession = this.activeSession;
          if (currentSession) {
            void this.startRecover(currentSession);
          }
        }
      },
      onError: () => {
        this.markConnectionUnhealthy(this.config.gameConnectionErrorMessage);
      },
      onActivity: () => {
        this.noteServerActivity();
      },
      onSnapshot: (snapshot) => {
        if (this.state.snapshot?.matchId === snapshot.matchId && snapshot.eventSequence < this.state.snapshot.eventSequence) {
          return;
        }
        if (snapshot.matchId && snapshot.matchId === this.awaitingFirstSnapshotMatchId) {
          this.clearFirstSnapshotTimeout();
          this.awaitingFirstSnapshotMatchId = '';
          this.firstSnapshotRecoverAttempts = 0;
        }
        this.noteServerActivity();
        if (snapshot.state === 'ended') {
          const selfUserId =
            this.activeSession?.userId ||
            this.sessionController.getState().userId;
          const selfPlayer = snapshot.players?.[selfUserId];
          if (selfPlayer && !selfPlayer.isGuest) {
            this.sessionController.applyCommittedRating(
              selfPlayer.mmr,
              selfPlayer.ratingRd,
            );
          }
        }
        this.patchState({
          activeMatchId: snapshot.matchId || this.state.activeMatchId,
          snapshot,
          teamPings: this.state.snapshot?.currentRound?.roundId === snapshot.currentRound?.roundId ? this.state.teamPings : [],
          lastFinalizedMatchId:
            snapshot.state === 'ended'
              ? snapshot.matchId
              : this.state.lastFinalizedMatchId,
        });
      },
      onTeamPing: (ping) => {
        if (!ping?.id || ping.roundId !== this.state.snapshot?.currentRound?.roundId) return;
        this.patchState({ teamPings: [...this.state.teamPings.filter((item) => item.id !== ping.id), ping] });
        setTimeout(() => {
          this.patchState({ teamPings: this.state.teamPings.filter((item) => item.id !== ping.id) });
        }, Math.max(0, ping.expiresAt - Date.now()));
      },
      onAckError: (message) => {
        this.patchState({ queueError: message });
      },
      onProtocolError: () => {
        this.markConnectionUnhealthy(this.config.gameConnectionErrorMessage, true);
      }
    });
  }

  start() {
    if (this.started || typeof window === 'undefined') return;
    this.destroyed = false;
    this.started = true;
    this.startHeartbeat();
  }

  destroy() {
    this.destroyed = true;
    this.started = false;
    this.recoverAbort?.abort();
    this.clearRecoverTracking();
    this.queueAbort?.abort();
    this.clearFirstSnapshotTimeout();
    this.socketClient.close();
    if (this.heartbeatInterval) clearInterval(this.heartbeatInterval);
    this.heartbeatInterval = null;
  }

  getState() {
    return this.state;
  }

  setAutoRecoverEnabled = (enabled: boolean) => {
    this.autoRecoverEnabled = enabled;
    if (!enabled) {
      this.recoverAbort?.abort();
      this.clearRecoverTracking();
    }
  };

  private patchState(patch: Partial<MatchState>) {
    this.state = { ...this.state, ...patch };
    if (!this.destroyed) {
      this.emit();
    }
  }

  private clearRecoverTracking() {
    this.recoverInFlight = false;
    this.recoverAbort = null;
    if (this.state.matchmaking.activeRecoverRequestID === null) return;
    this.state = {
      ...this.state,
      matchmaking: {
        ...this.state.matchmaking,
        activeRecoverRequestID: null
      }
    };
    if (!this.destroyed) {
      this.emit();
    }
  }

  private dispatchMatchmaking(action: MatchmakingAction) {
    this.patchState({ matchmaking: matchmakingReducer(this.state.matchmaking, action) });
  }

  private startHeartbeat() {
    this.heartbeatInterval = setInterval(() => {
      if (!this.socketClient.isOpen()) return;
      const now = Date.now();
      const lastSeen = Math.max(this.lastSocketOpenedAt, this.lastServerSeenAt);
      if (lastSeen > 0 && now - lastSeen > this.config.socketStaleAfterMs) {
        this.markConnectionUnhealthy(this.config.connectionErrorMessage, true);
        return;
      }
      this.sendGameCommand('ping', { userId: this.activeSession?.userId || '' }, { forceReconnect: true, silent: true });
    }, this.config.socketHeartbeatIntervalMs);
  }

  private noteServerActivity() {
    this.lastServerSeenAt = Date.now();
    const nextQueueError = this.state.queueError === this.config.connectionErrorMessage ? '' : this.state.queueError;
    if (this.state.connected && !this.state.connectionIssue && this.state.queueError === nextQueueError) {
      return;
    }
    this.patchState({
      connected: true,
      connectionIssue: '',
      queueError: nextQueueError
    });
  }

  private markConnectionUnhealthy(message = this.config.connectionErrorMessage, forceReconnect = false) {
    const next: Partial<MatchState> = {
      connected: false,
      connectionIssue: message
    };
    if (this.state.matchmaking.status === 'queueing') {
      next.queueError = message;
    }
    this.patchState(next);
    if (!forceReconnect) return;
    if (!this.socketClient.isOpenOrConnecting()) return;
    this.socketClient.close();
  }

  resetConnectionState = () => {
    this.recoverAbort?.abort();
    this.clearRecoverTracking();
    this.queueAbort?.abort();
    this.clearFirstSnapshotTimeout();
    this.socketClient.close();
    this.activeSession = null;
    this.lastSocketOpenedAt = 0;
    this.lastServerSeenAt = 0;
    this.patchState({
      ...this.state,
      connected: false,
      snapshot: null,
      lastFinalizedMatchId: '',
      activeMatchId: '',
      sourcePartyId: '',
      sourcePartyInviteCode: '',
      returnTarget: { kind: 'home' },
      queueError: '',
      singleplayerError: '',
      connectionIssue: '',
    });
  };

  setConnectionIssue = (value: string) => {
    this.patchState({ connectionIssue: value });
  };

  private async startRecover(session: AuthSessionSnapshot) {
    if (!session.userId || !session.accessToken || session.nicknameRequired) return;
    if (this.recoverInFlight || this.state.matchmaking.activeRecoverRequestID !== null) return;

    const requestID = ++this.recoverRequestId;
    const intentVersionAtStart = this.state.matchmaking.intentVersion;
    const controller = new AbortController();

    this.recoverInFlight = true;
    this.recoverAbort = controller;
    this.dispatchMatchmaking({ type: 'recover_started', requestID });

    try {
      const ensuredSession = await this.sessionController.ensureFreshSession();
      if (!ensuredSession) {
        this.dispatchMatchmaking({ type: 'recover_failed', requestID });
        this.sessionController.clearAuthSession('Session expired. Please sign in again.');
        return;
      }
      const targetMatchID = this.state.activeMatchId || this.state.snapshot?.matchId || '';
      if (!targetMatchID) {
        this.dispatchMatchmaking({ type: 'recover_failed', requestID });
        return;
      }
      const resolved = await fetchMatchSession(this.config, ensuredSession.accessToken, targetMatchID, controller.signal);
      if (resolved.status === 'live_connectable') {
        const recoveredSession = this.sessionController.getSessionSnapshot();
        if (!recoveredSession) {
          this.dispatchMatchmaking({ type: 'recover_failed', requestID });
          return;
        }
        this.dispatchMatchmaking({
          type: 'recover_resolved',
          requestID,
          intentVersionAtStart,
          outcome: 'matched',
          hasSnapshot: !!this.state.snapshot
        });
        this.connectToAssignedGame(recoveredSession, resolved.node, resolved.wsPath, resolved.ticket, resolved.matchId, {
          sourcePartyId: resolved.sourcePartyId,
          sourcePartyInviteCode: resolved.sourcePartyInviteCode,
          returnTarget: resolved.returnTarget
        });
        return;
      }
      if (resolved.status === 'history') {
        this.patchState({
          snapshot: resolved.snapshot,
          activeMatchId: resolved.matchId,
          connectionIssue: '',
          returnTarget: resolved.returnTarget || { kind: 'home' }
        });
        this.dispatchMatchmaking({
          type: 'recover_resolved',
          requestID,
          intentVersionAtStart,
          outcome: 'ready',
          hasSnapshot: true
        });
        return;
      }
      if (resolved.status === 'replaced') {
        this.patchState({
          snapshot: null,
          connectionIssue: 'This match was replaced by another session.'
        });
      } else {
        this.patchState({
          snapshot: null,
          connectionIssue: 'Match unavailable.'
        });
      }
      this.dispatchMatchmaking({
        type: 'recover_resolved',
        requestID,
        intentVersionAtStart,
        outcome: 'abandoned',
        hasSnapshot: false
      });
      return;
    } catch (error: any) {
      this.dispatchMatchmaking({ type: 'recover_failed', requestID });
    } finally {
      if (this.recoverAbort === controller) {
        this.recoverInFlight = false;
        this.recoverAbort = null;
      }
    }
  }

  private connectToAssignedGame(
    session: AuthSessionSnapshot,
    node: string,
    wsPath: string,
    ticket: string,
    matchId?: string,
    source?: { sourcePartyId?: string; sourcePartyInviteCode?: string; returnTarget?: MatchReturnTarget }
  ) {
    if (!session.userId || !session.accessToken) return;
    this.activeSession = session;
    this.patchState({
      activeMatchId: matchId || this.state.activeMatchId,
      lastFinalizedMatchId: '',
      sourcePartyId: source?.sourcePartyId || '',
      sourcePartyInviteCode: source?.sourcePartyInviteCode || '',
      returnTarget: source?.returnTarget || this.state.returnTarget || { kind: 'home' }
    });
    this.socketClient.connect(session, node, wsPath, ticket);
    this.startFirstSnapshotTimeout(matchId || this.state.activeMatchId);
  }

  private clearFirstSnapshotTimeout() {
    if (this.firstSnapshotTimeout) clearTimeout(this.firstSnapshotTimeout);
    this.firstSnapshotTimeout = null;
  }

  private startFirstSnapshotTimeout(matchId?: string) {
    const targetMatchId = matchId || '';
    if (!targetMatchId) return;
    this.clearFirstSnapshotTimeout();
    this.awaitingFirstSnapshotMatchId = targetMatchId;
    this.firstSnapshotTimeout = setTimeout(() => {
      if (this.destroyed || this.state.snapshot?.matchId === targetMatchId) return;
      const session = this.activeSession || this.sessionController.getSessionSnapshot();
      this.socketClient.close();
      this.patchState({
        connected: false,
        connectionIssue: 'Still trying to join the live match...'
      });
      if (!session || this.firstSnapshotRecoverAttempts >= 3) {
        this.dispatchMatchmaking({ type: 'set_status', status: 'abandoned', bumpIntent: false });
        this.patchState({ connectionIssue: 'Could not join the live match. Please retry.' });
        return;
      }
      this.firstSnapshotRecoverAttempts += 1;
      this.dispatchMatchmaking({ type: 'ws_closed' });
      void this.startRecover(session);
    }, 10000);
  }

  resumeResolvedMatch = async (
    assignment: { matchId: string; node: string; wsPath: string; ticket: string; sourcePartyId?: string; sourcePartyInviteCode?: string; returnTarget?: MatchReturnTarget },
    options?: { playMatchFoundSfx?: boolean }
  ) => {
    const session = this.sessionController.getSessionSnapshot() || (await this.sessionController.ensureFreshSession());
    if (!session || !assignment.node || !assignment.ticket) {
      return false;
    }
    this.patchState({ queueError: '', connectionIssue: '' });
    this.dispatchMatchmaking({ type: 'set_status', status: 'matched_connecting' });
    if (options?.playMatchFoundSfx) {
      this.playMatchFoundSfx();
    }
    this.connectToAssignedGame(session, assignment.node, assignment.wsPath, assignment.ticket, assignment.matchId, {
      sourcePartyId: assignment.sourcePartyId,
      sourcePartyInviteCode: assignment.sourcePartyInviteCode,
      returnTarget: assignment.returnTarget
    });
    return true;
  };

  joinQueue = (queues: QueueVariant[] = ['moving']) => {
    this.recoverAbort?.abort();
    this.queueAbort?.abort();
    this.patchState({ queueError: '' });
    const controller = new AbortController();
    this.queueAbort = controller;

    void (async () => {
      try {
        const session = await this.sessionController.getPlayableSession();
        if (!session) {
          this.patchState({ queueError: 'Unable to create session' });
          this.dispatchMatchmaking({ type: 'queue_error' });
          return;
        }
        this.dispatchMatchmaking({ type: 'join_requested', startedAt: Date.now() });
        await streamQueue(this.config, session, controller.signal, queues, (event) => {
          if (event.type === 'queue_status') {
            this.dispatchMatchmaking({ type: 'queue_status', status: event.status, queuedAt: event.queuedAt });
            return;
          }
          if (event.type === 'match_assigned') {
            this.dispatchMatchmaking({ type: 'match_found' });
            this.queueAbort = null;
            this.patchState({ queueError: '' });
            this.playMatchFoundSfx();
            if (event.node && event.ticket) {
              this.connectToAssignedGame(session, event.node, event.wsPath, event.ticket, event.matchId, {
                sourcePartyId: event.sourcePartyId,
                sourcePartyInviteCode: event.sourcePartyInviteCode,
                returnTarget: event.returnTarget
              });
            }
            return;
          }
          throw new Error(event.message);
        });
      } catch (error: any) {
        if (error?.name === 'AbortError') {
          return;
        }
        this.patchState({ queueError: error?.message || 'Queue failed' });
        this.dispatchMatchmaking({ type: 'queue_error' });
      } finally {
        if (this.queueAbort === controller) {
          this.queueAbort = null;
        }
      }
    })();
  };

  startSingleplayer = async (matchConfig?: MatchConfig, returnTarget?: MatchReturnTarget) => {
    if (this.singleplayerStartInFlight) {
      return '';
    }
    this.singleplayerStartInFlight = true;
    this.recoverAbort?.abort();
    this.queueAbort?.abort();
    this.patchState({ singleplayerError: '', connectionIssue: '' });
    this.dispatchMatchmaking({ type: 'set_status', status: 'matched_connecting' });
    const controller = new AbortController();
    this.queueAbort = controller;

    try {
      const session = await this.sessionController.getPlayableSession();
      if (!session) {
        this.patchState({ singleplayerError: 'Unable to create session' });
        this.dispatchMatchmaking({ type: 'set_status', status: 'ready' });
        return '';
      }
      const assignment = await startSingleplayerSession(
        this.config,
        session.accessToken,
        controller.signal,
        matchConfig,
        returnTarget || { kind: 'home' },
      );
      if (!assignment.node || !assignment.ticket) {
        throw new Error('Singleplayer unavailable');
      }
      if (this.queueAbort !== controller) {
        return '';
      }
      this.connectToAssignedGame(session, assignment.node, assignment.wsPath, assignment.ticket, assignment.matchId, { returnTarget: assignment.returnTarget });
      return assignment.matchId;
    } catch (error: any) {
      if (error?.name === 'AbortError') {
        return '';
      }
      this.patchState({ singleplayerError: error?.message || 'Singleplayer unavailable' });
      this.dispatchMatchmaking({ type: 'set_status', status: 'ready' });
      return '';
    } finally {
      this.singleplayerStartInFlight = false;
      if (this.queueAbort === controller) {
        this.queueAbort = null;
      }
    }
  };

  clearSingleplayerError = () => {
    if (!this.state.singleplayerError) return;
    this.patchState({ singleplayerError: '' });
  };

  cancelQueue = () => {
    if (!this.sessionController.getSessionSnapshot()) return;
    this.recoverAbort?.abort();
    this.queueAbort?.abort();
    this.dispatchMatchmaking({ type: 'leave_requested' });
    this.patchState({ queueError: '' });
  };

  setStatus = (status: MatchmakingState['status']) => {
    this.dispatchMatchmaking({ type: 'set_status', status });
  };

  private playMatchFoundSfx() {
    this.sfxController?.play('duel-game-start');
  }

  sendGameCommand = (type: string, payload: Record<string, unknown>, options?: SendGameCommandOptions) => {
    if (!this.socketClient.isOpen()) {
      if (!options?.silent) {
        this.markConnectionUnhealthy(options?.errorMessage ?? this.config.connectionErrorMessage, !!options?.forceReconnect);
      }
      return false;
    }
    const cmd = {
      commandId: `${this.activeSession?.userId || this.sessionController.getSessionSnapshot()?.userId || 'anon'}-${Date.now()}-${Math.floor(Math.random() * 1000)}`,
      type,
      payload,
      sentAt: Date.now()
    };
    try {
      return this.socketClient.send(cmd);
    } catch {
      if (!options?.silent) {
        this.markConnectionUnhealthy(options?.errorMessage ?? this.config.connectionErrorMessage, !!options?.forceReconnect);
      }
      return false;
    }
  };
}
