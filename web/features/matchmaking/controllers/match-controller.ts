import type { Snapshot, TeamPing } from '../../game/model/types';
import type { SfxController } from '../../../lib/audio/sfx';
import { Listeners } from "../../../lib/listeners";
import { initialMatchmakingState, matchmakingReducer, type MatchmakingAction, type MatchmakingState } from '../../../lib/matchmaking';
import type { AuthSessionSnapshot } from '../../auth/session';
import type { SessionController } from '../../auth/controllers/session-controller';
import type { ActiveMatch, QueueState } from '../../live/events';
import { GameplaySocketClient } from '../lib/gameplay-socket-client';
import { fetchMatchView, isOpenMatch, startSoloMatch, enterQueue, leaveQueue, type MatchConfig, type MatchReturnTarget, type QueueVariant } from '../lib/queue-client';
import { CONNECTION_ERROR_MESSAGE, GAME_CONNECTION_ERROR_MESSAGE, SOCKET_HEARTBEAT_INTERVAL_MS, SOCKET_STALE_AFTER_MS } from "../../../lib/constants";

// Where the player goes after a match and which party it came from, as its view says.
export type MatchSource = { sourcePartyId?: string; sourcePartyInviteCode?: string; returnTarget?: MatchReturnTarget };

type SendGameCommandOptions = {
  silent?: boolean;
};

export type MatchState = {
  matchmaking: MatchmakingState;
  connected: boolean;
  snapshot: Snapshot | null;
  lastFinalizedMatchId?: string;
  activeMatchId: string;
  // A match the server told this player they're seated in, for the app to go to.
  launchMatchId: string;
  // The open match the player can go back to, as the server last said.
  resumeMatchId: string;
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
  launchMatchId: '',
  resumeMatchId: '',
  sourcePartyId: '',
  sourcePartyInviteCode: '',
  returnTarget: { kind: 'home' },
  queueError: '',
  singleplayerError: '',
  connectionIssue: '',
  onlinePlayers: 0,
  teamPings: []
};

export class MatchController {
  private readonly listeners = new Listeners();
  readonly subscribe = this.listeners.subscribe;
  private state: MatchState = initialState;
  private readonly sessionController: SessionController;
  private readonly sfxController?: SfxController;
  private readonly socketClient: GameplaySocketClient;
  private activeSession: AuthSessionSnapshot | null = null;
  private recoverAbort: AbortController | null = null;
  private soloAbort: AbortController | null = null;
  private singleplayerStartInFlight = false;
  private heartbeatInterval: ReturnType<typeof setInterval> | null = null;
  private responseTimeout: ReturnType<typeof setTimeout> | null = null;
  private reconnectTimeout: ReturnType<typeof setTimeout> | null = null;
  private reconnectAttempt = 0;
  private awaitingSnapshot = false;
  private connectionWanted = false;
  private connectionGeneration = 0;
  private destroyed = false;
  private started = false;

  constructor(params: { sessionController: SessionController; sfxController?: SfxController }) {
    this.sessionController = params.sessionController;
    this.sfxController = params.sfxController;
    this.socketClient = new GameplaySocketClient({
      onOpen: () => {
        this.sendGameCommand('ping', { userId: this.activeSession?.userId || '' });
      },
      onClose: () => this.beginRecovery(),
      onError: () => this.beginRecovery(),
      onActivity: () => this.noteServerActivity(),
      onSnapshot: (snapshot) => {
        if (this.state.snapshot?.matchId === snapshot.matchId && snapshot.eventSequence < this.state.snapshot.eventSequence) {
          return;
        }
        if (snapshot.matchId !== this.state.activeMatchId) return;
        this.awaitingSnapshot = false;
        this.reconnectAttempt = 0;
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
          resumeMatchId:
            snapshot.state === 'ended' && snapshot.matchId === this.state.resumeMatchId
              ? ''
              : this.state.resumeMatchId,
        });
        this.noteServerActivity();
        this.dispatchMatchmaking({ type: 'game_connected' });
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
        this.beginRecovery();
      }
    });
  }

  start() {
    if (this.started || typeof window === 'undefined') return;
    this.destroyed = false;
    this.started = true;
    this.heartbeatInterval = setInterval(() => this.probeConnection(), SOCKET_HEARTBEAT_INTERVAL_MS);
    document.addEventListener('visibilitychange', this.handleVisibilityChange);
    window.addEventListener('online', this.handleOnline);
  }

  destroy() {
    this.destroyed = true;
    this.started = false;
    this.stopConnection();
    this.soloAbort?.abort();
    document.removeEventListener('visibilitychange', this.handleVisibilityChange);
    window.removeEventListener('online', this.handleOnline);
    if (this.heartbeatInterval) clearInterval(this.heartbeatInterval);
    this.heartbeatInterval = null;
  }

  getState() {
    return this.state;
  }

  private patchState(patch: Partial<MatchState>) {
    this.state = { ...this.state, ...patch };
    if (!this.destroyed) {
      this.listeners.emit();
    }
  }

  private dispatchMatchmaking(action: MatchmakingAction) {
    this.patchState({ matchmaking: matchmakingReducer(this.state.matchmaking, action) });
  }

  private clearResponseTimeout() {
    if (this.responseTimeout) clearTimeout(this.responseTimeout);
    this.responseTimeout = null;
  }

  private armResponseTimeout(delay: number) {
    this.clearResponseTimeout();
    if (document.hidden) return;
    this.responseTimeout = setTimeout(() => {
      this.responseTimeout = null;
      if (!document.hidden) this.beginRecovery();
    }, delay);
  }

  private handleVisibilityChange = () => {
    this.clearResponseTimeout();
    if (!document.hidden) this.handleOnline();
  };

  private handleOnline = () => {
    if (!this.connectionWanted || this.destroyed) return;
    if (this.socketClient.isOpen()) {
      this.clearResponseTimeout();
      this.probeConnection(10_000);
    } else if (this.socketClient.isConnecting()) {
      this.armResponseTimeout(10_000);
    } else {
      this.beginRecovery(true);
    }
  };

  private probeConnection(timeout = SOCKET_STALE_AFTER_MS) {
    if (!this.connectionWanted || document.hidden || this.responseTimeout || !this.socketClient.isOpen()) return;
    // Arm before sending: any server activity proves liveness, but only a new
    // snapshot makes a replacement socket ready for gameplay.
    this.armResponseTimeout(timeout);
    this.sendGameCommand('ping', { userId: this.activeSession?.userId || '' });
  }

  private noteServerActivity() {
    if (this.awaitingSnapshot) return;
    this.clearResponseTimeout();
    if (this.state.connected && !this.state.connectionIssue) return;
    this.patchState({
      connected: true,
      connectionIssue: '',
      queueError: this.state.queueError === CONNECTION_ERROR_MESSAGE ? '' : this.state.queueError
    });
  }

  private stopConnection() {
    this.connectionGeneration += 1;
    this.connectionWanted = false;
    this.recoverAbort?.abort();
    this.recoverAbort = null;
    if (this.reconnectTimeout) clearTimeout(this.reconnectTimeout);
    this.reconnectTimeout = null;
    this.clearResponseTimeout();
    this.socketClient.close();
    this.awaitingSnapshot = false;
    this.reconnectAttempt = 0;
    this.patchState({ connected: false });
  }

  private beginRecovery(immediate = false) {
    if (!this.connectionWanted || this.destroyed) return;
    this.clearResponseTimeout();
    this.socketClient.close();
    this.awaitingSnapshot = true;
    this.patchState({ connected: false, connectionIssue: GAME_CONNECTION_ERROR_MESSAGE });
    this.dispatchMatchmaking({ type: 'ws_closed' });
    if (this.recoverAbort) return;
    if (this.reconnectTimeout) {
      if (!immediate) return;
      clearTimeout(this.reconnectTimeout);
    }
    const delay = immediate ? 0 : Math.min(1000 * 2 ** Math.min(this.reconnectAttempt, 4), 15000) * (0.8 + Math.random() * 0.2);
    this.reconnectTimeout = setTimeout(() => {
      this.reconnectTimeout = null;
      this.reconnectAttempt += 1;
      void this.startRecover();
    }, delay);
  }

  private markUnavailable(message: string) {
    this.stopConnection();
    this.patchState({ connected: false, connectionIssue: message });
    this.dispatchMatchmaking({ type: 'set_status', status: 'abandoned' });
  }

  resetConnectionState = () => {
    this.stopConnection();
    this.soloAbort?.abort();
    this.activeSession = null;
    this.patchState({
      ...this.state,
      connected: false,
      snapshot: null,
      lastFinalizedMatchId: '',
      activeMatchId: '',
      launchMatchId: '',
      resumeMatchId: '',
      sourcePartyId: '',
      sourcePartyInviteCode: '',
      returnTarget: { kind: 'home' },
      queueError: '',
      singleplayerError: '',
      connectionIssue: '',
    });
  };

  private async startRecover() {
    if (!this.connectionWanted || this.destroyed || this.recoverAbort) return;
    const targetMatchID = this.state.activeMatchId;
    if (!targetMatchID) return;
    const controller = new AbortController();
    this.recoverAbort = controller;
    // Bound the whole attempt, including session refresh. Late results are
    // ignored after cancellation, leaving a match, or starting another attempt.
    const timeout = setTimeout(() => {
      if (this.recoverAbort !== controller) return;
      controller.abort();
      this.recoverAbort = null;
      this.beginRecovery();
    }, 15_000);
    const isCurrent = () => !controller.signal.aborted && this.recoverAbort === controller && this.connectionWanted && !this.destroyed;
    try {
      const session = await this.sessionController.ensureFreshSession();
      if (!isCurrent()) return;
      if (!session) {
        this.markUnavailable('Session expired. Please sign in again.');
        return;
      }
      const view = await fetchMatchView(targetMatchID, controller.signal, session.accessToken);
      if (!isCurrent()) return;
      if (isOpenMatch(view) && view.playing) {
        this.recoverAbort = null;
        this.dispatchMatchmaking({ type: 'set_status', status: 'matched_connecting' });
        this.connectToMatch(session, view.matchId, {
          sourcePartyId: view.party?.id,
          sourcePartyInviteCode: view.party?.inviteCode,
          returnTarget: view.returnTarget
        });
        return;
      }
      if (view.status === 'ended' && view.result) {
        this.stopConnection();
        this.patchState({
          connected: false,
          snapshot: view.result,
          lastFinalizedMatchId: view.matchId,
          activeMatchId: view.matchId,
          connectionIssue: '',
          returnTarget: view.returnTarget || { kind: 'home' }
        });
        this.dispatchMatchmaking({ type: 'set_status', status: 'ready' });
        return;
      }
      const message = view.signInRequired
        ? 'Session expired. Please sign in again.'
        : view.currentMatchId
          ? 'This match was replaced by another session.'
          : 'Match unavailable.';
      this.markUnavailable(message);
    } catch {
      // Network failures and server errors retry through the same recovery loop.
    } finally {
      clearTimeout(timeout);
      if (this.recoverAbort === controller) {
        this.recoverAbort = null;
        this.beginRecovery();
      }
    }
  }

  private connectToMatch(session: AuthSessionSnapshot, matchId: string, source?: MatchSource) {
    if (!session.userId || !session.accessToken || !matchId) return;
    this.activeSession = session;
    this.connectionWanted = true;
    this.awaitingSnapshot = true;
    this.patchState({
      connected: false,
      activeMatchId: matchId,
      lastFinalizedMatchId: '',
      sourcePartyId: source?.sourcePartyId || '',
      sourcePartyInviteCode: source?.sourcePartyInviteCode || '',
      returnTarget: source?.returnTarget || this.state.returnTarget || { kind: 'home' }
    });
    try {
      this.socketClient.connect(matchId);
      this.armResponseTimeout(10_000);
    } catch {
      this.beginRecovery();
    }
  }

  resumeMatch = async (matchId: string, source?: MatchSource) => {
    this.stopConnection();
    this.soloAbort?.abort();
    const generation = this.connectionGeneration;
    const session = this.sessionController.getSessionSnapshot() || (await this.sessionController.ensureFreshSession());
    if (this.destroyed || generation !== this.connectionGeneration || !session || !matchId) {
      return false;
    }
    this.patchState({ queueError: '', connectionIssue: '' });
    this.dispatchMatchmaking({ type: 'set_status', status: 'matched_connecting' });
    this.connectToMatch(session, matchId, source);
    return true;
  };

  joinQueue = (queues: QueueVariant[] = ['moving']) => {
    this.stopConnection();
    this.soloAbort?.abort();
    this.patchState({ queueError: '', launchMatchId: '' });
    void (async () => {
      try {
        const session = await this.sessionController.getPlayableSession();
        if (!session) throw new Error('Unable to create session');
        this.dispatchMatchmaking({ type: 'join_requested', startedAt: Date.now() });
        await enterQueue(session.accessToken, queues.length ? queues : ['moving']);
      } catch (error) {
        this.patchState({ queueError: error instanceof Error ? error.message : 'Queue failed' });
        this.dispatchMatchmaking({ type: 'queue_error' });
      }
    })();
  };

  /** Where the server says this person is in the queue. */
  applyQueue(queue: QueueState) {
    if (queue.error) this.patchState({ queueError: queue.error });
    if (queue.queued) {
      this.dispatchMatchmaking({ type: 'queue_status', status: 'queueing', queuedAt: queue.queuedAt });
    } else if (this.state.matchmaking.status === 'queueing') {
      this.dispatchMatchmaking({ type: 'queue_status', status: 'left' });
    }
  }

  /** What the server says on every connection, which may have changed while disconnected. */
  applySession(queue: QueueState, activeMatch?: ActiveMatch) {
    this.patchState({ resumeMatchId: activeMatch?.matchId ?? '' });
    this.applyQueue(queue);
  }

  /** The server seated this player in a new match. Says whether anything still has to connect to it. */
  matchStarted(matchId: string) {
    // A solo start goes there itself, once it has the match.
    if (this.state.launchMatchId === matchId || this.singleplayerStartInFlight || this.state.activeMatchId === matchId) return false;
    this.patchState({ launchMatchId: matchId, queueError: '' });
    this.dispatchMatchmaking({ type: 'match_found' });
    this.playMatchFoundSfx();
    return true;
  }

  startSingleplayer = async (matchConfig?: MatchConfig, returnTarget?: MatchReturnTarget) => {
    if (this.singleplayerStartInFlight) {
      return '';
    }
    this.singleplayerStartInFlight = true;
    this.stopConnection();
    this.soloAbort?.abort();
    this.patchState({ singleplayerError: '', connectionIssue: '' });
    this.dispatchMatchmaking({ type: 'set_status', status: 'matched_connecting' });
    const controller = new AbortController();
    this.soloAbort = controller;

    try {
      const session = await this.sessionController.getPlayableSession();
      if (controller.signal.aborted || this.destroyed) return '';
      if (!session) {
        this.patchState({ singleplayerError: 'Unable to create session' });
        this.dispatchMatchmaking({ type: 'set_status', status: 'ready' });
        return '';
      }
      const view = await startSoloMatch(
        session.accessToken,
        controller.signal,
        matchConfig,
        returnTarget || { kind: 'home' },
      );
      if (!view.matchId || !isOpenMatch(view)) {
        throw new Error('Singleplayer unavailable');
      }
      if (controller.signal.aborted || this.destroyed || this.soloAbort !== controller) {
        return '';
      }
      this.patchState({ launchMatchId: view.matchId });
      this.connectToMatch(session, view.matchId, { returnTarget: view.returnTarget });
      return view.matchId;
    } catch (error: any) {
      if (error?.name === 'AbortError') {
        return '';
      }
      this.patchState({ singleplayerError: error?.message || 'Singleplayer unavailable' });
      this.dispatchMatchmaking({ type: 'set_status', status: 'ready' });
      return '';
    } finally {
      this.singleplayerStartInFlight = false;
      if (this.soloAbort === controller) {
        this.soloAbort = null;
      }
    }
  };

  clearSingleplayerError = () => {
    if (!this.state.singleplayerError) return;
    this.patchState({ singleplayerError: '' });
  };

  cancelQueue = () => {
    this.dispatchMatchmaking({ type: 'leave_requested' });
    this.patchState({ queueError: '' });
    void this.sessionController.ensureFreshSession().then((session) => {
      if (session) return leaveQueue(session.accessToken);
    }).catch(() => undefined);
  };

  setStatus = (status: MatchmakingState['status']) => {
    this.dispatchMatchmaking({ type: 'set_status', status });
  };

  private playMatchFoundSfx() {
    this.sfxController?.play('duel-game-start');
  }

  sendGameCommand = (type: string, payload: Record<string, unknown>, options?: SendGameCommandOptions) => {
    if (!this.socketClient.isOpen()) {
      if (!options?.silent) this.beginRecovery();
      return false;
    }
    if (type !== 'ping' && !this.state.connected && !options?.silent) return false;
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
        this.beginRecovery();
      }
      return false;
    }
  };
}
