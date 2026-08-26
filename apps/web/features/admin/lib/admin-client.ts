import { apiFetch, readError } from "../../../lib/http";
import type { RuntimeConfig } from "../../../lib/runtime-config";
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
  config: RuntimeConfig,
  accessToken: string,
) {
  const resp = await apiFetch(config, `/v1/admin/bootstrap`, {
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
  config: RuntimeConfig,
  accessToken: string,
  query: string,
) {
  const resp = await apiFetch(
    config,
    `/v1/admin/players?query=${encodeURIComponent(query)}`,
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
  config: RuntimeConfig,
  accessToken: string,
) {
  const resp = await apiFetch(config, `/v1/admin/badges`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load grantable badges"));
  }
  return (await resp.json()) as { badges: AdminBadgeDefinition[] };
}

export async function requestAdminGrantBadge(
  config: RuntimeConfig,
  accessToken: string,
  payload: { nickname: string; badgeId: string },
) {
  const resp = await apiFetch(config, `/v1/admin/badges/grant`, {
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
  config: RuntimeConfig,
  accessToken: string,
  userId: string,
) {
  const resp = await apiFetch(
    config,
    `/v1/admin/players/${encodeURIComponent(userId)}`,
    {
      headers: { Authorization: `Bearer ${accessToken}` },
    },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load player detail"));
  }
  return resp.json();
}

export async function requestAdminPlayerMatches(
  config: RuntimeConfig,
  accessToken: string,
  userId: string,
) {
  const resp = await apiFetch(
    config,
    `/v1/admin/players/${encodeURIComponent(userId)}/matches`,
    { headers: { Authorization: `Bearer ${accessToken}` } },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load match history"));
  }
  return resp.json();
}

export async function requestAdminBanPlayer(
  config: RuntimeConfig,
  accessToken: string,
  userId: string,
  reason: string,
) {
  const resp = await apiFetch(
    config,
    `/v1/admin/players/${encodeURIComponent(userId)}/ban`,
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
  config: RuntimeConfig,
  accessToken: string,
  userId: string,
) {
  const resp = await apiFetch(
    config,
    `/v1/admin/players/${encodeURIComponent(userId)}/unban`,
    {
      method: "POST",
      headers: { Authorization: `Bearer ${accessToken}` },
    },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to unban player"));
  }
}

export async function requestAdminCommunityPardonPreview(config: RuntimeConfig, accessToken: string) {
  const resp = await apiFetch(config, `/v1/admin/moderation/community-pardon`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) throw new Error(await readError(resp, "Failed to preview community pardon"));
  return resp.json() as Promise<{ eligible: number; pardoned: number; cutoff: string }>;
}

export async function requestAdminCommunityPardon(config: RuntimeConfig, accessToken: string) {
  const resp = await apiFetch(config, `/v1/admin/moderation/community-pardon`, {
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
  config: RuntimeConfig,
  accessToken: string,
  userId: string,
) {
  const resp = await apiFetch(
    config,
    `/v1/admin/players/${encodeURIComponent(userId)}/report-mute`,
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
  config: RuntimeConfig,
  accessToken: string,
  userId: string,
) {
  const resp = await apiFetch(
    config,
    `/v1/admin/players/${encodeURIComponent(userId)}/moderator`,
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
  config: RuntimeConfig,
  accessToken: string,
  userId: string,
) {
  const resp = await apiFetch(
    config,
    `/v1/admin/players/${encodeURIComponent(userId)}/moderator`,
    {
      method: "DELETE",
      headers: { Authorization: `Bearer ${accessToken}` },
    },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to demote moderator"));
  }
}

export async function requestAdminMatchChat(
  config: RuntimeConfig,
  accessToken: string,
  matchId: string,
) {
  const resp = await apiFetch(
    config,
    `/v1/admin/matches/${encodeURIComponent(matchId)}/chat`,
    { headers: { Authorization: `Bearer ${accessToken}` } },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load match chat"));
  }
  return resp.json();
}

export async function requestAdminRoles(
  config: RuntimeConfig,
  accessToken: string,
) {
  const resp = await apiFetch(config, `/v1/admin/roles`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load roles"));
  }
  return resp.json();
}

export async function requestAdminGrantRole(
  config: RuntimeConfig,
  accessToken: string,
  payload: { userId: string; role: string; reason?: string },
) {
  const resp = await apiFetch(config, `/v1/admin/roles`, {
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
  config: RuntimeConfig,
  accessToken: string,
  userId: string,
  role: string,
  reason = "",
) {
  const resp = await apiFetch(
    config,
    `/v1/admin/roles/${encodeURIComponent(userId)}/${encodeURIComponent(role)}`,
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
  config: RuntimeConfig,
  accessToken: string,
) {
  const resp = await apiFetch(config, `/v1/admin/ip-signup-bans`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load IP bans"));
  }
  return resp.json();
}

export async function requestAdminAddIPSignupBan(
  config: RuntimeConfig,
  accessToken: string,
  ipAddress: string,
  reason: string,
) {
  const resp = await apiFetch(config, `/v1/admin/ip-signup-bans`, {
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
  config: RuntimeConfig,
  accessToken: string,
  ipAddress: string,
) {
  const resp = await apiFetch(
    config,
    `/v1/admin/ip-signup-bans/${encodeURIComponent(ipAddress)}`,
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
  config: RuntimeConfig,
  accessToken: string,
) {
  const resp = await apiFetch(config, `/v1/admin/maintenance`, {
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
  config: RuntimeConfig,
  accessToken: string,
) {
  const resp = await apiFetch(config, `/v1/admin/moderation/settings`, {
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
  config: RuntimeConfig,
  accessToken: string,
  settings: AdminModerationSettings,
) {
  const resp = await apiFetch(config, `/v1/admin/moderation/settings`, {
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
  config: RuntimeConfig,
  accessToken: string,
) {
  const resp = await apiFetch(config, `/v1/admin/integrations/discord`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load Discord integration settings"));
  }
  return resp.json() as Promise<AdminDiscordIntegrationSettings>;
}

export async function requestAdminPutDiscordIntegrationSettings(
  config: RuntimeConfig,
  accessToken: string,
  settings: AdminDiscordIntegrationSettings,
) {
  const resp = await apiFetch(config, `/v1/admin/integrations/discord`, {
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
  config: RuntimeConfig,
  accessToken: string,
) {
  const resp = await apiFetch(config, `/v1/admin/seasons`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load season settings"));
  }
  return resp.json() as Promise<AdminRankedSeasonSettings>;
}

export async function requestAdminSetRankedSeasonResetRule(
  config: RuntimeConfig,
  accessToken: string,
  monthlyResetDay: number,
) {
  const resp = await apiFetch(config, `/v1/admin/seasons/reset-rule`, {
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
  config: RuntimeConfig,
  accessToken: string,
  status: MaintenanceStatus,
) {
  const resp = await apiFetch(config, `/v1/admin/maintenance`, {
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
  config: RuntimeConfig,
  accessToken: string,
) {
  const resp = await apiFetch(config, `/v1/admin/maintenance`, {
    method: "DELETE",
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to clear maintenance"));
  }
}

export async function requestAdminGetChangelog(
  config: RuntimeConfig,
  accessToken: string,
) {
  const resp = await apiFetch(config, `/v1/admin/changelog`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to load changelog"));
  }
  return resp.json() as Promise<{ posts: ChangelogPost[] }>;
}

export async function requestAdminCreateChangelogPost(
  config: RuntimeConfig,
  accessToken: string,
  content: ChangelogPostInput,
) {
  const resp = await apiFetch(config, `/v1/admin/changelog`, {
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
  config: RuntimeConfig,
  accessToken: string,
  id: number,
  content: ChangelogPostInput,
) {
  const resp = await apiFetch(config, `/v1/admin/changelog/${encodeURIComponent(String(id))}`, {
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
  config: RuntimeConfig,
  accessToken: string,
  file: File,
  mapKey = "a-source-world",
) {
  const body = new FormData();
  body.append("file", file);
  const resp = await apiFetch(
    config,
    `/v1/admin/maps/${encodeURIComponent(mapKey)}/upload`,
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
