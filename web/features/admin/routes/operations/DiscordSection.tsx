import { useMutation, useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { Button } from "../../../../components/ui/button";
import { Input } from "../../../../components/ui/input";
import { Heading } from "../../../../components/ui/typography";
import { AdminPanel as Panel } from "../../components/admin-primitives";
import {
  requestAdminDiscordIntegrationSettings,
  requestAdminPutDiscordIntegrationSettings,
} from "../../lib/admin-client";
import type { OperationsSectionProps } from "./types";

export function DiscordSection(props: OperationsSectionProps) {
  const [discordSettings, setDiscordSettings] = useState({
    guildId: "",
    joinsChannelId: "",
    elo1000RoleId: "",
    elo1500RoleId: "",
    elo2000RoleId: "",
    reconcileIntervalMinutes: 15,
  });

  const discordSettingsQuery = useQuery({
    queryKey: ["admin-discord-integration-settings", props.accessToken],
    enabled: props.canManageAdmin && !!props.accessToken,
    queryFn: () => requestAdminDiscordIntegrationSettings(props.accessToken),
  });

  useEffect(() => {
    if (!discordSettingsQuery.data) return;
    setDiscordSettings(discordSettingsQuery.data);
  }, [discordSettingsQuery.data]);

  const saveDiscordSettings = useMutation({
    mutationFn: () =>
      requestAdminPutDiscordIntegrationSettings(
        props.accessToken,
        discordSettings,
      ),
    onSuccess: props.refreshAdminData,
  });

  if (props.leaf !== "discord") {
    return null;
  }

  return (
    <Panel className="p-4 xl:col-span-2">
      <Heading as="h3" variant="heading-sm">Discord Integration</Heading>
      <p className="mt-2 text-body-sm text-content-secondary">
        The bot token remains a deployment secret. These IDs refresh in the worker automatically.
      </p>
      <div className="mt-4 grid gap-3 md:grid-cols-2">
        <Input
          value={discordSettings.guildId}
          onChange={(event) => setDiscordSettings((current) => ({ ...current, guildId: event.target.value }))}
          placeholder="Guild ID"
        />
        <Input
          value={discordSettings.joinsChannelId}
          onChange={(event) => setDiscordSettings((current) => ({ ...current, joinsChannelId: event.target.value }))}
          placeholder="#joins channel ID (optional)"
        />
        <Input
          value={discordSettings.elo1000RoleId}
          onChange={(event) => setDiscordSettings((current) => ({ ...current, elo1000RoleId: event.target.value }))}
          placeholder="1k ELO role ID"
        />
        <Input
          value={discordSettings.elo1500RoleId}
          onChange={(event) => setDiscordSettings((current) => ({ ...current, elo1500RoleId: event.target.value }))}
          placeholder="1.5k ELO role ID"
        />
        <Input
          value={discordSettings.elo2000RoleId}
          onChange={(event) => setDiscordSettings((current) => ({ ...current, elo2000RoleId: event.target.value }))}
          placeholder="2k ELO role ID"
        />
        <Input
          type="number"
          min={1}
          max={1440}
          value={discordSettings.reconcileIntervalMinutes}
          onChange={(event) => setDiscordSettings((current) => ({
            ...current,
            reconcileIntervalMinutes: Number(event.target.value),
          }))}
          placeholder="Reconciliation interval (minutes)"
        />
      </div>
      <Button
        className="mt-4"
        disabled={
          saveDiscordSettings.isPending ||
          discordSettings.reconcileIntervalMinutes < 1 ||
          discordSettings.reconcileIntervalMinutes > 1440
        }
        onClick={() => void saveDiscordSettings.mutateAsync()}
      >
        Save Discord Settings
      </Button>
      {saveDiscordSettings.error ? (
        <p className="mt-3 text-body-sm text-status-danger">{saveDiscordSettings.error.message}</p>
      ) : null}
    </Panel>
  );
}
