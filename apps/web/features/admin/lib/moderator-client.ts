import { apiFetch, readError } from "../../../lib/http";
import type { RuntimeConfig } from "../../../lib/runtime-config";

export async function requestModeratorSubject(
  config: RuntimeConfig,
  accessToken: string,
  userId: string,
) {
  const resp = await apiFetch(
    config,
    `/v1/moderator/subjects/${encodeURIComponent(userId)}`,
    { headers: { Authorization: `Bearer ${accessToken}` } },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load moderation subject"));
  }
  return resp.json();
}

export async function requestModeratorCheatingBan(
  config: RuntimeConfig,
  accessToken: string,
  userId: string,
  reason: string,
) {
  const resp = await apiFetch(
    config,
    `/v1/moderator/subjects/${encodeURIComponent(userId)}/cheating-ban`,
    {
      method: "POST",
      headers: {
        "content-type": "application/json",
        Authorization: `Bearer ${accessToken}`,
      },
      body: JSON.stringify({ reason }),
    },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to ban player and issue refunds"));
  }
  return resp.json();
}

export async function requestModeratorSubjectUnban(
  config: RuntimeConfig,
  accessToken: string,
  userId: string,
  reason: string,
) {
  const resp = await apiFetch(
    config,
    `/v1/moderator/subjects/${encodeURIComponent(userId)}/unban`,
    {
      method: "POST",
      headers: {
        "content-type": "application/json",
        Authorization: `Bearer ${accessToken}`,
      },
      body: JSON.stringify({ reason }),
    },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to unban player"));
  }
}

export async function requestModeratorSubjectMute(
  config: RuntimeConfig,
  accessToken: string,
  userId: string,
  kind: "chat" | "report",
  reason: string,
  muted: boolean,
) {
  const resp = await apiFetch(config, `/v1/moderator/subjects/${encodeURIComponent(userId)}/mutes/${kind}`, {
    method: muted ? "POST" : "DELETE",
    headers: {
      "content-type": "application/json",
      Authorization: `Bearer ${accessToken}`,
    },
    body: muted ? JSON.stringify({ reason, durationHours: 168 }) : undefined,
  });
  if (!resp.ok) throw new Error(await readError(resp, `Failed to ${muted ? "mute" : "unmute"} player`));
}

export async function requestModeratorSignals(
  config: RuntimeConfig,
  accessToken: string,
) {
  const resp = await apiFetch(config, `/v1/moderator/signals`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load moderation signals"));
  }
  return resp.json();
}

export async function requestModeratorLog(
  config: RuntimeConfig,
  accessToken: string,
) {
  const resp = await apiFetch(config, `/v1/moderator/log`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load moderation log"));
  }
  return resp.json();
}
