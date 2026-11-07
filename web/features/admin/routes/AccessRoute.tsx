import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import { Select } from "../../../components/ui/select";
import { Table, TableHead } from "../../../components/ui/Table";
import { Heading, Text } from "../../../components/ui/typography";
import { AdminPanel as Panel } from "../components/admin-primitives";
import { requestAdminGrantRole, requestAdminRevokeRole, requestAdminRoles } from "../lib/admin-client";
import type { UserRoleGrant } from "../types";

export function AccessRoute(props: {
  accessToken: string;
  canManageAdmin: boolean;
  refreshAdminData: () => Promise<void>;
}) {
  const [userId, setUserId] = useState("");
  const [role, setRole] = useState("moderator");
  const [reason, setReason] = useState("");
  const rolesQuery = useQuery({
    queryKey: ["admin-roles", props.accessToken],
    enabled: props.canManageAdmin && !!props.accessToken,
    queryFn: () => requestAdminRoles(props.accessToken),
  });
  const grantRole = useMutation({
    mutationFn: () => requestAdminGrantRole(props.accessToken, { userId, role, reason }),
    onSuccess: props.refreshAdminData,
  });
  const revokeRole = useMutation({
    mutationFn: (grant: UserRoleGrant) => requestAdminRevokeRole(props.accessToken, grant.userId, grant.role, reason),
    onSuccess: props.refreshAdminData,
  });
  const roles = (rolesQuery.data?.roles || []) as UserRoleGrant[];
  return (
    <div className="space-y-4">
      <header>
        <Text as="p" variant="label" className="text-status-success">Access</Text>
        <Heading as="h2" variant="display-md" className="mt-1">Roles</Heading>
      </header>
      {!props.canManageAdmin ? (
        <Panel className="p-5 text-body-sm text-status-warning">Admin access is required to manage roles.</Panel>
      ) : (
        <>
          <Panel className="p-4">
            <div className="grid gap-3 md:grid-cols-[1fr_180px_1fr_auto]">
              <Input value={userId} onChange={(event) => setUserId(event.target.value)} placeholder="User ID" />
              <Select value={role} onChange={(event) => setRole(event.target.value)}>
                <option value="moderator">Moderator</option>
                <option value="judge">Judge</option>
                <option value="lanista">Lanista</option>
                <option value="admin">Admin</option>
              </Select>
              <Input value={reason} onChange={(event) => setReason(event.target.value)} placeholder="Reason" />
              <Button disabled={!userId.trim()} onClick={() => void grantRole.mutateAsync()}>Grant</Button>
            </div>
          </Panel>
          <Panel className="overflow-x-auto">
            <Table className="w-full min-w-[760px] text-left text-body-sm">
              <TableHead className="border-b border-border-default text-label uppercase text-content-secondary">
                <tr>
                  <th className="px-4 py-3">User</th>
                  <th className="px-4 py-3">Role</th>
                  <th className="px-4 py-3">Granted By</th>
                  <th className="px-4 py-3">Reason</th>
                  <th className="px-4 py-3 text-right">Actions</th>
                </tr>
              </TableHead>
              <tbody className="divide-y divide-border-default">
                {roles.map((grant) => (
                  <tr key={`${grant.userId}:${grant.role}`}>
                    <td className="px-4 py-3">
                      <p className="font-strong text-content-primary">{grant.displayName || grant.userId}</p>
                      <p className="text-body-sm text-content-secondary">{grant.email || grant.userId}</p>
                    </td>
                    <td className="px-4 py-3 font-semibold text-content-primary">{grant.role}</td>
                    <td className="px-4 py-3 text-content-secondary">{grant.grantedBy || "system"}</td>
                    <td className="px-4 py-3 text-content-secondary">{grant.reason || "-"}</td>
                    <td className="px-4 py-3 text-right">
                      <Button onClick={() => void revokeRole.mutateAsync(grant)}>Revoke</Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </Table>
          </Panel>
        </>
      )}
    </div>
  );
}
