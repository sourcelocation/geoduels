import { validatePublicConfig } from '../config/public-runtime-config.mjs';

export type PublicConfig = {
  NEXT_PUBLIC_SITE_URL: string;
  NEXT_PUBLIC_API_URL: string;
  NEXT_PUBLIC_GOOGLE_CLIENT_ID: string;
  NEXT_PUBLIC_GOOGLE_ALLOWED_ORIGINS: string;
  NEXT_PUBLIC_DISCORD_CLIENT_ID: string;
  NEXT_PUBLIC_TURNSTILE_SITE_KEY: string;
  NEXT_PUBLIC_GOOGLE_EMBED_KEY: string;
  NEXT_PUBLIC_APP_VERSION: string;
};

export type RuntimeConfig = {
  siteURL: string;
  apiURL: string;
  googleClientId: string;
  googleAllowedOrigins: string[];
  discordClientId: string;
  turnstileSiteKey: string;
  googleEmbedKey: string;
  appVersion: string;
  roundDurationMs: number;
  maxHP: number;
  socketHeartbeatIntervalMs: number;
  socketStaleAfterMs: number;
  connectionErrorMessage: string;
  gameConnectionErrorMessage: string;
};

function splitOrigins(value: string) {
  return value
    .split(',')
    .map((origin) => origin.trim())
    .filter(Boolean);
}

export function createRuntimeConfig(publicRuntimeConfig: PublicConfig): RuntimeConfig {
  validatePublicConfig(publicRuntimeConfig);
  const config: RuntimeConfig = {
    siteURL: publicRuntimeConfig.NEXT_PUBLIC_SITE_URL.replace(/\/$/, ''),
    apiURL: publicRuntimeConfig.NEXT_PUBLIC_API_URL,
    googleClientId: publicRuntimeConfig.NEXT_PUBLIC_GOOGLE_CLIENT_ID,
    googleAllowedOrigins: splitOrigins(publicRuntimeConfig.NEXT_PUBLIC_GOOGLE_ALLOWED_ORIGINS || ''),
    discordClientId: publicRuntimeConfig.NEXT_PUBLIC_DISCORD_CLIENT_ID,
    turnstileSiteKey: publicRuntimeConfig.NEXT_PUBLIC_TURNSTILE_SITE_KEY,
    googleEmbedKey: publicRuntimeConfig.NEXT_PUBLIC_GOOGLE_EMBED_KEY,
    appVersion: publicRuntimeConfig.NEXT_PUBLIC_APP_VERSION,
    roundDurationMs: 45_000,
    maxHP: 6_000,
    socketHeartbeatIntervalMs: 20_000,
    socketStaleAfterMs: 35_000,
    connectionErrorMessage: 'Connection error',
    gameConnectionErrorMessage: 'Connection lost. Reconnecting...'
  };
  if (process.env.NODE_ENV !== 'production') {
    Object.freeze(config.googleAllowedOrigins);
    Object.freeze(config);
  }
  return config;
}

export function normalizeHTTPBase(value: string): string {
  if (!value) return '';
  if (value.startsWith('ws://')) return `http://${value.slice(5)}`;
  if (value.startsWith('wss://')) return `https://${value.slice(6)}`;
  return value;
}

export function normalizeWSBase(value: string): string {
  if (!value) return '';
  if (value.startsWith('http://')) return `ws://${value.slice(7)}`;
  if (value.startsWith('https://')) return `wss://${value.slice(8)}`;
  return value;
}
