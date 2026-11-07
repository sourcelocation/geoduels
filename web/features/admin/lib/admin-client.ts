import { apiFetch, readError } from "../../../lib/http";
import type { ChangelogPost, ChangelogPostInput } from "../../changelog/types";
import type { MaintenanceStatus } from "../../matchmaking/lib/queue-client";

export type AdminBadgeDefinition = {
  id: string;
  kind: string;
  label: string;
  description: string;
  imageUrl: string;
  rarity?: string;
  maxLevel: number;
};

export async function requestAdminBootstrap(
  accessToken: string,
) {
  const resp = await apiFetch(`/api/staff/bootstrap`, {
    method: "POST",
    credentials: "include",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to bootstrap admin access"));
  }
  return resp.json();
}

export async function requestAdminPlayers(
  accessToken: string,
  query: string,
) {
  const resp = await apiFetch(
    `/api/staff/players?query=${encodeURIComponent(query)}`,
    {
      headers: { Authorization: `Bearer ${accessToken}` },
    },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to search players"));
  }
  return resp.json();
}

export async function requestAdminBadgeDefinitions(
  accessToken: string,
) {
  const resp = await apiFetch(`/api/staff/badges`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load grantable badges"));
  }
  return (await resp.json()) as { badges: AdminBadgeDefinition[] };
}

export async function requestAdminGrantBadge(
  accessToken: string,
  payload: { nickname: string; badgeId: string },
) {
  const resp = await apiFetch(`/api/staff/badges/grant`, {
    method: "POST",
    headers: {
      "content-type": "application/json",
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify(payload),
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to grant badge"));
  }
  return resp.json() as Promise<{ badge: AdminBadgeDefinition; changed: boolean }>;
}

export async function requestAdminPlayerDetail(
  accessToken: string,
  userId: string,
) {
  const resp = await apiFetch(
    `/api/staff/players/${encodeURIComponent(userId)}`,
    {
      headers: { Authorization: `Bearer ${accessToken}` },
    },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load player detail"));
  }
  return resp.json();
}

export async function requestAdminBanPlayer(
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
    throw new Error(await readError(resp, "Failed to ban player"));
  }
  return resp.json();
}

export async function requestAdminUnbanPlayer(
  accessToken: string,
  userId: string,
) {
  const resp = await apiFetch(
    `/api/staff/players/${encodeURIComponent(userId)}/unban`,
    {
      method: "POST",
      headers: { Authorization: `Bearer ${accessToken}` },
    },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to unban player"));
  }
}

export async function requestAdminCommunityPardonPreview(accessToken: string) {
  const resp = await apiFetch(`/api/staff/community-pardon`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) throw new Error(await readError(resp, "Failed to preview community pardon"));
  return resp.json() as Promise<{ eligible: number; pardoned: number; cutoff: string }>;
}

export async function requestAdminCommunityPardon(accessToken: string) {
  const resp = await apiFetch(`/api/staff/community-pardon`, {
    method: "POST",
    headers: {
      "content-type": "application/json",
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify({ confirm: true }),
  });
  if (!resp.ok) throw new Error(await readError(resp, "Failed to pardon banned players"));
  return resp.json() as Promise<{ eligible: number; pardoned: number; cutoff: string }>;
}

export async function requestAdminClearReporterMute(
  accessToken: string,
  userId: string,
) {
  const resp = await apiFetch(
    `/api/staff/players/${encodeURIComponent(userId)}/report-mute`,
    {
      method: "DELETE",
      headers: { Authorization: `Bearer ${accessToken}` },
    },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to unmute reporter"));
  }
}

export async function requestAdminPromoteModerator(
  accessToken: string,
  userId: string,
) {
  const resp = await apiFetch(
    `/api/staff/players/${encodeURIComponent(userId)}/roles/judge`,
    {
      method: "POST",
      headers: { Authorization: `Bearer ${accessToken}` },
    },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to promote moderator"));
  }
}

export async function requestAdminDemoteModerator(
  accessToken: string,
  userId: string,
) {
  const resp = await apiFetch(
    `/api/staff/players/${encodeURIComponent(userId)}/roles/judge`,
    {
      method: "DELETE",
      headers: { Authorization: `Bearer ${accessToken}` },
    },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to demote moderator"));
  }
}

export async function requestAdminRoles(
  accessToken: string,
) {
  const resp = await apiFetch(`/api/staff/roles`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load roles"));
  }
  return resp.json();
}

export async function requestAdminGrantRole(
  accessToken: string,
  payload: { userId: string; role: string; reason?: string },
) {
  const resp = await apiFetch(`/api/staff/roles`, {
    method: "POST",
    headers: {
      "content-type": "application/json",
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify(payload),
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to grant role"));
  }
}

export async function requestAdminRevokeRole(
  accessToken: string,
  userId: string,
  role: string,
  reason = "",
) {
  const resp = await apiFetch(
    `/api/staff/roles/${encodeURIComponent(userId)}/${encodeURIComponent(role)}`,
    {
      method: "DELETE",
      headers: {
        "content-type": "application/json",
        Authorization: `Bearer ${accessToken}`,
      },
      body: JSON.stringify({ reason }),
    },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to revoke role"));
  }
}

export async function requestAdminIPSignupBans(
  accessToken: string,
) {
  const resp = await apiFetch(`/api/staff/ip-bans`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load IP bans"));
  }
  return resp.json();
}

export async function requestAdminAddIPSignupBan(
  accessToken: string,
  ipAddress: string,
  reason: string,
) {
  const resp = await apiFetch(`/api/staff/ip-bans`, {
    method: "POST",
    headers: {
      "content-type": "application/json",
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify({ ipAddress, reason }),
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to add IP ban"));
  }
}

export async function requestAdminRemoveIPSignupBan(
  accessToken: string,
  ipAddress: string,
) {
  const resp = await apiFetch(
    `/api/staff/ip-bans/${encodeURIComponent(ipAddress)}`,
    {
      method: "DELETE",
      headers: { Authorization: `Bearer ${accessToken}` },
    },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to remove IP ban"));
  }
}

export async function requestAdminMaintenance(
  accessToken: string,
) {
  const resp = await apiFetch(`/api/staff/maintenance`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load maintenance"));
  }
  return resp.json() as Promise<MaintenanceStatus>;
}

export type AdminModerationSettings = {
  discordWebhookUrl: string;
};

export type AdminDiscordIntegrationSettings = {
  guildId: string;
  joinsChannelId: string;
  elo1000RoleId: string;
  elo1500RoleId: string;
  elo2000RoleId: string;
  managedRoleIds?: string[];
  reconcileIntervalMinutes: number;
};

export type AdminRankedSeasonSettings = {
  activeSeasonId: string;
  monthlyResetDay: number;
  nextResetAt?: string;
  lastResetAt?: string;
};

export async function requestAdminModerationSettings(
  accessToken: string,
) {
  const resp = await apiFetch(`/api/staff/settings/moderation`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(
      await readError(resp, "Failed to load moderation settings"),
    );
  }
  return resp.json() as Promise<AdminModerationSettings>;
}

export async function requestAdminPutModerationSettings(
  accessToken: string,
  settings: AdminModerationSettings,
) {
  const resp = await apiFetch(`/api/staff/settings/moderation`, {
    method: "PUT",
    headers: {
      "content-type": "application/json",
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify(settings),
  });
  if (!resp.ok) {
    throw new Error(
      await readError(resp, "Failed to save moderation settings"),
    );
  }
  return resp.json() as Promise<AdminModerationSettings>;
}

export async function requestAdminDiscordIntegrationSettings(
  accessToken: string,
) {
  const resp = await apiFetch(`/api/staff/settings/discord`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load Discord integration settings"));
  }
  return resp.json() as Promise<AdminDiscordIntegrationSettings>;
}

export async function requestAdminPutDiscordIntegrationSettings(
  accessToken: string,
  settings: AdminDiscordIntegrationSettings,
) {
  const resp = await apiFetch(`/api/staff/settings/discord`, {
    method: "PUT",
    headers: {
      Authorization: `Bearer ${accessToken}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify(settings),
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to save Discord integration settings"));
  }
  return resp.json() as Promise<AdminDiscordIntegrationSettings>;
}

export async function requestAdminRankedSeason(
  accessToken: string,
) {
  const resp = await apiFetch(`/api/staff/seasons`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load season settings"));
  }
  return resp.json() as Promise<AdminRankedSeasonSettings>;
}

export async function requestAdminSetRankedSeasonResetRule(
  accessToken: string,
  monthlyResetDay: number,
) {
  const resp = await apiFetch(`/api/staff/seasons/reset-rule`, {
    method: "PUT",
    headers: {
      "content-type": "application/json",
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify({ monthlyResetDay }),
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to save season reset rule"));
  }
  return resp.json() as Promise<AdminRankedSeasonSettings>;
}

export async function requestAdminPutMaintenance(
  accessToken: string,
  status: MaintenanceStatus,
) {
  const resp = await apiFetch(`/api/staff/maintenance`, {
    method: "PUT",
    headers: {
      "content-type": "application/json",
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify(status),
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to save maintenance"));
  }
  return resp.json() as Promise<MaintenanceStatus>;
}

export async function requestAdminClearMaintenance(
  accessToken: string,
) {
  const resp = await apiFetch(`/api/staff/maintenance`, {
    method: "DELETE",
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to clear maintenance"));
  }
}

export async function requestAdminGetChangelog(
  accessToken: string,
) {
  const resp = await apiFetch(`/api/staff/changelog`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load changelog"));
  }
  return resp.json() as Promise<{ posts: ChangelogPost[] }>;
}

export async function requestAdminCreateChangelogPost(
  accessToken: string,
  content: ChangelogPostInput,
) {
  const resp = await apiFetch(`/api/staff/changelog`, {
    method: "POST",
    headers: {
      "content-type": "application/json",
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify(content),
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to save changelog"));
  }
  return resp.json() as Promise<ChangelogPost>;
}

export async function requestAdminUpdateChangelogPost(
  accessToken: string,
  id: number,
  content: ChangelogPostInput,
) {
  const resp = await apiFetch(`/api/staff/changelog/${encodeURIComponent(String(id))}`, {
    method: "PUT",
    headers: {
      "content-type": "application/json",
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify(content),
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to save changelog"));
  }
  return resp.json() as Promise<ChangelogPost>;
}

export async function requestAdminUploadCurrentMap(
  accessToken: string,
  file: File,
  mapKey = "a-source-world",
) {
  const body = new FormData();
  body.append("file", file);
  const resp = await apiFetch(
    `/api/staff/maps/${encodeURIComponent(mapKey)}/upload`,
    {
      method: "POST",
      headers: { Authorization: `Bearer ${accessToken}` },
      body,
    },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to upload map"));
  }
  return resp.json();
}
