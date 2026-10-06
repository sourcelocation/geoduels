import Link from "next/link";
import { useMutation, useQuery } from "@tanstack/react-query";
import { ArrowLeft, Ban, ExternalLink, ShieldAlert } from "lucide-react";
import { useState } from "react";
import { PlayerWarnings } from "../components/PlayerWarnings";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import { CenteredSpinner } from "../../../components/ui/Spinner";
import { Table, TableHead } from "../../../components/ui/Table";
import { Heading, Text } from "../../../components/ui/typography";
import { useRuntimeConfig } from "../../../lib/runtime-config-context";
import { AdminDetailRow as DetailRow, AdminMetric as Metric, AdminPanel as Panel } from "../components/admin-primitives";
import { formatDate } from "../lib/admin-format";
import { requestAdminBanPlayer, requestAdminPlayerDetail, requestAdminUnbanPlayer } from "../lib/admin-client";
import { requestModeratorSubject, requestModeratorSubjectMute, requestModeratorSubjectUnban } from "../lib/moderator-client";
import type { ModerationSubjectProfile, PlayerDetail } from "../types";

export function PlayerDetailRoute(props: {
  config: ReturnType<typeof useRuntimeConfig>;
  accessToken: string;
  userId: string;
  canManageAdmin: boolean;
  basePath?: string;
  titleEyebrow?: string;
  refreshAdminData: () => Promise<void>;
}) {
  const [banReason, setBanReason] = useState("");
  const moderatorSubject = (props.basePath || "").startsWith("/admin/judge");
  const detailQuery = useQuery({
    queryKey: [moderatorSubject ? "moderator-subject" : "admin-player-detail", props.userId, props.accessToken],
    enabled: !!props.accessToken && !!props.userId,
    queryFn: () =>
      moderatorSubject
        ? requestModeratorSubject(props.config, props.accessToken, props.userId)
        : requestAdminPlayerDetail(props.config, props.accessToken, props.userId),
    refetchOnMount: false,
    staleTime: 30_000,
  });
  const banMutation = useMutation({
    mutationFn: () => requestAdminBanPlayer(props.config, props.accessToken, props.userId, banReason),
    onSuccess: props.refreshAdminData,
  });
  const unbanMutation = useMutation({
    mutationFn: () =>
      moderatorSubject
        ? requestModeratorSubjectUnban(props.config, props.accessToken, props.userId, banReason)
        : requestAdminUnbanPlayer(props.config, props.accessToken, props.userId),
    onSuccess: props.refreshAdminData,
  });
  const muteMutation = useMutation({
	mutationFn: ({ kind, muted }: { kind: "chat" | "report"; muted: boolean }) =>
	  requestModeratorSubjectMute(props.config, props.accessToken, props.userId, kind, banReason, muted),
	onSuccess: props.refreshAdminData,
  });
  const detail = detailQuery.data as (PlayerDetail & Partial<ModerationSubjectProfile>) | undefined;
  const player = detail?.player;
  const winRate = player?.gamesPlayed ? Math.round((player.wins / player.gamesPlayed) * 100) : 0;
	const chatMuted = !!player?.chatMutedAt && (!player.chatMutedUntil || new Date(player.chatMutedUntil).getTime() > Date.now());
	const reportMuted = !!player?.reportMutedAt && (!player.reportMutedUntil || new Date(player.reportMutedUntil).getTime() > Date.now());
  const basePath = props.basePath || "/admin/players";

  if (detailQuery.isLoading) {
    return <Panel><CenteredSpinner label="Loading player details" /></Panel>;
  }
  if (!player) {
    return <Panel className="p-5 text-body-sm text-content-secondary">Player detail unavailable.</Panel>;
  }

  return (
    <div className="space-y-4">
      <header className="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <Link href={basePath} className="inline-flex items-center gap-2 text-body-sm font-semibold text-content-secondary hover:text-content-primary">
            <ArrowLeft className="h-4 w-4" />
            Subjects
          </Link>
          <div className="mt-4 flex items-center gap-4">
            <div className="grid h-16 w-16 place-items-center rounded-md border border-border-strong bg-surface-panel text-heading-lg font-strong text-status-success">
              {(player.displayName || player.userId || "?").slice(0, 1).toUpperCase()}
            </div>
            <div>
              <Text as="p" variant="label" className="text-status-success">{props.titleEyebrow || "Player Detail"}</Text>
              <Heading as="h2" variant="display-md" className="mt-1 break-all">{player.displayName || player.userId}</Heading>
              <p className="mt-1 break-all text-body-sm text-content-secondary">{props.canManageAdmin ? player.email || player.userId : player.userId}</p>
            </div>
          </div>
        </div>
        <div className="flex flex-col gap-3 sm:flex-row">
          <Link href={`/players/${encodeURIComponent(player.displayName || player.userId)}`} className="inline-flex items-center justify-center gap-2 rounded-md border border-border-strong px-4 py-2 text-body-sm font-semibold text-status-info hover:border-status-info hover:text-content-primary">
            Public profile
            <ExternalLink className="h-4 w-4" />
          </Link>
          <Input value={banReason} onChange={(event) => setBanReason(event.target.value)} placeholder="Enforcement reason" className="w-full sm:w-80" />
          {player.isBanned ? (
            <Button onClick={() => void unbanMutation.mutateAsync()}>Unban</Button>
          ) : (
            <Button variant="danger" onClick={() => void banMutation.mutateAsync()}>
              <Ban className="h-4 w-4" />
              Ban
            </Button>
          )}
        </div>
      </header>

      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
        <Metric label="MMR" value={`${player.mmr}`} />
        <Metric label="Win Rate" value={`${winRate}%`} />
        <Metric label="Total Games" value={`${player.gamesPlayed}`} />
        <Metric label="Ranked Games" value={`${player.rankedGamesPlayed}`} />
        <Metric label="Status" value={player.isBanned ? "Banned" : "Active"} />
      </div>

      <div className="grid gap-4 xl:grid-cols-2">
        <Panel className="p-4">
          <div className="flex items-center gap-2">
            <ShieldAlert className={`h-5 w-5 ${player.isBanned ? "text-status-danger" : "text-status-success"}`} />
            <Heading as="h3" variant="heading-sm">Account Signals</Heading>
          </div>
          <div className="mt-4 space-y-3 text-body-sm">
            <DetailRow label="User ID" value={player.userId} />
            <DetailRow label="Account" value={player.isGuest ? "Guest" : "Registered"} />
            <DetailRow label="Role" value={player.isAdmin ? "Admin" : player.isModerator ? "Moderator" : "Player"} />
            <DetailRow label="Ban" value={player.isBanned ? `${player.banReason || "No reason"}${player.banExpiresAt ? ` · until ${formatDate(player.banExpiresAt)}` : " · permanent"}` : "None"} />
            <DetailRow label="Chat Mute" value={chatMuted ? `${player.chatMuteReason || "No reason"}${player.chatMutedUntil ? ` · until ${formatDate(player.chatMutedUntil)}` : " · permanent"}` : "None"} />
            <DetailRow label="Report Mute" value={reportMuted ? `${player.reportMuteReason || "No reason"}${player.reportMutedUntil ? ` · until ${formatDate(player.reportMutedUntil)}` : " · permanent"}` : "None"} />
            {props.canManageAdmin ? <DetailRow label="Last IP" value={player.lastIpAddress || "Unknown"} /> : null}
          </div>
		  {moderatorSubject ? (
			<div className="mt-4 flex flex-wrap gap-2">
			  <Button onClick={() => void muteMutation.mutateAsync({ kind: "chat", muted: !chatMuted })}>{chatMuted ? "Unmute chat" : "Mute chat 7d"}</Button>
			  <Button onClick={() => void muteMutation.mutateAsync({ kind: "report", muted: !reportMuted })}>{reportMuted ? "Unmute reports" : "Mute reports 7d"}</Button>
			</div>
		  ) : null}
        </Panel>
      </div>

      <div className="grid gap-4">
        <Panel className="p-4">
          <Heading as="h3" variant="heading-sm">Stats</Heading>
          <div className="mt-4 grid gap-3 sm:grid-cols-2">
			<Metric label="Tracked Matches" value={`${player.trackedMatches || player.gamesPlayed}`} />
			<Metric label="Ranked Matches" value={`${player.rankedMatches || player.rankedGamesPlayed}`} />
			<Metric label="Duels" value={`${player.duelMatches || 0}`} />
			<Metric label="Singleplayer" value={`${player.singleplayerRuns || 0}`} />
			<Metric label="Wins" value={`${player.wins}`} />
			<Metric label="Losses" value={`${player.losses || 0}`} />
          </div>
        </Panel>
      </div>

      <PlayerWarnings config={props.config} accessToken={props.accessToken} userId={props.userId}
        onChanged={async () => { await detailQuery.refetch(); await props.refreshAdminData(); }} />

      {moderatorSubject ? (
        <div className="grid gap-4 xl:grid-cols-2">
          <Panel className="p-4">
            <Heading as="h3" variant="heading-sm">Moderator Log</Heading>
            <div className="mt-3 space-y-2">
              {(detail.log || []).map((entry) => (
                <div key={entry.id} className="rounded-md border border-border-default bg-surface-grouped p-3 text-body-sm">
                  <p className="font-semibold text-content-primary">{entry.action}</p>
                  <p className="mt-1 text-content-secondary">{entry.reason || "No reason"} · {entry.actorName || entry.actorUserId || "system"}</p>
                  <p className="mt-1 text-body-sm text-content-secondary">{formatDate(entry.createdAt)}</p>
                </div>
              ))}
              {!detail.log?.length ? <p className="text-body-sm text-content-secondary">No moderator actions for this subject.</p> : null}
            </div>
          </Panel>
          <Panel className="p-4">
            <Heading as="h3" variant="heading-sm">Recent Signals</Heading>
            <div className="mt-3 space-y-2">
              {(detail.signals || []).map((signal) => (
                <div key={signal.id} className="rounded-md border border-border-default bg-surface-grouped p-3 text-body-sm">
                  <p className="font-semibold text-content-primary">{signal.reasonCode}</p>
                  <p className="mt-1 text-content-secondary">{signal.source} · {signal.severity} / {signal.evidenceStrength}</p>
                </div>
              ))}
              {!detail.signals?.length ? <p className="text-body-sm text-content-secondary">No moderation signals for this subject.</p> : null}
            </div>
          </Panel>
        </div>
      ) : null}

      {props.canManageAdmin ? (
        <Panel className="p-4">
          <Heading as="h3" variant="heading-sm">Linked Identity History</Heading>
          <div className="mt-3 overflow-x-auto">
            <Table className="w-full min-w-[760px] text-left text-body-sm">
              <TableHead className="border-b border-border-default text-label uppercase text-content-secondary">
                <tr>
                  <th className="px-3 py-2">Provider</th>
                  <th className="px-3 py-2">Provider User</th>
                  <th className="px-3 py-2">Email</th>
                  <th className="px-3 py-2">Name</th>
                  <th className="px-3 py-2">Last Seen</th>
                </tr>
              </TableHead>
              <tbody className="divide-y divide-border-default">
                {(player.identities || []).map((identity) => (
                  <tr key={`${identity.provider}:${identity.providerUserId}:${identity.lastSeenAt || ""}`}>
                    <td className="px-3 py-2 text-content-primary">{identity.provider}</td>
                    <td className="px-3 py-2 text-content-secondary">{identity.providerUserId}</td>
                    <td className="px-3 py-2 text-content-secondary">{identity.email || "None"}</td>
                    <td className="px-3 py-2 text-content-secondary">{identity.providerName || "None"}</td>
                    <td className="px-3 py-2 text-content-secondary">{formatDate(identity.lastSeenAt)}</td>
                  </tr>
                ))}
              </tbody>
            </Table>
            {!player.identities?.length ? <p className="mt-3 text-body-sm text-content-secondary">No linked identity history.</p> : null}
          </div>
        </Panel>
      ) : null}
    </div>
  );
}
