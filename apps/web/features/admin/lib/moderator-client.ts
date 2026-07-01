import { apiFetch, readError } from "../../../lib/http";
import type { RuntimeConfig } from "../../../lib/runtime-config";

export async function requestModeratorTasks(
  config: RuntimeConfig,
  accessToken: string,
  view = "queue",
) {
  const resp = await apiFetch(config, `/v1/moderator/tasks?view=${encodeURIComponent(view)}`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load moderation tasks"));
  }
  return resp.json();
}

export async function requestModeratorClaimTask(
  config: RuntimeConfig,
  accessToken: string,
  taskId: number,
) {
  const resp = await apiFetch(
    config,
    `/v1/moderator/tasks/${encodeURIComponent(taskId)}/claim`,
    { method: "POST", headers: { Authorization: `Bearer ${accessToken}` } },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to claim moderation task"));
  }
  return resp.json();
}

export async function requestModeratorReleaseTask(
  config: RuntimeConfig,
  accessToken: string,
  taskId: number,
) {
  const resp = await apiFetch(
    config,
    `/v1/moderator/tasks/${encodeURIComponent(taskId)}/release`,
    { method: "POST", headers: { Authorization: `Bearer ${accessToken}` } },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to release moderation task"));
  }
  return resp.json();
}

export async function requestModeratorIncident(
  config: RuntimeConfig,
  accessToken: string,
  incidentId: number,
) {
  const resp = await apiFetch(
    config,
    `/v1/moderator/incidents/${encodeURIComponent(incidentId)}`,
    { headers: { Authorization: `Bearer ${accessToken}` } },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load moderation incident"));
  }
  return resp.json();
}

export async function requestModeratorVerdict(
  config: RuntimeConfig,
  accessToken: string,
  incidentId: number,
  action: {
    taskId?: number;
    verdict: string;
    reasonCode: string;
    note?: string;
    enforcementAction?: string;
    durationHours?: number;
  },
) {
  const resp = await apiFetch(
    config,
    `/v1/moderator/incidents/${encodeURIComponent(incidentId)}/verdicts`,
    {
      method: "POST",
      headers: {
        "content-type": "application/json",
        Authorization: `Bearer ${accessToken}`,
      },
      body: JSON.stringify(action),
    },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to submit moderation verdict"));
  }
  return resp.json();
}

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

export async function requestModeratorEnforcementActions(
  config: RuntimeConfig,
  accessToken: string,
) {
  const resp = await apiFetch(config, `/v1/moderator/enforcement/actions`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load enforcement actions"));
  }
  return resp.json();
}
