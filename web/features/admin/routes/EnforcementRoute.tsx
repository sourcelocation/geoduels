import { useQuery } from "@tanstack/react-query";
import { Table, TableHead } from "../../../components/ui/Table";
import { Heading, Text } from "../../../components/ui/typography";
import { AdminPanel as Panel } from "../components/admin-primitives";
import { formatDate } from "../lib/admin-format";
import { requestModeratorLog } from "../lib/moderator-client";
import type { ModerationTimelineItem } from "../types";

export function EnforcementRoute(props: {
  accessToken: string;
  canViewEnforcement: boolean;
}) {
  const actionsQuery = useQuery({
	queryKey: ["moderator-log", props.accessToken],
	enabled: props.canViewEnforcement && !!props.accessToken,
	queryFn: () => requestModeratorLog(props.accessToken),
  });
  if (!props.canViewEnforcement) {
    return <Panel className="p-5 text-body-sm text-content-secondary">Moderator access is required for the moderation log.</Panel>;
  }
  const actions = (actionsQuery.data?.log || []) as ModerationTimelineItem[];
  return (
    <div className="space-y-4">
      <header>
		<Text as="p" variant="label" className="text-status-success">Moderation</Text>
		<Heading as="h2" variant="display-md" className="mt-1">Moderator Log</Heading>
      </header>
      <Panel className="overflow-x-auto">
        <Table className="w-full min-w-[900px] text-left text-body-sm">
          <TableHead className="border-b border-border-default text-label uppercase text-content-secondary">
            <tr>
			  <th className="px-4 py-3">Subject</th>
              <th className="px-4 py-3">Action</th>
              <th className="px-4 py-3">Actor</th>
			  <th className="px-4 py-3">Expires</th>
			  <th className="px-4 py-3">Reason</th>
              <th className="px-4 py-3">Created</th>
            </tr>
          </TableHead>
          <tbody className="divide-y divide-border-default">
            {actions.map((action) => (
              <tr key={action.id}>
                <td className="px-4 py-3">
				  <p className="font-strong text-content-primary">{action.subjectName || action.subjectUserId || "Deleted user"}</p>
			  <p className="text-body-sm text-content-secondary">{action.subjectUserId}</p>
                </td>
				<td className="px-4 py-3 font-semibold text-content-primary">{action.action}</td>
				<td className="px-4 py-3 text-content-secondary">{action.actorName || action.actorUserId || "system"}</td>
				<td className="px-4 py-3 text-content-secondary">{action.expiresAt ? formatDate(action.expiresAt) : "-"}</td>
				<td className="px-4 py-3 text-content-secondary">{action.reason || "-"}</td>
                <td className="px-4 py-3 text-content-secondary">{new Date(action.createdAt).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </Table>
		{!actionsQuery.isLoading && actions.length === 0 ? <p className="p-4 text-body-sm text-content-secondary">No moderator actions yet.</p> : null}
      </Panel>
    </div>
  );
}
