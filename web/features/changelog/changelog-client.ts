
import { apiFetch } from "../../lib/http";
import type { ChangelogPost } from "./types";

export async function requestChangelogPosts() {
  const resp = await apiFetch("/api/content/changelog");
  if (!resp.ok) {
    throw new Error("Failed to load changelog");
  }
  return resp.json() as Promise<{ posts: ChangelogPost[] }>;
}

export async function requestChangelogPost(slug: string) {
  const resp = await apiFetch(`/api/content/changelog/${encodeURIComponent(slug)}`);
  if (resp.status === 404) {
    return null;
  }
  if (!resp.ok) {
    throw new Error("Failed to load changelog post");
  }
  return resp.json() as Promise<ChangelogPost>;
}
