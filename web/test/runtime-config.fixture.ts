import type { RuntimeConfig } from '../lib/runtime-config';

export function createRuntimeConfigFixture(overrides: Partial<RuntimeConfig> = {}): RuntimeConfig {
  return {
    siteURL: 'http://localhost:3000',
    apiURL: 'http://localhost:8080',
    googleClientId: '',
    googleAllowedOrigins: [],
    discordClientId: '',
    turnstileSiteKey: '',
    googleEmbedKey: 'NO_KEY_DEFINED',
    appVersion: 'dev',
    roundDurationMs: 45_000,
    maxHP: 6_000,
    socketHeartbeatIntervalMs: 20_000,
    socketStaleAfterMs: 35_000,
    connectionErrorMessage: 'Connection error',
    gameConnectionErrorMessage: 'Connection lost. Reconnecting...',
    ...overrides
  };
}
