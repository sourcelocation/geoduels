import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createRuntimeConfig, normalizeHTTPBase, normalizeWSBase } from './runtime-config';

describe('runtime-config', () => {
    const originalAppVersion = process.env.NEXT_PUBLIC_APP_VERSION;
    const originalGitSha = process.env.NEXT_PUBLIC_GIT_SHA;

    beforeEach(() => {
        vi.unstubAllEnvs();
        delete process.env.NEXT_PUBLIC_APP_VERSION;
        delete process.env.NEXT_PUBLIC_GIT_SHA;
    });

    afterEach(() => {
        vi.unstubAllEnvs();
        if (originalAppVersion === undefined) {
            delete process.env.NEXT_PUBLIC_APP_VERSION;
        } else {
            process.env.NEXT_PUBLIC_APP_VERSION = originalAppVersion;
        }
        if (originalGitSha === undefined) {
            delete process.env.NEXT_PUBLIC_GIT_SHA;
        } else {
            process.env.NEXT_PUBLIC_GIT_SHA = originalGitSha;
        }
    });

    it('builds config from provided runtime overrides', () => {
        const config = createRuntimeConfig({
            NEXT_PUBLIC_QUEUE_URL: 'https://queue.example.com',
            NEXT_PUBLIC_REALTIME_URL: 'wss://realtime.example.com',
            NEXT_PUBLIC_API_URL: 'https://api.example.com',
            NEXT_PUBLIC_GOOGLE_CLIENT_ID: 'google-client',
            NEXT_PUBLIC_GOOGLE_ALLOWED_ORIGINS: 'https://one.test, https://two.test',
            NEXT_PUBLIC_TURNSTILE_SITE_KEY: 'turnstile-site-key',
            NEXT_PUBLIC_GOOGLE_EMBED_KEY: 'embed-key',
            NEXT_PUBLIC_APP_VERSION: 'sha-123'
        });

        expect(config.queueURL).toBe('https://queue.example.com');
        expect(config.realtimeBaseURL).toBe('wss://realtime.example.com');
        expect(config.apiURL).toBe('https://api.example.com');
        expect(config.googleClientId).toBe('google-client');
        expect(config.googleAllowedOrigins).toEqual(['https://one.test', 'https://two.test']);
        expect(config.turnstileSiteKey).toBe('turnstile-site-key');
        expect(config.googleEmbedKey).toBe('embed-key');
        expect(config.appVersion).toBe('sha-123');
    });

    it('ignores placeholder runtime values', () => {
        const config = createRuntimeConfig({
            NEXT_PUBLIC_API_URL: 'REPLACE_WITH_API_URL',
            NEXT_PUBLIC_APP_VERSION: 'REPLACE_WITH_SHA'
        });

        expect(config.apiURL).toBe('http://localhost:8080');
        expect(config.appVersion).toBe('dev');
    });

    it('normalizes websocket and http base urls', () => {
        expect(normalizeHTTPBase('ws://localhost:8090')).toBe('http://localhost:8090');
        expect(normalizeHTTPBase('wss://example.com')).toBe('https://example.com');
        expect(normalizeWSBase('http://localhost:8092')).toBe('ws://localhost:8092');
        expect(normalizeWSBase('https://example.com')).toBe('wss://example.com');
    });
});
