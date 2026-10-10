// Explicit browser-safe allowlist. Never serialize the entire environment.
export const publicConfigKeys = [
  'NEXT_PUBLIC_SITE_URL', 'NEXT_PUBLIC_API_URL', 'NEXT_PUBLIC_GOOGLE_CLIENT_ID',
  'NEXT_PUBLIC_GOOGLE_ALLOWED_ORIGINS', 'NEXT_PUBLIC_DISCORD_CLIENT_ID',
  'NEXT_PUBLIC_TURNSTILE_SITE_KEY', 'NEXT_PUBLIC_GOOGLE_EMBED_KEY',
  'NEXT_PUBLIC_APP_VERSION'
];

export function resolvePublicConfig(env, development = false) {
  const config = Object.fromEntries(publicConfigKeys.map((key) => {
    const value = env[key];
    return [key, typeof value === 'string' && !value.startsWith('REPLACE_WITH_') ? value : ''];
  }));
  config.NEXT_PUBLIC_SITE_URL ||= development ? 'http://localhost:3000' : '';
  config.NEXT_PUBLIC_API_URL ||= development ? 'http://localhost:8080' : config.NEXT_PUBLIC_SITE_URL;
  config.NEXT_PUBLIC_APP_VERSION ||= (env.NEXT_PUBLIC_GIT_SHA || 'dev').slice(0, 12);
  return config;
}

export function validatePublicConfig(config) {
  for (const key of publicConfigKeys.slice(0, 2)) {
    const protocols = ['http:', 'https:'];
    let url;
    try { url = new URL(config[key]); } catch { /* Report the configuration key below. */ }
    if (!url || !protocols.includes(url.protocol) || url.username || url.password || url.search || url.hash) {
      throw new Error(`Invalid runtime configuration: ${key} must be an absolute ${protocols.join('/')} URL without credentials, query, or fragment.`);
    }
  }
}
