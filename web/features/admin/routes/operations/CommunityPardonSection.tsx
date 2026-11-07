import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Button } from "../../../../components/ui/button";
import { AlertDialog } from "../../../../components/ui/Dialog";
import { Heading } from "../../../../components/ui/typography";
import { AdminPanel as Panel } from "../../components/admin-primitives";
import {
  requestAdminCommunityPardon,
  requestAdminCommunityPardonPreview,
} from "../../lib/admin-client";
import type { OperationsSectionProps } from "./types";

export function CommunityPardonSection(props: OperationsSectionProps) {
  const [pardonDialogOpen, setPardonDialogOpen] = useState(false);
  const [pardonResult, setPardonResult] = useState<{ eligible: number; pardoned: number } | null>(null);

  const pardonQuery = useQuery({
    queryKey: ["admin-community-pardon", props.accessToken],
    enabled: props.canManageAdmin && !!props.accessToken,
    queryFn: () => requestAdminCommunityPardonPreview(props.accessToken),
  });
  const pardonMutation = useMutation({
    mutationFn: () => requestAdminCommunityPardon(props.accessToken),
    onSuccess: async (result) => {
      setPardonDialogOpen(false);
      setPardonResult(result);
      await props.refreshAdminData();
      await pardonQuery.refetch();
    },
  });

  const visible = props.leaf === "maintenance" || props.leaf === "";

  return (
    <>
      {visible ? (
        <Panel className="p-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <Heading as="h3" variant="heading-sm">v2 Community Pardon</Heading>
              <p className="mt-2 max-w-2xl text-body-sm text-content-secondary">
                Unban every currently banned player whose latest ban is more than seven days old. This includes cheating bans and preserves the moderation history.
              </p>
              <p className="mt-2 text-body-sm text-content-secondary">
                Eligible now: <span className="font-semibold text-content-primary">{pardonQuery.data?.eligible ?? "…"}</span>
              </p>
              {pardonResult ? <p className="mt-2 text-body-sm text-status-success">Pardoned {pardonResult.pardoned} player(s).</p> : null}
              {pardonMutation.isError ? <p className="mt-2 text-body-sm text-status-danger">{pardonMutation.error instanceof Error ? pardonMutation.error.message : "Pardon failed."}</p> : null}
            </div>
            <Button variant="danger" type="button" disabled={!pardonQuery.data?.eligible || pardonMutation.isPending} onClick={() => setPardonDialogOpen(true)}>
              Pardon banned players
            </Button>
          </div>
        </Panel>
      ) : null}
      {pardonDialogOpen ? (
        <AlertDialog
          title="Pardon banned players?"
          description={`This will unban ${pardonQuery.data?.eligible ?? 0} player(s) whose active ban is older than seven days, including cheating bans. This cannot be automatically undone.`}
          onClose={() => setPardonDialogOpen(false)}
          placement="center"
        >
          <div className="flex justify-end gap-2">
            <Button type="button" variant="secondary" onClick={() => setPardonDialogOpen(false)}>Cancel</Button>
            <Button type="button" variant="danger" loading={pardonMutation.isPending} loadingLabel="Pardoning" onClick={() => void pardonMutation.mutateAsync()}>Confirm pardon</Button>
          </div>
        </AlertDialog>
      ) : null}
    </>
  );
}
