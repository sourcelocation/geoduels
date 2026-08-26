import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { Table, TableHead } from "../../../components/ui/Table";
import { Heading, Text } from "../../../components/ui/typography";
import { toPublicEntityId } from "../../../lib/entity-id";
import type { RuntimeConfig } from "../../../lib/runtime-config";
import { requestModeratorSignals } from "../lib/moderator-client";
import type { ModerationSignal } from "../types";
import { AdminPanel as Panel } from "./admin-primitives";

export function SignalsRoute(props: { config: RuntimeConfig; accessToken: string }) {
  const signalsQuery = useQuery({
    queryKey: ["moderator-signals", props.accessToken],
    enabled: !!props.accessToken,
    queryFn: () => requestModeratorSignals(props.config, props.accessToken),
    staleTime: 5_000,
  });
  const signals = (signalsQuery.data?.signals || []) as ModerationSignal[];
  return (
    <div className="space-y-4">
      <header>
        <Text as="p" variant="label" className="text-status-success">Signals</Text>
        <Heading as="h2" variant="display-md" className="mt-1">Signal Stream</Heading>
      </header>
      <Panel className="overflow-x-auto">
        <Table className="w-full min-w-[960px] text-left text-body-sm">
          <TableHead className="border-b border-border-default text-label uppercase text-content-secondary">
            <tr>
              <th className="px-4 py-3">Subject</th>
              <th className="px-4 py-3">Source</th>
              <th className="px-4 py-3">Severity</th>
              <th className="px-4 py-3">Evidence</th>
              <th className="px-4 py-3">Reason</th>
              <th className="px-4 py-3">Match</th>
              <th className="px-4 py-3">When</th>
            </tr>
          </TableHead>
          <tbody className="divide-y divide-border-default">
            {signals.map((signal) => (
              <tr key={signal.id}>
                <td className="px-4 py-3">
				  <Link className="font-strong text-content-primary hover:text-status-success" href={`/moderator/subjects/${encodeURIComponent(toPublicEntityId(signal.subjectUserId))}`}>
                    {signal.subjectName || signal.subjectUserId}
                  </Link>
                  <p className="text-body-sm text-content-secondary">{signal.subjectUserId}</p>
                </td>
                <td className="px-4 py-3 text-content-secondary">{signal.source}</td>
                <td className="px-4 py-3 font-semibold text-content-primary">{signal.severity}</td>
                <td className="px-4 py-3 text-content-secondary">{signal.evidenceStrength}</td>
                <td className="px-4 py-3 text-content-secondary">{signal.reasonCode}</td>
                <td className="px-4 py-3">
                  {signal.matchId ? (
                    <Link className="text-status-info hover:text-content-primary" href={`/match/${encodeURIComponent(toPublicEntityId(signal.matchId))}`}>
                      Match
                    </Link>
                  ) : (
                    <span className="text-content-secondary">-</span>
                  )}
                </td>
                <td className="px-4 py-3 text-content-secondary">{new Date(signal.occurredAt).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </Table>
        {!signalsQuery.isLoading && signals.length === 0 ? <p className="p-4 text-body-sm text-content-secondary">No moderation signals yet.</p> : null}
      </Panel>
    </div>
  );
}
