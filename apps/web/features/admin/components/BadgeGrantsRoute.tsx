import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import { Select } from "../../../components/ui/select";
import { Heading, Text } from "../../../components/ui/typography";
import type { RuntimeConfig } from "../../../lib/runtime-config";
import {
  requestAdminBadgeDefinitions,
  requestAdminGrantBadge,
  type AdminBadgeDefinition,
} from "../lib/admin-client";
import { AdminPanel as Panel } from "./admin-primitives";

export function BadgeGrantsRoute(props: {
  config: RuntimeConfig;
  accessToken: string;
  canManageAdmin: boolean;
}) {
  const [nickname, setNickname] = useState("");
  const [badgeId, setBadgeId] = useState("");
  const [success, setSuccess] = useState("");
  const badgesQuery = useQuery({
    queryKey: ["admin-badge-definitions", props.accessToken],
    enabled: props.canManageAdmin && !!props.accessToken,
    queryFn: () => requestAdminBadgeDefinitions(props.config, props.accessToken),
    staleTime: 5 * 60_000,
  });
  const grantMutation = useMutation({
    mutationFn: () => requestAdminGrantBadge(props.config, props.accessToken, { nickname, badgeId }),
    onSuccess: (result) => {
      setSuccess(result.changed
        ? `${result.badge.label} granted to ${nickname.trim()}.`
        : `${result.badge.label} is already owned by ${nickname.trim()}.`);
      setNickname("");
    },
  });
  const badges = (badgesQuery.data?.badges || []) as AdminBadgeDefinition[];
  const selectedBadge = badges.find((badge) => badge.id === badgeId);

  if (!props.canManageAdmin) {
    return <Panel className="p-5 text-body-sm text-content-secondary">Admin access is required to grant badges.</Panel>;
  }

  return (
    <div className="space-y-4">
      <header>
        <Text as="p" variant="label" className="text-status-success">Access</Text>
        <Heading as="h2" variant="display-md" className="mt-1">Badge Grants</Heading>
      </header>
      <Panel className="p-4">
        <p className="text-body-sm text-content-secondary">
          Grant a catalog badge at level 1. System-derived rank, role, and legacy badges are intentionally excluded.
        </p>
        <div className="mt-4 grid gap-3 md:grid-cols-3 md:items-end">
          <label className="grid gap-1 text-body-sm font-semibold text-content-secondary">
            Nickname
            <Input value={nickname} onChange={(event) => setNickname(event.target.value)} placeholder="Player nickname" />
          </label>
          <label className="grid gap-1 text-body-sm font-semibold text-content-secondary">
            Badge
            <Select value={badgeId} onChange={(event) => { setBadgeId(event.target.value); setSuccess(""); }} disabled={badgesQuery.isLoading}>
              <option value="">Select a badge</option>
              {badges.map((badge) => <option key={badge.id} value={badge.id}>{badge.label}</option>)}
            </Select>
          </label>
          <Button
            type="button"
            disabled={!nickname.trim() || !badgeId || grantMutation.isPending}
            onClick={() => { setSuccess(""); void grantMutation.mutateAsync(); }}
          >
            {grantMutation.isPending ? "Granting..." : "Give Badge"}
          </Button>
        </div>
        {selectedBadge ? <p className="mt-3 text-body-sm text-content-secondary">{selectedBadge.description}</p> : null}
        {success ? <p className="mt-3 text-body-sm text-status-success">{success}</p> : null}
        {badgesQuery.error ? <p className="mt-3 text-body-sm text-status-danger">{badgesQuery.error instanceof Error ? badgesQuery.error.message : "Failed to load badges"}</p> : null}
        {grantMutation.error ? <p className="mt-3 text-body-sm text-status-danger">{grantMutation.error instanceof Error ? grantMutation.error.message : "Failed to grant badge"}</p> : null}
      </Panel>
    </div>
  );
}
