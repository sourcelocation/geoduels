import Head from "next/head";
import dynamic from "next/dynamic";
import Link from "next/link";
import { useRouter } from "next/router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, Ban, ChevronRight, ExternalLink, ShieldAlert } from "lucide-react";
import { useEffect, useState } from "react";
import { Button } from "../../components/ui/button";
import { AlertDialog } from "../../components/ui/Dialog";
import { Badge } from "../../components/ui/Badge";
import { Input } from "../../components/ui/input";
import { Select } from "../../components/ui/select";
import { Textarea } from "../../components/ui/textarea";
import { Checkbox } from "../../components/ui/Switch";
import { CenteredSpinner } from "../../components/ui/Spinner";
import { Table, TableHead } from "../../components/ui/Table";
import { Heading, Text } from "../../components/ui/typography";
import { toPublicEntityId } from "../../lib/entity-id";
import { AdminDetailRow as DetailRow, AdminMetric as Metric, AdminPanel as Panel } from "./components/admin-primitives";
import { SignalsRoute } from "./components/SignalsRoute";
import { BadgeGrantsRoute } from "./components/BadgeGrantsRoute";
import { formatAdminDate, fromLocalDateTime, localDateTime, sanitizeSlugInput, slugify } from "./lib/admin-format";
import {
  requestAdminAddIPSignupBan,
  requestAdminBanPlayer,
  requestAdminCommunityPardon,
  requestAdminCommunityPardonPreview,
  requestAdminClearMaintenance,
  requestAdminDiscordIntegrationSettings,
  requestAdminGrantRole,
  requestAdminCreateChangelogPost,
  requestAdminGetChangelog,
  requestAdminIPSignupBans,
  requestAdminMaintenance,
  requestAdminModerationSettings,
  requestAdminPlayerDetail,
  requestAdminPlayers,
  requestAdminPutMaintenance,
  requestAdminPutDiscordIntegrationSettings,
  requestAdminPutModerationSettings,
  requestAdminRankedSeason,
  requestAdminRevokeRole,
  requestAdminRoles,
  requestAdminRemoveIPSignupBan,
  requestAdminSetRankedSeasonResetRule,
  requestAdminUnbanPlayer,
  requestAdminUpdateChangelogPost,
} from "./lib/admin-client";
import {
  requestModeratorLog,
  requestModeratorSubjectMute,
  requestModeratorSubjectUnban,
  requestModeratorSubject,
} from "./lib/moderator-client";
import { adminNav, moderatorNav, moderatorPathFromRouter, pathFromRouter } from "./lib/admin-navigation";
import type { IPBan, ModerationSubjectProfile, ModerationTimelineItem, Player, PlayerDetail, UserRoleGrant } from "./types";
import type { ChangelogPost, ChangelogPostInput } from "../changelog/types";
import { useHomeModel } from "../home/model/useHomeModel";
import type { MaintenanceStatus } from "../matchmaking/lib/queue-client";
import { getRuntimeConfig } from "../../lib/runtime-config";

const SimpleMDE = dynamic(() => import("react-simplemde-editor"), {
  ssr: false,
});

const changelogMarkdownOptions = {
  autofocus: false,
  spellChecker: false,
  status: false,
  minHeight: "460px",
  previewClass: ["editor-preview", "markdown-content"],
};

type AdminSurface = "admin" | "moderator";

export default function AdminPage({ surface = "admin" }: { surface?: AdminSurface }) {
  const config = getRuntimeConfig();
  const router = useRouter();
  const queryClient = useQueryClient();
  const { view } = useHomeModel({ routeContext: "home", backgroundDataEnabled: false });
  const moderatorSurface = surface === "moderator";
  const path = moderatorSurface ? moderatorPathFromRouter(router) : pathFromRouter(router);
  const section = path[0] || (moderatorSurface ? "subjects" : "operations");
  const leaf = path[1] || "";
  const accessToken = view.auth.accessToken;
  const canViewReports = !!view.auth.isAdmin || !!view.auth.isModerator;
  const canManageAdmin = !!view.auth.isAdmin;
  const hasSurfaceAccess = moderatorSurface ? canViewReports : canManageAdmin;
  const navGroups = moderatorSurface ? moderatorNav : adminNav;
  const consoleTitle = moderatorSurface ? "Moderator Workbench" : "Admin Console";
  const consoleEyebrow = moderatorSurface ? "Review" : "Operations";

  useEffect(() => {
    if (router.pathname === "/admin" && router.isReady) {
      void router.replace("/admin/operations/maintenance");
    }
    if (router.pathname === "/moderator" && router.isReady) {
      void router.replace("/moderator/subjects");
    }
  }, [leaf, moderatorSurface, router, section]);

  const refreshAdminData = async () => {
    await Promise.all([
	  queryClient.invalidateQueries({ queryKey: ["admin-players"] }), queryClient.invalidateQueries({ queryKey: ["moderator-signals"] }),
      queryClient.invalidateQueries({ queryKey: ["moderator-subject"] }), queryClient.invalidateQueries({ queryKey: ["admin-player-matches"] }),
      queryClient.invalidateQueries({ queryKey: ["admin-player-detail"] }), queryClient.invalidateQueries({ queryKey: ["admin-ip-signup-bans"] }),
      queryClient.invalidateQueries({ queryKey: ["admin-changelog"] }), queryClient.invalidateQueries({ queryKey: ["admin-maintenance"] }),
      queryClient.invalidateQueries({ queryKey: ["admin-moderation-settings"] }), queryClient.invalidateQueries({ queryKey: ["admin-discord-integration-settings"] }),
      queryClient.invalidateQueries({ queryKey: ["admin-ranked-season"] }), queryClient.invalidateQueries({ queryKey: ["moderator-log"] }),
      queryClient.invalidateQueries({ queryKey: ["admin-roles"] }),
    ]);
  };

  return (
    <>
      <Head>
        <title>GeoDuels | {moderatorSurface ? "Moderator" : "Admin"}</title>
        <meta name="robots" content="noindex,nofollow" />
      </Head>
      <main data-ui-theme="operational" className="min-h-screen bg-surface-page text-content-primary">
        <div className="grid min-h-screen lg:grid-cols-[280px_minmax(0,1fr)]">
          <aside className="border-r border-border-default bg-surface-page px-4 py-5">
            <Link href="/" className="mb-5 block rounded-lg border border-border-default bg-surface-panel p-4">
              <Text as="p" variant="label" className="text-status-success">
                GeoDuels {consoleEyebrow}
              </Text>
              <Heading as="h1" variant="heading-md" className="mt-1">{consoleTitle}</Heading>
            </Link>
            <nav className="space-y-5">
              {navGroups.map((group) => (
                <div key={group.title}>
                  <Text as="p" variant="caption" className="mb-2 px-2">
                    {group.title}
                  </Text>
                  <div className="space-y-1">
                    {group.items.map((item) => {
                      const selected = router.asPath.split("?")[0] === item.href;
                      const Icon = item.icon;
                      return (
                        <Link
                          key={item.href}
                          href={item.href}
                          className={`flex items-center gap-3 rounded-md px-3 py-2 text-body-sm font-semibold transition ${
                            selected
                              ? "bg-action-primary text-content-on-action"
                              : "text-content-secondary hover:bg-surface-panel hover:text-content-primary"
                          }`}
                        >
                          <Icon className="h-4 w-4" />
                          <span>{item.label}</span>
                          {selected ? <ChevronRight className="ml-auto h-4 w-4" /> : null}
                        </Link>
                      );
                    })}
                  </div>
                </div>
              ))}
            </nav>
          </aside>
          <div className="min-w-0 px-4 py-5 sm:px-6 lg:px-8">
            {!view.auth.userId ? (
              <Panel className="p-5 text-body-sm text-content-secondary">Sign in first to access the admin console.</Panel>
            ) : null}
            {view.auth.userId && !canViewReports ? (
              <Panel className="border-status-warning/40 bg-status-warning/10 p-5 text-body-sm text-status-warning">
                This account does not have {moderatorSurface ? "moderator" : "admin"} access.
              </Panel>
            ) : null}
            {view.auth.userId && canViewReports && !hasSurfaceAccess ? (
              <Panel className="border-status-warning/40 bg-status-warning/10 p-5 text-body-sm text-status-warning">
                This surface is admin-only. Use the moderator workbench for review tools.
              </Panel>
            ) : null}
            {hasSurfaceAccess ? (
              <>
                {moderatorSurface && section === "subjects" && !leaf ? (
                  <PlayersRoute
                    config={config}
                    accessToken={accessToken}
                    canManageAdmin={canManageAdmin}
                    basePath="/moderator/subjects"
                    title="Subject Search"
                    eyebrow="Moderation"
                    searchPlaceholder="Search user ID or display name"
                  />
                ) : null}
                {moderatorSurface && section === "subjects" && leaf ? (
                  <PlayerDetailRoute
                    config={config}
                    accessToken={accessToken}
                    userId={leaf}
                    canManageAdmin={canManageAdmin}
                    basePath="/moderator/subjects"
                    titleEyebrow="Moderation Subject"
                    refreshAdminData={refreshAdminData}
                  />
                ) : null}
                {moderatorSurface && section === "log" ? (
                  <EnforcementRoute config={config} accessToken={accessToken} canViewEnforcement={canViewReports} />
                ) : null}
                {moderatorSurface && section === "signals" ? (
                  <SignalsRoute config={config} accessToken={accessToken} />
                ) : null}
                {!moderatorSurface && (section === "operations" || section === "content") ? (
                  <OperationsRoute
                    config={config}
                    accessToken={accessToken}
                    leaf={leaf || path[1] || path[0]}
                    canManageAdmin={canManageAdmin}
                    refreshAdminData={refreshAdminData}
                  />
                ) : null}
                {!moderatorSurface && section === "access" ? (
                  leaf === "badges" ? (
                    <BadgeGrantsRoute config={config} accessToken={accessToken} canManageAdmin={canManageAdmin} />
                  ) : (
                    <AccessRoute
                      config={config}
                      accessToken={accessToken}
                      canManageAdmin={canManageAdmin}
                      refreshAdminData={refreshAdminData}
                    />
                  )
                ) : null}
              </>
            ) : null}
          </div>
        </div>
      </main>
    </>
  );
}

function PlayersRoute(props: {
  config: ReturnType<typeof getRuntimeConfig>;
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
    queryFn: () => requestAdminPlayers(props.config, props.accessToken, query),
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

function PlayerDetailRoute(props: {
  config: ReturnType<typeof getRuntimeConfig>;
  accessToken: string;
  userId: string;
  canManageAdmin: boolean;
  basePath?: string;
  titleEyebrow?: string;
  refreshAdminData: () => Promise<void>;
}) {
  const [banReason, setBanReason] = useState("");
  const moderatorSubject = (props.basePath || "").startsWith("/moderator");
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

function formatDate(value?: string) {
  if (!value) return "Unknown";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Unknown";
  return date.toLocaleString();
}

function formatUTCDate(value?: string) {
  if (!value) return "Unknown";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Unknown";
  return date.toLocaleString(undefined, { timeZone: "UTC", timeZoneName: "short" });
}

function OperationsRoute(props: {
  config: ReturnType<typeof getRuntimeConfig>;
  accessToken: string;
  leaf: string;
  canManageAdmin: boolean;
  refreshAdminData: () => Promise<void>;
}) {
  const [selectedChangelogId, setSelectedChangelogId] = useState<number | "new">("new");
  const [changelogDraft, setChangelogDraft] = useState<ChangelogPostInput>({
    slug: "",
    title: "",
    markdown: "",
    published: true,
  });
  const [phase, setPhase] = useState<MaintenanceStatus["phase"]>("normal");
  const [message, setMessage] = useState("");
  const [startsAt, setStartsAt] = useState("");
  const [endsAt, setEndsAt] = useState("");
  const [queuePaused, setQueuePaused] = useState(false);
  const [playPaused, setPlayPaused] = useState(false);
  const [webhook, setWebhook] = useState("");
  const [discordSettings, setDiscordSettings] = useState({
    guildId: "",
    joinsChannelId: "",
    elo1000RoleId: "",
    elo1500RoleId: "",
    elo2000RoleId: "",
    reconcileIntervalMinutes: 15,
  });
  const [ipAddress, setIPAddress] = useState("");
  const [ipReason, setIPReason] = useState("");
  const [monthlyResetDay, setMonthlyResetDay] = useState("1");
  const [pardonDialogOpen, setPardonDialogOpen] = useState(false);
  const [pardonResult, setPardonResult] = useState<{ eligible: number; pardoned: number } | null>(null);

  const maintenanceQuery = useQuery({
    queryKey: ["admin-maintenance", props.accessToken],
    enabled: props.canManageAdmin && !!props.accessToken,
    queryFn: () => requestAdminMaintenance(props.config, props.accessToken),
  });
  const changelogQuery = useQuery({
    queryKey: ["admin-changelog", props.accessToken],
    enabled: props.canManageAdmin && !!props.accessToken,
    queryFn: () => requestAdminGetChangelog(props.config, props.accessToken),
  });
  const settingsQuery = useQuery({
    queryKey: ["admin-moderation-settings", props.accessToken],
    enabled: props.canManageAdmin && !!props.accessToken,
    queryFn: () => requestAdminModerationSettings(props.config, props.accessToken),
  });
  const discordSettingsQuery = useQuery({
    queryKey: ["admin-discord-integration-settings", props.accessToken],
    enabled: props.canManageAdmin && !!props.accessToken,
    queryFn: () => requestAdminDiscordIntegrationSettings(props.config, props.accessToken),
  });
  const ipBansQuery = useQuery({
    queryKey: ["admin-ip-signup-bans", props.accessToken],
    enabled: props.canManageAdmin && !!props.accessToken,
    queryFn: () => requestAdminIPSignupBans(props.config, props.accessToken),
  });
  const seasonQuery = useQuery({
    queryKey: ["admin-ranked-season", props.accessToken],
    enabled: props.canManageAdmin && !!props.accessToken,
    queryFn: () => requestAdminRankedSeason(props.config, props.accessToken),
  });
  const pardonQuery = useQuery({
    queryKey: ["admin-community-pardon", props.accessToken],
    enabled: props.canManageAdmin && !!props.accessToken,
    queryFn: () => requestAdminCommunityPardonPreview(props.config, props.accessToken),
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

  useEffect(() => {
    const posts = changelogQuery.data?.posts || [];
    if (selectedChangelogId !== "new" || posts.length === 0) return;
    const latest = posts[0];
    setSelectedChangelogId(latest.id);
    setChangelogDraft({
      slug: latest.slug,
      title: latest.title,
      markdown: latest.markdown,
      published: latest.published,
    });
  }, [changelogQuery.data]);

  const selectChangelogPost = (post: ChangelogPost) => {
    setSelectedChangelogId(post.id);
    setChangelogDraft({
      slug: post.slug,
      title: post.title,
      markdown: post.markdown,
      published: post.published,
    });
  };

  const startNewChangelogPost = () => {
    setSelectedChangelogId("new");
    setChangelogDraft({
      slug: "",
      title: "",
      markdown: "",
      published: true,
    });
  };

  useEffect(() => {
    setWebhook(settingsQuery.data?.discordWebhookUrl || "");
  }, [settingsQuery.data?.discordWebhookUrl]);

  useEffect(() => {
    if (!discordSettingsQuery.data) return;
    setDiscordSettings(discordSettingsQuery.data);
  }, [discordSettingsQuery.data]);

  useEffect(() => {
    if (typeof seasonQuery.data?.monthlyResetDay !== "number") return;
    setMonthlyResetDay(String(seasonQuery.data.monthlyResetDay));
  }, [seasonQuery.data?.monthlyResetDay]);

  const saveMaintenance = useMutation({
    mutationFn: () =>
      requestAdminPutMaintenance(props.config, props.accessToken, {
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
    mutationFn: () => requestAdminClearMaintenance(props.config, props.accessToken),
    onSuccess: props.refreshAdminData,
  });
  const saveChangelogPost = useMutation({
    mutationFn: () => {
      const content = {
        ...changelogDraft,
        slug: changelogDraft.slug || slugify(changelogDraft.title),
      };
      if (selectedChangelogId === "new") {
        return requestAdminCreateChangelogPost(props.config, props.accessToken, content);
      }
      return requestAdminUpdateChangelogPost(props.config, props.accessToken, selectedChangelogId, content);
    },
    onSuccess: async (post) => {
      setSelectedChangelogId(post.id);
      setChangelogDraft({
        slug: post.slug,
        title: post.title,
        markdown: post.markdown,
        published: post.published,
      });
      await props.refreshAdminData();
    },
  });
  const saveSettings = useMutation({
    mutationFn: () => requestAdminPutModerationSettings(props.config, props.accessToken, { discordWebhookUrl: webhook }),
    onSuccess: props.refreshAdminData,
  });
  const saveDiscordSettings = useMutation({
    mutationFn: () =>
      requestAdminPutDiscordIntegrationSettings(
        props.config,
        props.accessToken,
        discordSettings,
      ),
    onSuccess: props.refreshAdminData,
  });
  const addIPBan = useMutation({
    mutationFn: () => requestAdminAddIPSignupBan(props.config, props.accessToken, ipAddress, ipReason),
    onSuccess: props.refreshAdminData,
  });
  const removeIPBan = useMutation({
    mutationFn: (ip: string) => requestAdminRemoveIPSignupBan(props.config, props.accessToken, ip),
    onSuccess: props.refreshAdminData,
  });
  const saveSeasonResetRule = useMutation({
    mutationFn: () => requestAdminSetRankedSeasonResetRule(props.config, props.accessToken, Number(monthlyResetDay)),
    onSuccess: props.refreshAdminData,
  });
  const pardonMutation = useMutation({
    mutationFn: () => requestAdminCommunityPardon(props.config, props.accessToken),
    onSuccess: async (result) => {
      setPardonDialogOpen(false);
      setPardonResult(result);
      await props.refreshAdminData();
      await pardonQuery.refetch();
    },
  });

  if (!props.canManageAdmin) {
    return <Panel className="p-5 text-body-sm text-content-secondary">Admin access is required for operations.</Panel>;
  }

  const ipBans = (ipBansQuery.data?.bans || []) as IPBan[];
  const changelogPosts = changelogQuery.data?.posts || [];
  const selectedChangelogPost =
    selectedChangelogId === "new"
      ? null
      : changelogPosts.find((post) => post.id === selectedChangelogId) || null;

  return (
    <div className="space-y-4">
      <header>
        <Text as="p" variant="label" className="text-status-success">Operations</Text>
        <Heading as="h2" variant="display-md" className="mt-1">Admin Operations</Heading>
      </header>
      {props.leaf === "maintenance" || props.leaf === "" ? (
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
      <div className="grid gap-4 xl:grid-cols-2">
        {(props.leaf === "maintenance" || props.leaf === "") ? (
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
        ) : null}

        {props.leaf === "notifications" ? (
          <Panel className="p-4">
            <Heading as="h3" variant="heading-sm">Report Notifications</Heading>
            <Input className="mt-4 w-full" type="password" value={webhook} onChange={(event) => setWebhook(event.target.value)} placeholder="Discord webhook URL" />
            <Button className="mt-3" onClick={() => void saveSettings.mutateAsync()}>Save Webhook</Button>
          </Panel>
        ) : null}

        {props.leaf === "discord" ? (
          <Panel className="p-4 xl:col-span-2">
            <Heading as="h3" variant="heading-sm">Discord Integration</Heading>
            <p className="mt-2 text-body-sm text-content-secondary">
              The bot token remains a deployment secret. These IDs refresh in the worker automatically.
            </p>
            <div className="mt-4 grid gap-3 md:grid-cols-2">
              <Input
                value={discordSettings.guildId}
                onChange={(event) => setDiscordSettings((current) => ({ ...current, guildId: event.target.value }))}
                placeholder="Guild ID"
              />
              <Input
                value={discordSettings.joinsChannelId}
                onChange={(event) => setDiscordSettings((current) => ({ ...current, joinsChannelId: event.target.value }))}
                placeholder="#joins channel ID (optional)"
              />
              <Input
                value={discordSettings.elo1000RoleId}
                onChange={(event) => setDiscordSettings((current) => ({ ...current, elo1000RoleId: event.target.value }))}
                placeholder="1k ELO role ID"
              />
              <Input
                value={discordSettings.elo1500RoleId}
                onChange={(event) => setDiscordSettings((current) => ({ ...current, elo1500RoleId: event.target.value }))}
                placeholder="1.5k ELO role ID"
              />
              <Input
                value={discordSettings.elo2000RoleId}
                onChange={(event) => setDiscordSettings((current) => ({ ...current, elo2000RoleId: event.target.value }))}
                placeholder="2k ELO role ID"
              />
              <Input
                type="number"
                min={1}
                max={1440}
                value={discordSettings.reconcileIntervalMinutes}
                onChange={(event) => setDiscordSettings((current) => ({
                  ...current,
                  reconcileIntervalMinutes: Number(event.target.value),
                }))}
                placeholder="Reconciliation interval (minutes)"
              />
            </div>
            <Button
              className="mt-4"
              disabled={
                saveDiscordSettings.isPending ||
                discordSettings.reconcileIntervalMinutes < 1 ||
                discordSettings.reconcileIntervalMinutes > 1440
              }
              onClick={() => void saveDiscordSettings.mutateAsync()}
            >
              Save Discord Settings
            </Button>
            {saveDiscordSettings.error ? (
              <p className="mt-3 text-body-sm text-status-danger">{saveDiscordSettings.error.message}</p>
            ) : null}
          </Panel>
        ) : null}

        {props.leaf === "seasons" ? (
          <Panel className="p-4">
            <Heading as="h3" variant="heading-sm">Ranked Season</Heading>
            <p className="mt-2 text-body-sm text-content-secondary">Active: {seasonQuery.data?.activeSeasonId || "loading"}</p>
            <div className="mt-4 grid gap-3 md:grid-cols-[180px_1fr]">
              <Input
                type="number"
                min={1}
                max={28}
                value={monthlyResetDay}
                onChange={(event) => setMonthlyResetDay(event.target.value)}
                placeholder="Reset day"
              />
              <div className="rounded-lg border border-border-default bg-surface-inset px-3 py-2 text-body-sm text-content-secondary">
                <p>Monthly on day {seasonQuery.data?.monthlyResetDay || "--"} at 21:00 UTC</p>
                <p className="mt-1 text-body-sm text-content-secondary">
                  Next reset: {seasonQuery.data?.nextResetAt ? formatUTCDate(seasonQuery.data.nextResetAt) : "Not scheduled"}
                </p>
              </div>
            </div>
            <div className="mt-4">
              <Button disabled={Number(monthlyResetDay) < 1 || Number(monthlyResetDay) > 28} onClick={() => void saveSeasonResetRule.mutateAsync()}>Save Reset Rule</Button>
            </div>
          </Panel>
        ) : null}

        {props.leaf === "changelog" ? (
          <Panel className="p-4 xl:col-span-2">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <Heading as="h3" variant="heading-sm">Changelog</Heading>
                <p className="mt-1 text-body-sm text-content-secondary">
                  Write release notes as Markdown. Saving a post updates its modified date automatically.
                </p>
              </div>
              <Button onClick={startNewChangelogPost}>New Post</Button>
            </div>

            <div className="mt-5 grid gap-4 xl:grid-cols-[280px_minmax(0,1fr)]">
              <div className="space-y-2">
                {changelogPosts.length === 0 ? (
                  <div className="rounded-md border border-border-default bg-surface-grouped p-3 text-body-sm text-content-secondary">
                    No changelog posts yet.
                  </div>
                ) : null}
                {changelogPosts.map((post) => {
                  const selected = selectedChangelogId === post.id;
                  return (
                    <Button
                      variant="ghost"
                      key={post.id}
                      type="button"
                      onClick={() => selectChangelogPost(post)}
                      className={`w-full rounded-md border p-3 text-left transition ${
                        selected
                          ? "border-status-success bg-status-success/10"
                          : "border-border-default bg-surface-grouped hover:border-border-strong"
                      }`}
                    >
                      <div className="flex items-center justify-between gap-2">
                        <p className="line-clamp-2 font-strong text-content-primary">{post.title}</p>
                        <Badge tone={post.published ? "success" : "warning"}>
                          {post.published ? "Live" : "Draft"}
                        </Badge>
                      </div>
                      <p className="mt-1 truncate text-body-sm text-content-secondary">/{post.slug}</p>
                      <p className="mt-2 text-body-sm text-content-secondary">
                        Modified {formatAdminDate(post.updatedAt)}
                      </p>
                    </Button>
                  );
                })}
              </div>

              <div className="min-w-0 space-y-3">
                <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_minmax(0,0.8fr)]">
                  <Input
                    value={changelogDraft.title}
                    onChange={(event) =>
                      setChangelogDraft((draft) => ({
                        ...draft,
                        title: event.target.value,
                        slug: draft.slug || slugify(event.target.value),
                      }))
                    }
                    placeholder="Post title"
                  />
                  <Input
                    value={changelogDraft.slug}
                    onChange={(event) =>
                      setChangelogDraft((draft) => ({
                        ...draft,
                        slug: sanitizeSlugInput(event.target.value),
                      }))
                    }
                    placeholder="url-slug"
                  />
                </div>
                <div className="admin-markdown-editor overflow-hidden rounded-lg border border-border-default">
                  <SimpleMDE
                    value={changelogDraft.markdown}
                    onChange={(value) =>
                      setChangelogDraft((draft) => ({ ...draft, markdown: value || "" }))
                    }
                    options={changelogMarkdownOptions}
                  />
                </div>
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <label className="flex items-center gap-2 text-body-sm font-semibold text-content-secondary">
                    <Checkbox
                      type="checkbox"
                      checked={changelogDraft.published}
                      onChange={(event) =>
                        setChangelogDraft((draft) => ({ ...draft, published: event.target.checked }))
                      }
                    />
                    Published
                  </label>
                  <div className="flex items-center gap-2">
                    {selectedChangelogPost ? (
                      <Link
                        href={`/changelog/${encodeURIComponent(selectedChangelogPost.slug)}`}
                        className="inline-flex items-center gap-2 rounded-md border border-border-strong px-3 py-2 text-body-sm font-semibold text-content-primary hover:border-status-success hover:text-status-success"
                      >
                        View Post
                        <ExternalLink className="h-4 w-4" />
                      </Link>
                    ) : null}
                    <Button
                      disabled={!changelogDraft.title.trim() || saveChangelogPost.isPending}
                      onClick={() => void saveChangelogPost.mutateAsync()}
                    >
                      {saveChangelogPost.isPending ? "Saving..." : selectedChangelogId === "new" ? "Create Post" : "Save Post"}
                    </Button>
                  </div>
                </div>
                {saveChangelogPost.error ? (
                  <p className="text-body-sm font-semibold text-status-danger">
                    {saveChangelogPost.error instanceof Error ? saveChangelogPost.error.message : "Failed to save changelog post"}
                  </p>
                ) : null}
              </div>
            </div>
          </Panel>
        ) : null}

        {props.leaf === "ip-signup-blocks" ? (
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
        ) : null}
      </div>
    </div>
  );
}

function EnforcementRoute(props: {
  config: ReturnType<typeof getRuntimeConfig>;
  accessToken: string;
  canViewEnforcement: boolean;
}) {
  const actionsQuery = useQuery({
	queryKey: ["moderator-log", props.accessToken],
	enabled: props.canViewEnforcement && !!props.accessToken,
	queryFn: () => requestModeratorLog(props.config, props.accessToken),
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

function AccessRoute(props: {
  config: ReturnType<typeof getRuntimeConfig>;
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
    queryFn: () => requestAdminRoles(props.config, props.accessToken),
  });
  const grantRole = useMutation({
    mutationFn: () => requestAdminGrantRole(props.config, props.accessToken, { userId, role, reason }),
    onSuccess: props.refreshAdminData,
  });
  const revokeRole = useMutation({
    mutationFn: (grant: UserRoleGrant) => requestAdminRevokeRole(props.config, props.accessToken, grant.userId, grant.role, reason),
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
