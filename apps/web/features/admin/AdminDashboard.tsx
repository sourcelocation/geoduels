import Head from "next/head";
import dynamic from "next/dynamic";
import Link from "next/link";
import { useRouter } from "next/router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, Ban, ChevronRight, ExternalLink, Gavel, LineChart, ShieldAlert } from "lucide-react";
import { useEffect, useState } from "react";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Select } from "../../components/ui/select";
import { Textarea } from "../../components/ui/textarea";
import { toPublicEntityId } from "../../lib/entity-id";
import { AdminDetailRow as DetailRow, AdminMetric as Metric, AdminPanel as Panel } from "./components/admin-primitives";
import { ModeratorIncidentRoute } from "./components/ModeratorIncidentRoute";
import { SignalsRoute } from "./components/SignalsRoute";
import { formatAdminDate, fromLocalDateTime, localDateTime, sanitizeSlugInput, slugify } from "./lib/admin-format";
import {
  requestAdminAddIPSignupBan,
  requestAdminBanPlayer,
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
  requestModeratorEnforcementActions,
  requestModeratorSubjectUnban,
  requestModeratorSubject,
  requestModeratorTasks,
} from "./lib/moderator-client";
import { requestPlayerMatches } from "../players/lib/player-client";
import { adminNav, isModerationReviewSection, moderationTitleForRoute, moderationViewForRoute, moderatorNav, moderatorPathFromRouter, pathFromRouter } from "./lib/admin-navigation";
import type { EnforcementAction, IPBan, MatchHistory, ModerationSubjectProfile, ModerationTask, Player, PlayerDetail, UserRoleGrant } from "./types";
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
  const section = path[0] || (moderatorSurface ? "queue" : "operations");
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
      void router.replace("/moderator/queue");
    }
  }, [leaf, moderatorSurface, router, section]);

  const refreshAdminData = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["admin-players"] }), queryClient.invalidateQueries({ queryKey: ["moderator-tasks"] }),
      queryClient.invalidateQueries({ queryKey: ["moderator-incident"] }), queryClient.invalidateQueries({ queryKey: ["moderator-signals"] }),
      queryClient.invalidateQueries({ queryKey: ["moderator-subject"] }), queryClient.invalidateQueries({ queryKey: ["admin-player-matches"] }),
      queryClient.invalidateQueries({ queryKey: ["admin-player-detail"] }), queryClient.invalidateQueries({ queryKey: ["admin-ip-signup-bans"] }),
      queryClient.invalidateQueries({ queryKey: ["admin-changelog"] }), queryClient.invalidateQueries({ queryKey: ["admin-maintenance"] }),
      queryClient.invalidateQueries({ queryKey: ["admin-moderation-settings"] }), queryClient.invalidateQueries({ queryKey: ["admin-discord-integration-settings"] }),
      queryClient.invalidateQueries({ queryKey: ["admin-ranked-season"] }), queryClient.invalidateQueries({ queryKey: ["admin-enforcement-actions"] }),
      queryClient.invalidateQueries({ queryKey: ["admin-roles"] }),
    ]);
  };

  const activeView = moderationViewForRoute(section, leaf);

  return (
    <>
      <Head>
        <title>GeoDuels | {moderatorSurface ? "Moderator" : "Admin"}</title>
        <meta name="robots" content="noindex,nofollow" />
      </Head>
      <main className="min-h-screen bg-slate-950 text-slate-100">
        <div className="grid min-h-screen lg:grid-cols-[280px_minmax(0,1fr)]">
          <aside className="border-r border-slate-800 bg-slate-950 px-4 py-5">
            <Link href="/" className="mb-5 block rounded-lg border border-slate-800 bg-slate-900/70 p-4">
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-emerald-300">
                GeoDuels {consoleEyebrow}
              </p>
              <h1 className="mt-1 text-xl font-black text-white">{consoleTitle}</h1>
            </Link>
            <nav className="space-y-5">
              {navGroups.map((group) => (
                <div key={group.title}>
                  <p className="mb-2 px-2 text-[11px] font-bold uppercase tracking-[0.16em] text-slate-500">
                    {group.title}
                  </p>
                  <div className="space-y-1">
                    {group.items.map((item) => {
                      const selected = router.asPath.split("?")[0] === item.href;
                      const Icon = item.icon;
                      return (
                        <Link
                          key={item.href}
                          href={item.href}
                          className={`flex items-center gap-3 rounded-md px-3 py-2 text-sm font-semibold transition ${
                            selected
                              ? "bg-emerald-400 text-slate-950"
                              : "text-slate-300 hover:bg-slate-900 hover:text-white"
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
              <Panel className="p-5 text-slate-300">Sign in first to access the admin console.</Panel>
            ) : null}
            {view.auth.userId && !canViewReports ? (
              <Panel className="border-amber-500/40 bg-amber-500/10 p-5 text-amber-100">
                This account does not have {moderatorSurface ? "moderator" : "admin"} access.
              </Panel>
            ) : null}
            {view.auth.userId && canViewReports && !hasSurfaceAccess ? (
              <Panel className="border-amber-500/40 bg-amber-500/10 p-5 text-amber-100">
                This surface is admin-only. Use the moderator workbench for review tools.
              </Panel>
            ) : null}
            {hasSurfaceAccess ? (
              <>
                {moderatorSurface && isModerationReviewSection(section) ? (
                  <ModerationRoute
                    config={config}
                    accessToken={accessToken}
                    view={activeView}
                    title={moderationTitleForRoute(section, leaf)}
                    refreshAdminData={refreshAdminData}
                  />
                ) : null}
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
                {moderatorSurface && section === "incidents" && leaf ? (
                  <ModeratorIncidentRoute
                    config={config}
                    accessToken={accessToken}
                    incidentId={Number(leaf) || 0}
                    refreshAdminData={refreshAdminData}
                  />
                ) : null}
                {moderatorSurface && section === "enforcement" ? (
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
                  <AccessRoute
                    config={config}
                    accessToken={accessToken}
                    canManageAdmin={canManageAdmin}
                    refreshAdminData={refreshAdminData}
                  />
                ) : null}
              </>
            ) : null}
          </div>
        </div>
      </main>
    </>
  );
}

function ModerationRoute(props: {
  config: ReturnType<typeof getRuntimeConfig>;
  accessToken: string;
  view: string;
  title?: string;
  refreshAdminData: () => Promise<void>;
}) {
  const tasksQuery = useQuery({
    queryKey: ["moderator-tasks", props.view, props.accessToken],
    enabled: !!props.accessToken,
    queryFn: () => requestModeratorTasks(props.config, props.accessToken, props.view),
    staleTime: 5_000,
  });
  const tasks = (tasksQuery.data?.tasks || []) as ModerationTask[];
  const title = props.title || props.view
    .split("-")
    .map((part) => part.slice(0, 1).toUpperCase() + part.slice(1))
    .join(" ");

  return (
    <div className="space-y-4">
      <header className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <p className="text-xs font-bold uppercase tracking-[0.16em] text-emerald-300">Moderation</p>
          <h2 className="mt-1 text-3xl font-black text-white">{title}</h2>
          <p className="mt-2 max-w-2xl text-sm text-slate-400">
            Work the queue from oldest/highest-risk incidents. Open an incident for evidence, context, and verdicts.
          </p>
        </div>
        <p className="text-sm text-slate-400">{tasks.length} tasks</p>
      </header>

      <Panel className="overflow-hidden">
        <div className="grid gap-0 divide-y divide-slate-900">
          {tasks.map((task) => (
            <Link
              key={task.id}
              href={`/moderator/incidents/${task.incidentId}`}
              className="grid gap-3 px-4 py-4 transition hover:bg-slate-900 md:grid-cols-[minmax(0,1fr)_160px_120px] md:items-center"
            >
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <p className="truncate font-bold text-white">{task.incident.subjectName || task.incident.subjectUserId}</p>
                  <span className="rounded bg-slate-800 px-2 py-0.5 text-[11px] font-bold uppercase text-slate-300">{task.incident.severity}</span>
                  <span className="rounded bg-slate-800 px-2 py-0.5 text-[11px] font-bold uppercase text-slate-300">{task.incident.evidenceStrength}</span>
                </div>
                <p className="mt-1 text-sm text-slate-400">{task.incident.reasonCode} · {task.incident.signalCount} signals · {task.incident.uniqueReporterCount} reporters</p>
                <p className="mt-1 text-xs text-slate-500">Incident #{task.incidentId} · Task #{task.id}</p>
              </div>
              <div className="text-sm text-slate-400">
                <p className="font-semibold text-slate-200">{task.assignedTo ? "Claimed" : task.status}</p>
                <p className="text-xs text-slate-500">{new Date(task.updatedAt || task.createdAt).toLocaleString()}</p>
              </div>
              <div className="justify-self-start md:justify-self-end">
                <span className="inline-flex min-h-9 items-center rounded-md border border-slate-700 px-3 text-sm font-semibold text-sky-300">
                  Open
                  <ChevronRight className="ml-1 h-4 w-4" />
                </span>
              </div>
            </Link>
          ))}
          {!tasksQuery.isLoading && tasks.length === 0 ? (
            <p className="p-4 text-sm text-slate-400">No tasks in this view.</p>
          ) : null}
          {tasksQuery.isLoading ? <p className="p-4 text-sm text-slate-400">Loading tasks...</p> : null}
        </div>
      </Panel>
    </div>
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
        <p className="text-xs font-bold uppercase tracking-[0.16em] text-emerald-300">{props.eyebrow || "Players"}</p>
        <h2 className="mt-1 text-3xl font-black text-white">{props.title || "Player Search"}</h2>
      </header>
      <Panel className="p-4">
        <div className="flex flex-col gap-3 sm:flex-row">
          <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={props.searchPlaceholder || "Search user ID, name, email, OAuth ID"} className="w-full" />
        </div>
      </Panel>
      <Panel className="overflow-x-auto">
        <table className="w-full min-w-[840px] text-left text-sm">
          <thead className="border-b border-slate-800 text-xs uppercase tracking-[0.12em] text-slate-500">
            <tr>
              <th className="px-4 py-3">Player</th>
              <th className="px-4 py-3">MMR</th>
              <th className="px-4 py-3">Record</th>
              <th className="px-4 py-3">Status</th>
              <th className="px-4 py-3 text-right">Open</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-900">
            {players.map((player) => (
              <tr key={player.userId}>
                <td className="px-4 py-3">
                  <Link className="text-left font-bold text-white hover:text-emerald-300" href={`${basePath}/${encodeURIComponent(toPublicEntityId(player.userId))}`}>
                    {player.displayName || player.userId}
                  </Link>
                  <p className="mt-1 text-xs text-slate-500">{props.canManageAdmin ? player.email || player.userId : player.userId}</p>
                </td>
                <td className="px-4 py-3">{player.mmr}</td>
                <td className="px-4 py-3 text-slate-400">{player.wins}W / {player.gamesPlayed}G</td>
                <td className="px-4 py-3">{player.isBanned ? "Banned" : "Active"}</td>
                <td className="px-4 py-3 text-right">
                  <Link className="inline-flex items-center gap-2 rounded-md border border-slate-700 px-3 py-2 font-semibold text-slate-100 hover:border-emerald-400 hover:text-emerald-200" href={`${basePath}/${encodeURIComponent(toPublicEntityId(player.userId))}`}>
                    Details
                    <ChevronRight className="h-4 w-4" />
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {!players.length ? <p className="p-4 text-sm text-slate-400">No players found.</p> : null}
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
  const matchesQuery = useQuery({
    queryKey: ["admin-player-matches", props.userId, props.accessToken],
    enabled: !!props.accessToken && !!props.userId,
    queryFn: () => requestPlayerMatches(props.config, props.userId, 25),
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
  const detail = detailQuery.data as (PlayerDetail & Partial<ModerationSubjectProfile>) | undefined;
  const player = detail?.player;
  const matches = (matchesQuery.data?.matches || []) as MatchHistory[];
  const winRate = player?.gamesPlayed ? Math.round((player.wins / player.gamesPlayed) * 100) : 0;
  const basePath = props.basePath || "/admin/players";

  if (detailQuery.isLoading) {
    return <Panel className="p-5 text-slate-300">Loading player details...</Panel>;
  }
  if (!player) {
    return <Panel className="p-5 text-slate-300">Player detail unavailable.</Panel>;
  }

  return (
    <div className="space-y-4">
      <header className="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <Link href={basePath} className="inline-flex items-center gap-2 text-sm font-semibold text-slate-400 hover:text-white">
            <ArrowLeft className="h-4 w-4" />
            Subjects
          </Link>
          <div className="mt-4 flex items-center gap-4">
            <div className="grid h-16 w-16 place-items-center rounded-md border border-slate-700 bg-slate-900 text-2xl font-black text-emerald-200">
              {(player.displayName || player.userId || "?").slice(0, 1).toUpperCase()}
            </div>
            <div>
              <p className="text-xs font-bold uppercase tracking-[0.16em] text-emerald-300">{props.titleEyebrow || "Player Detail"}</p>
              <h2 className="mt-1 break-all text-3xl font-black text-white">{player.displayName || player.userId}</h2>
              <p className="mt-1 break-all text-sm text-slate-400">{props.canManageAdmin ? player.email || player.userId : player.userId}</p>
            </div>
          </div>
        </div>
        <div className="flex flex-col gap-3 sm:flex-row">
          <Link href={`/players/${encodeURIComponent(player.displayName || player.userId)}`} className="inline-flex items-center justify-center gap-2 rounded-md border border-slate-700 px-4 py-2 text-sm font-semibold text-sky-300 hover:border-sky-400 hover:text-white">
            Public profile
            <ExternalLink className="h-4 w-4" />
          </Link>
          <Input value={banReason} onChange={(event) => setBanReason(event.target.value)} placeholder="Enforcement reason" className="w-full sm:w-80" />
          {player.isBanned ? (
            <Button onClick={() => void unbanMutation.mutateAsync()}>Unban</Button>
          ) : (
            <Button className="border-red-500/50 bg-red-500/15 text-red-100" onClick={() => void banMutation.mutateAsync()}>
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

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.35fr)_minmax(340px,0.65fr)]">
        <Panel className="p-4">
          <div className="mb-3 flex items-center justify-between gap-3">
            <div>
              <p className="text-xs font-bold uppercase tracking-[0.14em] text-slate-500">Past 7 Days</p>
              <h3 className="mt-1 font-black text-white">ELO History</h3>
            </div>
            <LineChart className="h-5 w-5 text-emerald-300" />
          </div>
          <EloHistoryChart points={detail.eloHistory || []} fallbackMmr={player.mmr} />
        </Panel>
        <Panel className="p-4">
          <div className="flex items-center gap-2">
            <ShieldAlert className={`h-5 w-5 ${player.isBanned ? "text-red-300" : "text-emerald-300"}`} />
            <h3 className="font-black text-white">Account Signals</h3>
          </div>
          <div className="mt-4 space-y-3 text-sm">
            <DetailRow label="User ID" value={player.userId} />
            <DetailRow label="Account" value={player.isGuest ? "Guest" : "Registered"} />
            <DetailRow label="Role" value={player.isAdmin ? "Admin" : player.isModerator ? "Moderator" : "Player"} />
            <DetailRow label="Ban Reason" value={player.banReason || "None"} />
            <DetailRow label="Report Mute" value={player.reportMutedUntil ? formatDate(player.reportMutedUntil) : "None"} />
            {props.canManageAdmin ? <DetailRow label="Last IP" value={player.lastIpAddress || "Unknown"} /> : null}
          </div>
        </Panel>
      </div>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
        <Panel className="p-4">
          <h3 className="font-black text-white">Stats</h3>
          <div className="mt-4 grid gap-3 sm:grid-cols-2">
            <Metric label="Tracked Matches" value={`${detail.stats?.totalMatches || player.gamesPlayed}`} />
            <Metric label="Ranked Matches" value={`${detail.stats?.rankedMatches || player.rankedGamesPlayed}`} />
            <Metric label="Duels" value={`${detail.stats?.duelMatches || 0}`} />
            <Metric label="Singleplayer" value={`${detail.stats?.singleplayerRuns || 0}`} />
            <Metric label="Wins" value={`${detail.stats?.wins || player.wins}`} />
            <Metric label="Losses" value={`${detail.stats?.losses || 0}`} />
          </div>
        </Panel>
        <Panel className="p-4">
          <h3 className="font-black text-white">Recent Matches</h3>
          <div className="mt-3 space-y-2">
            {matches.map((match) => (
              <Link key={match.matchId} href={`/match/${encodeURIComponent(toPublicEntityId(match.matchId))}`} className="flex items-center justify-between gap-3 rounded-md border border-slate-800 bg-slate-900/60 p-3 text-sm hover:bg-slate-900">
                <div className="min-w-0">
                  <p className="truncate font-semibold text-white">{match.matchId}</p>
                  <p className="mt-1 text-slate-500">{match.mode} · {formatDate(match.endedAt)}</p>
                </div>
                <ExternalLink className="h-4 w-4 shrink-0 text-slate-500" />
              </Link>
            ))}
            {!matches.length ? <p className="text-sm text-slate-400">No persisted match history yet.</p> : null}
          </div>
        </Panel>
      </div>

      {moderatorSubject ? (
        <div className="grid gap-4 xl:grid-cols-2">
          <Panel className="p-4">
            <h3 className="font-black text-white">Moderation Incidents</h3>
            <div className="mt-3 space-y-2">
              {(detail.incidents || []).map((incident) => (
                <Link key={incident.id} href={`/moderator/incidents/${incident.id}`} className="block rounded-md border border-slate-800 bg-slate-900/60 p-3 text-sm hover:bg-slate-900">
                  <p className="font-semibold text-white">#{incident.id} · {incident.reasonCode}</p>
                  <p className="mt-1 text-slate-400">{incident.status} · {incident.severity} · {incident.signalCount} signals</p>
                </Link>
              ))}
              {!detail.incidents?.length ? <p className="text-sm text-slate-400">No moderation incidents for this subject.</p> : null}
            </div>
          </Panel>
          <Panel className="p-4">
            <h3 className="font-black text-white">Recent Signals</h3>
            <div className="mt-3 space-y-2">
              {(detail.signals || []).map((signal) => (
                <div key={signal.id} className="rounded-md border border-slate-800 bg-slate-900/60 p-3 text-sm">
                  <p className="font-semibold text-white">{signal.reasonCode}</p>
                  <p className="mt-1 text-slate-400">{signal.source} · {signal.severity} / {signal.evidenceStrength}</p>
                </div>
              ))}
              {!detail.signals?.length ? <p className="text-sm text-slate-400">No moderation signals for this subject.</p> : null}
            </div>
          </Panel>
        </div>
      ) : null}

      {props.canManageAdmin ? (
        <Panel className="p-4">
          <h3 className="font-black text-white">Linked Identity History</h3>
          <div className="mt-3 overflow-x-auto">
            <table className="w-full min-w-[760px] text-left text-sm">
              <thead className="border-b border-slate-800 text-xs uppercase tracking-[0.12em] text-slate-500">
                <tr>
                  <th className="px-3 py-2">Provider</th>
                  <th className="px-3 py-2">Provider User</th>
                  <th className="px-3 py-2">Email</th>
                  <th className="px-3 py-2">Name</th>
                  <th className="px-3 py-2">Last Seen</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-900">
                {(player.identities || []).map((identity) => (
                  <tr key={`${identity.provider}:${identity.providerUserId}:${identity.lastSeenAt || ""}`}>
                    <td className="px-3 py-2 text-white">{identity.provider}</td>
                    <td className="px-3 py-2 text-slate-400">{identity.providerUserId}</td>
                    <td className="px-3 py-2 text-slate-400">{identity.email || "None"}</td>
                    <td className="px-3 py-2 text-slate-400">{identity.providerName || "None"}</td>
                    <td className="px-3 py-2 text-slate-400">{formatDate(identity.lastSeenAt)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            {!player.identities?.length ? <p className="mt-3 text-sm text-slate-400">No linked identity history.</p> : null}
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

function EloHistoryChart(props: { points: PlayerDetail["eloHistory"]; fallbackMmr: number }) {
  const points = props.points || [];
  if (!points.length) {
    return (
      <div className="grid h-64 place-items-center rounded-md border border-slate-800 bg-slate-900/40 text-sm text-slate-400">
        No ranked ELO changes in the last 7 days.
      </div>
    );
  }
  const width = 720;
  const height = 260;
  const padX = 42;
  const padY = 28;
  const values = points.map((point) => point.mmr);
  const min = Math.min(...values, props.fallbackMmr);
  const max = Math.max(...values, props.fallbackMmr);
  const spread = Math.max(1, max - min);
  const xStep = points.length === 1 ? 0 : (width - padX * 2) / (points.length - 1);
  const coords = points.map((point, index) => {
    const x = points.length === 1 ? width / 2 : padX + index * xStep;
    const y = height - padY - ((point.mmr - min) / spread) * (height - padY * 2);
    return { x, y, point };
  });
  const polyline = coords.map((coord) => `${coord.x},${coord.y}`).join(" ");

  return (
    <div className="overflow-hidden rounded-md border border-slate-800 bg-slate-900/40">
      <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-label="Seven day ELO history" className="h-64 w-full">
        <line x1={padX} y1={padY} x2={padX} y2={height - padY} stroke="#334155" strokeWidth="1" />
        <line x1={padX} y1={height - padY} x2={width - padX} y2={height - padY} stroke="#334155" strokeWidth="1" />
        <text x={padX} y={18} fill="#94a3b8" fontSize="12">{max}</text>
        <text x={padX} y={height - 8} fill="#94a3b8" fontSize="12">{min}</text>
        <polyline fill="none" stroke="#34d399" strokeWidth="4" strokeLinecap="round" strokeLinejoin="round" points={polyline} />
        {coords.map(({ x, y, point }) => (
          <g key={point.date}>
            <circle cx={x} cy={y} r="5" fill="#34d399" />
            <text x={x} y={height - 10} textAnchor="middle" fill="#94a3b8" fontSize="11">
              {new Date(point.date).toLocaleDateString(undefined, { month: "short", day: "numeric" })}
            </text>
          </g>
        ))}
      </svg>
      <div className="grid divide-y divide-slate-800 border-t border-slate-800 sm:grid-cols-3 sm:divide-x sm:divide-y-0">
        {points.slice(-3).map((point) => (
          <div key={point.date} className="p-3 text-sm">
            <p className="font-bold text-white">{point.mmr} MMR</p>
            <p className={point.delta >= 0 ? "text-emerald-300" : "text-red-300"}>
              {point.delta >= 0 ? "+" : ""}{point.delta} across {point.played} ranked
            </p>
          </div>
        ))}
      </div>
    </div>
  );
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

  if (!props.canManageAdmin) {
    return <Panel className="p-5 text-slate-400">Admin access is required for operations.</Panel>;
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
        <p className="text-xs font-bold uppercase tracking-[0.16em] text-emerald-300">Operations</p>
        <h2 className="mt-1 text-3xl font-black text-white">Admin Operations</h2>
      </header>
      <div className="grid gap-4 xl:grid-cols-2">
        {(props.leaf === "maintenance" || props.leaf === "") ? (
          <Panel className="p-4">
            <h3 className="font-black text-white">Maintenance</h3>
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
              <label className="flex items-center gap-2 text-sm text-slate-300"><input type="checkbox" checked={queuePaused} onChange={(event) => setQueuePaused(event.target.checked)} /> Pause queue</label>
              <label className="flex items-center gap-2 text-sm text-slate-300"><input type="checkbox" checked={playPaused} onChange={(event) => setPlayPaused(event.target.checked)} /> Pause play</label>
            </div>
            <div className="mt-4 flex gap-2">
              <Button onClick={() => void saveMaintenance.mutateAsync()}>Save</Button>
              <Button onClick={() => void clearMaintenance.mutateAsync()}>Clear</Button>
            </div>
          </Panel>
        ) : null}

        {props.leaf === "notifications" ? (
          <Panel className="p-4">
            <h3 className="font-black text-white">Report Notifications</h3>
            <Input className="mt-4 w-full" type="password" value={webhook} onChange={(event) => setWebhook(event.target.value)} placeholder="Discord webhook URL" />
            <Button className="mt-3" onClick={() => void saveSettings.mutateAsync()}>Save Webhook</Button>
          </Panel>
        ) : null}

        {props.leaf === "discord" ? (
          <Panel className="p-4 xl:col-span-2">
            <h3 className="font-black text-white">Discord Integration</h3>
            <p className="mt-2 text-sm text-slate-400">
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
              <p className="mt-3 text-sm text-red-300">{saveDiscordSettings.error.message}</p>
            ) : null}
          </Panel>
        ) : null}

        {props.leaf === "seasons" ? (
          <Panel className="p-4">
            <h3 className="font-black text-white">Ranked Season</h3>
            <p className="mt-2 text-sm text-slate-400">Active: {seasonQuery.data?.activeSeasonId || "loading"}</p>
            <div className="mt-4 grid gap-3 md:grid-cols-[180px_1fr]">
              <Input
                type="number"
                min={1}
                max={28}
                value={monthlyResetDay}
                onChange={(event) => setMonthlyResetDay(event.target.value)}
                placeholder="Reset day"
              />
              <div className="rounded-xl border border-white/10 bg-black/20 px-3 py-2 text-sm text-slate-300">
                <p>Monthly on day {seasonQuery.data?.monthlyResetDay || "--"} at 21:00 UTC</p>
                <p className="mt-1 text-xs text-slate-500">
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
                <h3 className="font-black text-white">Changelog</h3>
                <p className="mt-1 text-sm text-slate-400">
                  Write release notes as Markdown. Saving a post updates its modified date automatically.
                </p>
              </div>
              <Button onClick={startNewChangelogPost}>New Post</Button>
            </div>

            <div className="mt-5 grid gap-4 xl:grid-cols-[280px_minmax(0,1fr)]">
              <div className="space-y-2">
                {changelogPosts.length === 0 ? (
                  <div className="rounded-md border border-slate-800 bg-slate-900/60 p-3 text-sm text-slate-400">
                    No changelog posts yet.
                  </div>
                ) : null}
                {changelogPosts.map((post) => {
                  const selected = selectedChangelogId === post.id;
                  return (
                    <button
                      key={post.id}
                      type="button"
                      onClick={() => selectChangelogPost(post)}
                      className={`w-full rounded-md border p-3 text-left transition ${
                        selected
                          ? "border-emerald-400 bg-emerald-400/10"
                          : "border-slate-800 bg-slate-900/60 hover:border-slate-600"
                      }`}
                    >
                      <div className="flex items-center justify-between gap-2">
                        <p className="line-clamp-2 font-bold text-white">{post.title}</p>
                        <span className={`rounded px-2 py-0.5 text-[11px] font-bold uppercase ${
                          post.published ? "bg-emerald-400/15 text-emerald-200" : "bg-amber-400/15 text-amber-200"
                        }`}>
                          {post.published ? "Live" : "Draft"}
                        </span>
                      </div>
                      <p className="mt-1 truncate text-xs text-slate-500">/{post.slug}</p>
                      <p className="mt-2 text-xs text-slate-500">
                        Modified {formatAdminDate(post.updatedAt)}
                      </p>
                    </button>
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
                <div className="admin-markdown-editor overflow-hidden rounded-lg border border-slate-800">
                  <SimpleMDE
                    value={changelogDraft.markdown}
                    onChange={(value) =>
                      setChangelogDraft((draft) => ({ ...draft, markdown: value || "" }))
                    }
                    options={changelogMarkdownOptions}
                  />
                </div>
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <label className="flex items-center gap-2 text-sm font-semibold text-slate-300">
                    <input
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
                        className="inline-flex items-center gap-2 rounded-md border border-slate-700 px-3 py-2 text-sm font-semibold text-slate-100 hover:border-emerald-400 hover:text-emerald-200"
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
                  <p className="text-sm font-semibold text-red-300">
                    {saveChangelogPost.error instanceof Error ? saveChangelogPost.error.message : "Failed to save changelog post"}
                  </p>
                ) : null}
              </div>
            </div>
          </Panel>
        ) : null}

        {props.leaf === "ip-signup-blocks" ? (
          <Panel className="p-4">
            <h3 className="font-black text-white">IP Signup Blocks</h3>
            <div className="mt-4 grid gap-2 md:grid-cols-[1fr_1fr_auto]">
              <Input value={ipAddress} onChange={(event) => setIPAddress(event.target.value)} placeholder="IP address" />
              <Input value={ipReason} onChange={(event) => setIPReason(event.target.value)} placeholder="Reason" />
              <Button disabled={!ipAddress} onClick={() => void addIPBan.mutateAsync()}>Block</Button>
            </div>
            <div className="mt-4 space-y-2">
              {ipBans.map((ban) => (
                <div key={ban.id} className="flex items-center justify-between rounded-md border border-slate-800 bg-slate-900/60 p-3">
                  <div>
                    <p className="font-semibold text-white">{ban.ipAddress}</p>
                    <p className="text-sm text-slate-500">{ban.reason || "No reason"}</p>
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
    queryKey: ["admin-enforcement-actions", props.accessToken],
    enabled: props.canViewEnforcement && !!props.accessToken,
    queryFn: () => requestModeratorEnforcementActions(props.config, props.accessToken),
  });
  if (!props.canViewEnforcement) {
    return <Panel className="p-5 text-slate-400">Moderator access is required for enforcement history.</Panel>;
  }
  const actions = (actionsQuery.data?.actions || []) as EnforcementAction[];
  return (
    <div className="space-y-4">
      <header>
        <p className="text-xs font-bold uppercase tracking-[0.16em] text-emerald-300">Enforcement</p>
        <h2 className="mt-1 text-3xl font-black text-white">Action History</h2>
      </header>
      <Panel className="overflow-x-auto">
        <table className="w-full min-w-[900px] text-left text-sm">
          <thead className="border-b border-slate-800 text-xs uppercase tracking-[0.12em] text-slate-500">
            <tr>
              <th className="px-4 py-3">Target</th>
              <th className="px-4 py-3">Action</th>
              <th className="px-4 py-3">Actor</th>
              <th className="px-4 py-3">Incident</th>
              <th className="px-4 py-3">Reason</th>
              <th className="px-4 py-3">Created</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-900">
            {actions.map((action) => (
              <tr key={action.id}>
                <td className="px-4 py-3">
                  <p className="font-bold text-white">{action.targetName || action.targetUserId}</p>
                  <p className="text-xs text-slate-500">{action.targetUserId}</p>
                </td>
                <td className="px-4 py-3 font-semibold text-white">{action.actionType}</td>
                <td className="px-4 py-3 text-slate-400">{action.actorName || action.actorUserId || "system"}</td>
                <td className="px-4 py-3">
                  {action.sourceIncidentId ? (
                    <Link className="text-sky-300 hover:text-white" href={`/moderator/incidents/${action.sourceIncidentId}`}>
                      #{action.sourceIncidentId}
                    </Link>
                  ) : (
                    <span className="text-slate-500">-</span>
                  )}
                </td>
                <td className="px-4 py-3 text-slate-400">{action.reasonNote || action.reasonCode || "-"}</td>
                <td className="px-4 py-3 text-slate-500">{new Date(action.createdAt).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {!actionsQuery.isLoading && actions.length === 0 ? <p className="p-4 text-sm text-slate-400">No enforcement actions yet.</p> : null}
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
        <p className="text-xs font-bold uppercase tracking-[0.16em] text-emerald-300">Access</p>
        <h2 className="mt-1 text-3xl font-black text-white">Roles</h2>
      </header>
      {!props.canManageAdmin ? (
        <Panel className="p-5 text-amber-200">Admin access is required to manage roles.</Panel>
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
            <table className="w-full min-w-[760px] text-left text-sm">
              <thead className="border-b border-slate-800 text-xs uppercase tracking-[0.12em] text-slate-500">
                <tr>
                  <th className="px-4 py-3">User</th>
                  <th className="px-4 py-3">Role</th>
                  <th className="px-4 py-3">Granted By</th>
                  <th className="px-4 py-3">Reason</th>
                  <th className="px-4 py-3 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-900">
                {roles.map((grant) => (
                  <tr key={`${grant.userId}:${grant.role}`}>
                    <td className="px-4 py-3">
                      <p className="font-bold text-white">{grant.displayName || grant.userId}</p>
                      <p className="text-xs text-slate-500">{grant.email || grant.userId}</p>
                    </td>
                    <td className="px-4 py-3 font-semibold text-white">{grant.role}</td>
                    <td className="px-4 py-3 text-slate-400">{grant.grantedBy || "system"}</td>
                    <td className="px-4 py-3 text-slate-400">{grant.reason || "-"}</td>
                    <td className="px-4 py-3 text-right">
                      <Button onClick={() => void revokeRole.mutateAsync(grant)}>Revoke</Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </Panel>
        </>
      )}
    </div>
  );
}
