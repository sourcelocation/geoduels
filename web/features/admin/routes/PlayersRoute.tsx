import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { ChevronRight } from "lucide-react";
import { useState } from "react";
import { Input } from "../../../components/ui/input";
import { Table, TableHead } from "../../../components/ui/Table";
import { Heading, Text } from "../../../components/ui/typography";
import { toPublicEntityId } from "../../../lib/entity-id";
import { AdminPanel as Panel } from "../components/admin-primitives";
import { requestAdminPlayers } from "../lib/admin-client";
import type { Player } from "../types";

export function PlayersRoute(props: {
  accessToken: string;
  canManageAdmin: boolean;
  basePath?: string;
  title?: string;
  eyebrow?: string;
  searchPlaceholder?: string;
}) {
  const [query, setQuery] = useState("");
  const playersQuery = useQuery({
    queryKey: ["admin-players", query, props.accessToken],
    enabled: !!props.accessToken,
    queryFn: () => requestAdminPlayers(props.accessToken, query),
    staleTime: 5_000,
  });
  const players = (playersQuery.data?.players || []) as Player[];
  const basePath = props.basePath || "/admin/players";

  return (
    <div className="space-y-4">
      <header>
        <Text as="p" variant="label" className="text-status-success">{props.eyebrow || "Players"}</Text>
        <Heading as="h2" variant="display-md" className="mt-1">{props.title || "Player Search"}</Heading>
      </header>
      <Panel className="p-4">
        <div className="flex flex-col gap-3 sm:flex-row">
          <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={props.searchPlaceholder || "Search user ID, name, email, OAuth ID"} className="w-full" />
        </div>
      </Panel>
      <Panel className="overflow-x-auto">
        <Table className="w-full min-w-[840px] text-left text-body-sm">
          <TableHead className="border-b border-border-default text-label uppercase text-content-secondary">
            <tr>
              <th className="px-4 py-3">Player</th>
              <th className="px-4 py-3">MMR</th>
              <th className="px-4 py-3">Record</th>
              <th className="px-4 py-3">Status</th>
              <th className="px-4 py-3 text-right">Open</th>
            </tr>
          </TableHead>
          <tbody className="divide-y divide-border-default">
            {players.map((player) => (
              <tr key={player.userId}>
                <td className="px-4 py-3">
                  <Link className="text-left text-body-sm font-strong text-content-primary hover:text-status-success" href={`${basePath}/${encodeURIComponent(toPublicEntityId(player.userId))}`}>
                    {player.displayName || player.userId}
                  </Link>
                  <p className="mt-1 text-body-sm text-content-secondary">{props.canManageAdmin ? player.email || player.userId : player.userId}</p>
                </td>
                <td className="px-4 py-3">{player.mmr}</td>
                <td className="px-4 py-3 text-body-sm text-content-secondary">{player.wins}W / {player.gamesPlayed}G</td>
                <td className="px-4 py-3">{player.isBanned ? "Banned" : "Active"}</td>
                <td className="px-4 py-3 text-right">
                  <Link className="inline-flex items-center gap-2 rounded-md border border-border-strong px-3 py-2 text-body-sm font-semibold text-content-primary hover:border-status-success hover:text-status-success" href={`${basePath}/${encodeURIComponent(toPublicEntityId(player.userId))}`}>
                    Details
                    <ChevronRight className="h-4 w-4" />
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
        {!players.length ? <p className="p-4 text-body-sm text-content-secondary">No players found.</p> : null}
      </Panel>
    </div>
  );
}
