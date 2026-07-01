import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
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
        <p className="text-xs font-bold uppercase tracking-[0.16em] text-emerald-300">Signals</p>
        <h2 className="mt-1 text-3xl font-black text-white">Signal Stream</h2>
      </header>
      <Panel className="overflow-x-auto">
        <table className="w-full min-w-[960px] text-left text-sm">
          <thead className="border-b border-slate-800 text-xs uppercase tracking-[0.12em] text-slate-500">
            <tr>
              <th className="px-4 py-3">Subject</th>
              <th className="px-4 py-3">Source</th>
              <th className="px-4 py-3">Severity</th>
              <th className="px-4 py-3">Evidence</th>
              <th className="px-4 py-3">Reason</th>
              <th className="px-4 py-3">Match</th>
              <th className="px-4 py-3">When</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-900">
            {signals.map((signal) => (
              <tr key={signal.id}>
                <td className="px-4 py-3">
                  <Link className="font-bold text-white hover:text-emerald-300" href={`/players/${encodeURIComponent(toPublicEntityId(signal.subjectUserId))}?staff=1`}>
                    {signal.subjectName || signal.subjectUserId}
                  </Link>
                  <p className="text-xs text-slate-500">{signal.subjectUserId}</p>
                </td>
                <td className="px-4 py-3 text-slate-300">{signal.source}</td>
                <td className="px-4 py-3 font-semibold text-white">{signal.severity}</td>
                <td className="px-4 py-3 text-slate-300">{signal.evidenceStrength}</td>
                <td className="px-4 py-3 text-slate-400">{signal.reasonCode}</td>
                <td className="px-4 py-3">
                  {signal.matchId ? (
                    <Link className="text-sky-300 hover:text-white" href={`/match/${encodeURIComponent(toPublicEntityId(signal.matchId))}`}>
                      Match
                    </Link>
                  ) : (
                    <span className="text-slate-500">-</span>
                  )}
                </td>
                <td className="px-4 py-3 text-slate-500">{new Date(signal.occurredAt).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {!signalsQuery.isLoading && signals.length === 0 ? <p className="p-4 text-sm text-slate-400">No moderation signals yet.</p> : null}
      </Panel>
    </div>
  );
}
