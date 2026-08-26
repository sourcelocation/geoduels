import type { Snapshot, TeamPing } from '../../game/model/types';
import type { RuntimeConfig } from '../../../lib/runtime-config';
import { normalizeWSBase } from '../../../lib/runtime-config';
import type { AuthSessionSnapshot } from '../../auth/session';

type Handlers = {
  onOpen: () => void;
  onClose: (event: { expected: boolean }) => void;
  onError: () => void;
  onActivity: () => void;
  onSnapshot: (snapshot: Snapshot) => void;
  onTeamPing: (ping: TeamPing) => void;
  onAckError: (message: string) => void;
  onProtocolError: () => void;
};

type ManagedSocket = {
  ws: WebSocket;
  expectedClose: boolean;
};

export class GameplaySocketClient {
  private socket: ManagedSocket | null = null;
  private readonly config: RuntimeConfig;
  private readonly handlers: Handlers;

  constructor(config: RuntimeConfig, handlers: Handlers) {
    this.config = config;
    this.handlers = handlers;
  }

  connect(session: AuthSessionSnapshot, node: string, wsPath: string, ticket: string) {
    const base = normalizeWSBase(this.config.realtimeBaseURL).replace(/\/$/, '');
    const path = (wsPath || `/ws/${node}`).startsWith('/') ? (wsPath || `/ws/${node}`) : `/${wsPath || `ws/${node}`}`;
    const target = `${base}${path}?ticket=${encodeURIComponent(ticket)}`;
    this.close(true);
    const ws = new WebSocket(target);
    const managed: ManagedSocket = { ws, expectedClose: false };
    this.socket = managed;

    ws.onopen = () => {
      if (this.socket?.ws !== ws) return;
      this.handlers.onOpen();
    };

    ws.onerror = () => {
      if (this.socket?.ws !== ws) return;
      this.handlers.onError();
    };

    ws.onclose = () => {
      if (this.socket?.ws === ws) {
        this.socket = null;
      }
      this.handlers.onClose({ expected: managed.expectedClose });
    };

    ws.onmessage = (evt) => {
      if (this.socket?.ws !== ws) return;
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
    return !!this.socket?.ws && this.socket.ws.readyState === WebSocket.OPEN;
  }

  isOpenOrConnecting() {
    return !!this.socket?.ws && (this.socket.ws.readyState === WebSocket.OPEN || this.socket.ws.readyState === WebSocket.CONNECTING);
  }

  send(command: Record<string, unknown>) {
    if (!this.socket?.ws || this.socket.ws.readyState !== WebSocket.OPEN) {
      return false;
    }
    this.socket.ws.send(JSON.stringify(command));
    return true;
  }

  close(expected = true) {
    const current = this.socket;
    this.socket = null;
    if (!current) return;
    current.expectedClose = expected;
    current.ws.close();
  }
}
