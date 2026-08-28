import type { RuntimeConfig } from "../../lib/runtime-config";
import {
  requestGuestSession,
  requestLogout,
  requestRefreshSession,
  requestSession,
  type AuthSessionPayload,
} from "./lib/auth-client";
import { decodeAccessTokenExpiry } from "./lib/token-expiry";
import type { AuthSessionSnapshot } from "./session";
import { dismissAllModals, hasMountedModals } from "../../components/ui/modal-dismissal";

export type GuestVerification = () => Promise<string>;
type AuthBroadcast = { source: string; type: "changed" | "logout" };
const AUTH_LOCK = "geoduels-auth-cookie";
const AUTH_CHANNEL = "geoduels-auth";

/** The sole browser owner allowed to commit authentication state. */
export class AuthGateway {
  private payload: AuthSessionPayload | null = null;
  private restored = false;
  private generation = 0;
  private restorePromise: Promise<AuthSessionSnapshot | null> | null = null;
  private restoreGeneration = -1;
  private refreshPromise: Promise<AuthSessionSnapshot | null> | null = null;
  private guestVerification: GuestVerification | null = null;
  private channel: BroadcastChannel | null = null;
  private readonly source = Math.random().toString(36).slice(2);
  private readonly listeners = new Set<(snapshot: AuthSessionSnapshot | null) => void>();

  constructor(private readonly config: RuntimeConfig) {
    if (typeof BroadcastChannel !== "undefined") {
      try {
        this.channel = new BroadcastChannel(AUTH_CHANNEL);
        this.channel.onmessage = (event: MessageEvent<AuthBroadcast>) => this.receiveBroadcast(event.data);
      } catch {
        this.channel = null;
      }
    }
  }

  dispose() {
    this.channel?.close();
    this.channel = null;
    this.listeners.clear();
  }

  setGuestVerification(callback: GuestVerification | null) {
    this.guestVerification = callback;
  }

  subscribe(listener: (snapshot: AuthSessionSnapshot | null) => void) {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  getPayload() {
    return this.payload;
  }

  isRestored() {
    return this.restored;
  }

  getSnapshot(): AuthSessionSnapshot | null {
    return toSnapshot(this.payload);
  }

  private publish() {
    const snapshot = this.getSnapshot();
    this.listeners.forEach((listener) => listener(snapshot));
  }

  private commit(payload: AuthSessionPayload | null, restored = true) {
    this.payload = payload;
    this.restored = restored;
    this.publish();
    return this.getSnapshot();
  }

  private announce(type: AuthBroadcast["type"]) {
    this.channel?.postMessage({ source: this.source, type } satisfies AuthBroadcast);
  }

  private async receiveBroadcast(message: AuthBroadcast) {
    if (!message || message.source === this.source) return;
    ++this.generation;
    if (message.type === "logout") {
      if (hasMountedModals()) await dismissAllModals();
      return void this.commit(null, true);
    }
    this.commit(null, false);
    void this.bootstrap({ force: true }).catch(() => undefined);
  }

  private async withCookieLock<T>(work: () => Promise<T>): Promise<T> {
    const locks = typeof navigator !== "undefined" ? navigator.locks : undefined;
    if (!locks) return work();
    return locks.request(AUTH_LOCK, work);
  }

  /** Domain mutations supersede outstanding network results. */
  applyPayload(payload: AuthSessionPayload | null) {
    ++this.generation;
    return this.commit(payload);
  }

  applySnapshot(
    snapshot: AuthSessionSnapshot | null,
    identity?: {
      isGuest?: boolean;
      isAdmin?: boolean;
      isModerator?: boolean;
      displayName?: string;
      email?: string;
      avatarUrl?: string;
    },
  ) {
    if (!snapshot) return this.applyPayload(null);
    return this.applyPayload({
      accessToken: snapshot.accessToken,
      nicknameRequired: snapshot.nicknameRequired,
      authMigrationRequired: snapshot.authMigrationRequired,
      recoveryAvailable: snapshot.recoveryAvailable,
      linkedProviders: snapshot.linkedProviders,
      canPlay: snapshot.canPlay,
      suggestedNickname: snapshot.nicknameInput,
      user: {
        ...(this.payload?.user || {}),
        id: snapshot.userId,
        ...(typeof identity?.isGuest === "boolean" ? { isGuest: identity.isGuest } : {}),
        ...(typeof identity?.isAdmin === "boolean" ? { isAdmin: identity.isAdmin } : {}),
        ...(typeof identity?.isModerator === "boolean" ? { isModerator: identity.isModerator } : {}),
        ...(identity?.displayName ? { display_name: identity.displayName } : {}),
        ...(identity?.email ? { email: identity.email } : {}),
        ...(identity?.avatarUrl ? { avatar_url: identity.avatarUrl } : {}),
      },
    });
  }

  async bootstrap(options?: { force?: boolean }) {
    if (this.restorePromise && this.restoreGeneration === this.generation) return this.restorePromise;
    if (this.restored && !options?.force) return this.getSnapshot();

    const generation = this.generation;
    this.restoreGeneration = generation;
    const promise = requestSession(this.config)
      .then((payload) => {
        if (generation === this.generation) this.commit(payload);
        return this.getSnapshot();
      })
      .catch((error) => {
        if (generation === this.generation) {
          this.restored = true;
          this.publish();
        }
        throw error;
      })
      .finally(() => {
        if (this.restorePromise === promise) this.restorePromise = null;
      });
    this.restorePromise = promise;
    return promise;
  }

  /** OAuth is an identity boundary, not an independent query owner. */
  async oauthCompleted() {
    ++this.generation;
    this.restored = false;
    this.publish();
    const session = await this.bootstrap({ force: true });
    this.announce("changed");
    return session;
  }

  async refresh() {
    if (this.refreshPromise) return this.refreshPromise;
    const requestedGeneration = this.generation;
    const promise = this.withCookieLock(async () => {
      // Another tab may have rotated the cookie while this tab waited for the lock.
      if (requestedGeneration !== this.generation) return this.bootstrap({ force: true });
      const generation = this.generation;
      const payload = await requestRefreshSession(this.config);
      if (generation === this.generation) {
        const snapshot = this.commit(payload);
        this.announce("changed");
        return snapshot;
      }
      return this.getSnapshot();
    }).finally(() => {
      if (this.refreshPromise === promise) this.refreshPromise = null;
    });
    this.refreshPromise = promise;
    return promise;
  }

  async ensurePlayableSession() {
    const operationGeneration = this.generation;
    let session = this.getSnapshot();
    if (!session) session = await this.bootstrap();
    if (session && !session.nicknameRequired && !session.authMigrationRequired) {
      return this.ensureFreshSession(60_000, { forceRefresh: !!this.payload?.user?.isGuest });
    }
    if (operationGeneration !== this.generation) return this.getSnapshot();

    const guest = await this.withCookieLock(async () => {
      if (operationGeneration !== this.generation) return null;
      const generation = ++this.generation;
      const token = this.guestVerification ? await this.guestVerification() : "";
      const payload = await requestGuestSession(this.config, token);
      if (generation === this.generation) {
        const snapshot = this.commit(payload);
        this.announce("changed");
        return snapshot;
      }
      return this.getSnapshot();
    });
    return guest || this.getSnapshot();
  }

  async ensureFreshSession(
    minValidityMs = 60_000,
    options?: { forceRefresh?: boolean; allowNicknameRequired?: boolean },
  ) {
    const current = this.getSnapshot();
    if (!current || (!options?.allowNicknameRequired && current.nicknameRequired)) return null;
    if (!options?.forceRefresh && current.expiresAt && Date.now() + minValidityMs < current.expiresAt) {
      return current;
    }
    const refreshed = await this.refresh();
    if (refreshed) return refreshed;
    return !options?.forceRefresh && current.expiresAt && Date.now() < current.expiresAt ? current : null;
  }

  async logout() {
    ++this.generation;
    if (hasMountedModals()) await dismissAllModals();
    this.commit(null);
    await this.withCookieLock(async () => {
      await requestLogout(this.config);
    });
    // A remote auth-change may have arrived while this tab waited for the lock.
    // Completion is therefore a second, final identity boundary.
    ++this.generation;
    this.commit(null);
    this.announce("logout");
  }

  async clear() {
    if (hasMountedModals()) await dismissAllModals();
    this.applyPayload(null);
  }
}

function toSnapshot(payload: AuthSessionPayload | null): AuthSessionSnapshot | null {
  if (!payload?.user?.id || !payload.accessToken) return null;
  return {
    userId: payload.user.id,
    accessToken: payload.accessToken,
    nicknameRequired: !!payload.nicknameRequired,
    authMigrationRequired: !!payload.authMigrationRequired,
    recoveryAvailable: !!payload.recoveryAvailable,
    linkedProviders: payload.linkedProviders || [],
    canPlay: payload.canPlay ?? (!payload.nicknameRequired && !payload.authMigrationRequired),
    nicknameInput: payload.suggestedNickname || "",
    expiresAt: decodeAccessTokenExpiry(payload.accessToken),
  };
}

let gateway: AuthGateway | null = null;
let gatewayConfig: RuntimeConfig | null = null;

export function getAuthGateway(config: RuntimeConfig) {
  if (!gateway || gatewayConfig !== config) {
    gateway?.dispose();
    gateway = new AuthGateway(config);
    gatewayConfig = config;
  }
  return gateway;
}
