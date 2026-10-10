import type { Snapshot, TeamPing } from '../../game/model/types';
import type { RuntimeConfig } from '../../../lib/runtime-config';
import { matchSocketURL } from './queue-client';

type Handlers = {
  onOpen: () => void;
  onClose: () => void;
  onError: () => void;
  onActivity: () => void;
  onSnapshot: (snapshot: Snapshot) => void;
  onTeamPing: (ping: TeamPing) => void;
  onAckError: (message: string) => void;
  onProtocolError: () => void;
};

export class GameplaySocketClient {
  private socket: WebSocket | null = null;
  private readonly config: RuntimeConfig;
  private readonly handlers: Handlers;

  constructor(config: RuntimeConfig, handlers: Handlers) {
    this.config = config;
    this.handlers = handlers;
  }

  connect(matchId: string, accessToken: string) {
    const target = matchSocketURL(this.config, matchId, accessToken);
    this.close();
    const ws = new WebSocket(target);
    this.socket = ws;

    ws.onopen = () => {
      if (this.socket !== ws) return;
      this.handlers.onOpen();
    };

    ws.onerror = () => {
      if (this.socket !== ws) return;
      this.handlers.onError();
    };

    ws.onclose = () => {
      if (this.socket !== ws) return;
      this.socket = null;
      this.handlers.onClose();
    };

    ws.onmessage = (evt) => {
      if (this.socket !== ws) return;
      let msg: any;
      try {
        msg = JSON.parse(evt.data);
      } catch {
        this.handlers.onProtocolError();
        return;
      }
      if (msg.kind === 'ack') {
        this.handlers.onActivity();
        if (msg.status === 'error') {
          this.handlers.onAckError(msg.message || msg.errorCode || 'Command failed');
        }
        return;
      }
      if (
        msg.kind !== 'event'
      ) return;
      if (msg.type === 'team.ping') {
        this.handlers.onActivity();
        this.handlers.onTeamPing(msg.payload as TeamPing);
        return;
      }
      if (!['match.snapshot', 'match.state', 'match.lifecycle.v2.snapshot'].includes(msg.type)) return;
      const snapshot = msg.payload as Snapshot;
      const serverTs = typeof msg.serverTs === 'number' && Number.isFinite(msg.serverTs) ? msg.serverTs : undefined;
      this.handlers.onSnapshot(serverTs === undefined ? snapshot : { ...snapshot, serverUnixMs: serverTs });
    };

    return target;
  }

  isOpen() {
    return !!this.socket && this.socket.readyState === WebSocket.OPEN;
  }

  isConnecting() {
    return !!this.socket && this.socket.readyState === WebSocket.CONNECTING;
  }

  send(command: Record<string, unknown>) {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      return false;
    }
    this.socket.send(JSON.stringify(command));
    return true;
  }

  close() {
    const current = this.socket;
    this.socket = null;
    if (!current) return;
    current.close();
  }
}
