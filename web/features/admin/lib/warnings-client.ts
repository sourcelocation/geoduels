import { apiFetch, readError } from "../../../lib/http";

export type ModerationWarning = {
  id: number;
  category: string;
  message: string;
  previousNickname?: string;
  resetNickname?: string;
  evidenceMessageId?: string;
  actorUserId?: string;
  actorName?: string;
  createdAt: string;
  acknowledgedAt?: string;
  withdrawnAt?: string;
};

export async function warningRequest<T>(token: string, path: string, method = "GET", body?: unknown): Promise<T> {
  const response = await apiFetch(`/api/v1/${path}`, {
    method,
    headers: { Authorization: `Bearer ${token}`, ...(body ? { "content-type": "application/json" } : {}) },
    ...(body ? { body: JSON.stringify(body) } : {}),
  });
  if (!response.ok) throw new Error(await readError(response, "Unable to update warnings"));
  if (response.status === 204 || response.status === 201) return undefined as T;
  return response.json();
}
