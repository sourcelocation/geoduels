import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Button } from "../../../../components/ui/button";
import { Input } from "../../../../components/ui/input";
import { Heading } from "../../../../components/ui/typography";
import { AdminPanel as Panel } from "../../components/admin-primitives";
import {
  requestAdminAddIPSignupBan,
  requestAdminIPSignupBans,
  requestAdminRemoveIPSignupBan,
} from "../../lib/admin-client";
import type { IPBan } from "../../types";
import type { OperationsSectionProps } from "./types";

export function IpSignupBlocksSection(props: OperationsSectionProps) {
  const [ipAddress, setIPAddress] = useState("");
  const [ipReason, setIPReason] = useState("");

  const ipBansQuery = useQuery({
    queryKey: ["admin-ip-signup-bans", props.accessToken],
    enabled: props.canManageAdmin && !!props.accessToken,
    queryFn: () => requestAdminIPSignupBans(props.accessToken),
  });

  const addIPBan = useMutation({
    mutationFn: () => requestAdminAddIPSignupBan(props.accessToken, ipAddress, ipReason),
    onSuccess: props.refreshAdminData,
  });
  const removeIPBan = useMutation({
    mutationFn: (ip: string) => requestAdminRemoveIPSignupBan(props.accessToken, ip),
    onSuccess: props.refreshAdminData,
  });

  if (props.leaf !== "ip-signup-blocks") {
    return null;
  }

  const ipBans = (ipBansQuery.data?.bans || []) as IPBan[];

  return (
    <Panel className="p-4">
      <Heading as="h3" variant="heading-sm">IP Signup Blocks</Heading>
      <div className="mt-4 grid gap-2 md:grid-cols-[1fr_1fr_auto]">
        <Input value={ipAddress} onChange={(event) => setIPAddress(event.target.value)} placeholder="IP address" />
        <Input value={ipReason} onChange={(event) => setIPReason(event.target.value)} placeholder="Reason" />
        <Button disabled={!ipAddress} onClick={() => void addIPBan.mutateAsync()}>Block</Button>
      </div>
      <div className="mt-4 space-y-2">
        {ipBans.map((ban) => (
          <div key={ban.id} className="flex items-center justify-between rounded-md border border-border-default bg-surface-grouped p-3">
            <div>
              <p className="font-semibold text-content-primary">{ban.ipAddress}</p>
              <p className="text-body-sm text-content-secondary">{ban.reason || "No reason"}</p>
            </div>
            <Button onClick={() => void removeIPBan.mutateAsync(ban.ipAddress)}>Remove</Button>
          </div>
        ))}
      </div>
    </Panel>
  );
}
