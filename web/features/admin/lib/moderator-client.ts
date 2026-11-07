import { apiFetch, readError } from "../../../lib/http";

export async function requestModeratorSubject(
  accessToken: string,
  userId: string,
) {
  const resp = await apiFetch(
    `/api/staff/players/${encodeURIComponent(userId)}/review`,
    { headers: { Authorization: `Bearer ${accessToken}` } },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load moderation subject"));
  }
  return resp.json();
}

export async function requestModeratorCheatingBan(
  accessToken: string,
  userId: string,
  reason: string,
) {
  const resp = await apiFetch(
    `/api/staff/players/${encodeURIComponent(userId)}/ban`,
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
  accessToken: string,
  userId: string,
  reason: string,
) {
  const resp = await apiFetch(
    `/api/staff/players/${encodeURIComponent(userId)}/unban`,
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
  accessToken: string,
  userId: string,
  kind: "chat" | "report",
  reason: string,
  muted: boolean,
) {
  const resp = await apiFetch(`/api/staff/players/${encodeURIComponent(userId)}/mutes/${kind}`, {
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
  accessToken: string,
) {
  const resp = await apiFetch(`/api/staff/reports`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load moderation signals"));
  }
  return resp.json();
}

export async function requestModeratorLog(
  accessToken: string,
) {
  const resp = await apiFetch(`/api/staff/audit`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load moderation log"));
  }
  return resp.json();
}
