import type { RuntimeConfig } from "./runtime-config";

export async function readError(resp: Response, fallback: string) {
  const text = await resp.text();
  return text || fallback;
}

export function apiPath(config: Pick<RuntimeConfig, "apiURL">, path: string): string {
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  const base = config.apiURL.trim();
  if (!base) return normalizedPath;
  return new URL(normalizedPath, base.endsWith("/") ? base : `${base}/`).toString();
}

function serverAPIBase() {
  return (
    process.env.API_PROXY_URL ||
    process.env.NEXT_PUBLIC_SITE_URL ||
    "http://localhost:3000"
  );
}

export function apiFetchPath(config: Pick<RuntimeConfig, "apiURL">, path: string): string {
  const browserPath = apiPath(config, path);
  if (config.apiURL.trim()) {
    return browserPath;
  }
  const serverBase = process.env.API_PROXY_URL;
  if (serverBase) {
    return new URL(browserPath, serverBase).toString();
  }
  if (typeof window !== "undefined") {
    return browserPath;
  }
  return new URL(browserPath, serverAPIBase()).toString();
}

export function authHeaders(accessToken?: string): HeadersInit | undefined {
  return accessToken ? { Authorization: `Bearer ${accessToken}` } : undefined;
}

export function mergeHeaders(...headers: Array<HeadersInit | undefined>): HeadersInit {
  return Object.assign({}, ...headers.filter(Boolean));
}

export function apiFetch(config: Pick<RuntimeConfig, "apiURL">, path: string, init?: RequestInit) {
  return fetch(apiFetchPath(config, path), init);
}

export async function expectJSON<T>(response: Response, fallback = "Request failed"): Promise<T> {
  if (!response.ok) throw new Error(await readError(response, fallback));
  return response.json();
}
