export async function readError(resp: Response, fallback: string) {
  const text = await resp.text();
  return text || fallback;
}

// The API is on the site's origin under /api. The server, rendering a page, reaches it directly.
function apiURL(path: string) {
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  if (typeof window !== "undefined") return normalizedPath;
  if (!process.env.API_URL) throw new Error("API_URL must be set to render pages that read the API.");
  return new URL(normalizedPath, process.env.API_URL).toString();
}

export function apiSocketURL(path: string): string {
  return new URL(path, window.location.origin).toString().replace(/^http/, "ws");
}

export function authHeaders(accessToken?: string): HeadersInit | undefined {
  return accessToken ? { Authorization: `Bearer ${accessToken}` } : undefined;
}

export function mergeHeaders(...headers: Array<HeadersInit | undefined>): HeadersInit {
  return Object.assign({}, ...headers.filter(Boolean));
}

export function apiFetch(path: string, init?: RequestInit) {
  return fetch(apiURL(path), init);
}

export async function expectJSON<T>(response: Response, fallback = "Request failed"): Promise<T> {
  if (!response.ok) throw new Error(await readError(response, fallback));
  return response.json();
}
