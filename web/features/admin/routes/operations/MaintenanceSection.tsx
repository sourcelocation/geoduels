import { useMutation, useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { Button } from "../../../../components/ui/button";
import { Input } from "../../../../components/ui/input";
import { Select } from "../../../../components/ui/select";
import { Checkbox } from "../../../../components/ui/Switch";
import { Textarea } from "../../../../components/ui/textarea";
import { Heading } from "../../../../components/ui/typography";
import type { MaintenanceStatus } from "../../../matchmaking/lib/queue-client";
import { AdminPanel as Panel } from "../../components/admin-primitives";
import { fromLocalDateTime, localDateTime } from "../../lib/admin-format";
import {
  requestAdminClearMaintenance,
  requestAdminMaintenance,
  requestAdminPutMaintenance,
} from "../../lib/admin-client";
import type { OperationsSectionProps } from "./types";

export function MaintenanceSection(props: OperationsSectionProps) {
  const [phase, setPhase] = useState<MaintenanceStatus["phase"]>("normal");
  const [message, setMessage] = useState("");
  const [startsAt, setStartsAt] = useState("");
  const [endsAt, setEndsAt] = useState("");
  const [queuePaused, setQueuePaused] = useState(false);
  const [playPaused, setPlayPaused] = useState(false);

  const maintenanceQuery = useQuery({
    queryKey: ["admin-maintenance", props.accessToken],
    enabled: props.canManageAdmin && !!props.accessToken,
    queryFn: () => requestAdminMaintenance(props.accessToken),
  });

  useEffect(() => {
    const status = maintenanceQuery.data;
    if (!status) return;
    setPhase(status.phase || "normal");
    setMessage(status.message || "");
    setStartsAt(localDateTime(status.startsAt));
    setEndsAt(localDateTime(status.endsAt));
    setQueuePaused(!!status.queuePaused);
    setPlayPaused(!!status.playPaused);
  }, [maintenanceQuery.data]);

  const saveMaintenance = useMutation({
    mutationFn: () =>
      requestAdminPutMaintenance(props.accessToken, {
        phase,
        message,
        startsAt: fromLocalDateTime(startsAt) || undefined,
        endsAt: fromLocalDateTime(endsAt) || undefined,
        queuePaused,
        playPaused,
      }),
    onSuccess: props.refreshAdminData,
  });
  const clearMaintenance = useMutation({
    mutationFn: () => requestAdminClearMaintenance(props.accessToken),
    onSuccess: props.refreshAdminData,
  });

  if (props.leaf !== "maintenance" && props.leaf !== "") {
    return null;
  }

  return (
    <Panel className="p-4">
      <Heading as="h3" variant="heading-sm">Maintenance</Heading>
      <div className="mt-4 grid gap-3 md:grid-cols-3">
        <Select value={phase} onChange={(event) => setPhase(event.target.value as MaintenanceStatus["phase"])}>
          <option value="normal">Normal</option>
          <option value="warning">Warning</option>
          <option value="active">Active</option>
        </Select>
        <Input type="datetime-local" value={startsAt} onChange={(event) => setStartsAt(event.target.value)} />
        <Input type="datetime-local" value={endsAt} onChange={(event) => setEndsAt(event.target.value)} />
      </div>
      <Textarea className="mt-3 min-h-24 w-full" value={message} onChange={(event) => setMessage(event.target.value)} placeholder="Maintenance message" />
      <div className="mt-3 grid gap-2 md:grid-cols-2">
        <label className="flex items-center gap-2 text-body-sm text-content-secondary"><Checkbox checked={queuePaused} onChange={(event) => setQueuePaused(event.target.checked)} /> Pause queue</label>
        <label className="flex items-center gap-2 text-body-sm text-content-secondary"><Checkbox checked={playPaused} onChange={(event) => setPlayPaused(event.target.checked)} /> Pause play</label>
      </div>
      <div className="mt-4 flex gap-2">
        <Button onClick={() => void saveMaintenance.mutateAsync()}>Save</Button>
        <Button onClick={() => void clearMaintenance.mutateAsync()}>Clear</Button>
      </div>
    </Panel>
  );
}
