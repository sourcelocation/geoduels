import { useMutation, useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { Button } from "../../../../components/ui/button";
import { Input } from "../../../../components/ui/input";
import { Heading } from "../../../../components/ui/typography";
import { AdminPanel as Panel } from "../../components/admin-primitives";
import {
  requestAdminModerationSettings,
  requestAdminPutModerationSettings,
} from "../../lib/admin-client";
import type { OperationsSectionProps } from "./types";

export function NotificationsSection(props: OperationsSectionProps) {
  const [webhook, setWebhook] = useState("");

  const settingsQuery = useQuery({
    queryKey: ["admin-moderation-settings", props.accessToken],
    enabled: props.canManageAdmin && !!props.accessToken,
    queryFn: () => requestAdminModerationSettings(props.accessToken),
  });

  useEffect(() => {
    setWebhook(settingsQuery.data?.discordWebhookUrl || "");
  }, [settingsQuery.data?.discordWebhookUrl]);

  const saveSettings = useMutation({
    mutationFn: () => requestAdminPutModerationSettings(props.accessToken, { discordWebhookUrl: webhook }),
    onSuccess: props.refreshAdminData,
  });

  if (props.leaf !== "notifications") {
    return null;
  }

  return (
    <Panel className="p-4">
      <Heading as="h3" variant="heading-sm">Report Notifications</Heading>
      <Input className="mt-4 w-full" type="password" value={webhook} onChange={(event) => setWebhook(event.target.value)} placeholder="Discord webhook URL" />
      <Button className="mt-3" onClick={() => void saveSettings.mutateAsync()}>Save Webhook</Button>
    </Panel>
  );
}
