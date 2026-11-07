import { useAuthState } from "../auth/components/AuthProvider";
import { MapOfTheWeekRoute } from "./components/MapOfTheWeekRoute";
import Head from "next/head";
import Link from "next/link";
import { useRouter } from "next/router";
import { useQueryClient } from "@tanstack/react-query";
import { ChevronRight } from "lucide-react";
import { useEffect } from "react";
import { Heading, Text } from "../../components/ui/typography";
import { AdminPanel as Panel } from "./components/admin-primitives";
import { SignalsRoute } from "./components/SignalsRoute";
import { BadgeGrantsRoute } from "./components/BadgeGrantsRoute";
import { AccessRoute } from "./routes/AccessRoute";
import { EnforcementRoute } from "./routes/EnforcementRoute";
import { OperationsRoute } from "./routes/OperationsRoute";
import { PlayerDetailRoute } from "./routes/PlayerDetailRoute";
import { PlayersRoute } from "./routes/PlayersRoute";
import { staffNavigation, moderatorPathFromRouter, pathFromRouter } from "./lib/admin-navigation";
import { useHomeModel } from "../home/model/useHomeModel";

type AdminSurface = "admin" | "moderator";

export default function AdminPage({ surface = "admin" }: { surface?: AdminSurface }) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { view } = useHomeModel({ routeContext: "home", backgroundDataEnabled: false });
  const auth = useAuthState();
  const rawPath = surface === "moderator" ? moderatorPathFromRouter(router) : pathFromRouter(router);
  const moderatorSurface = surface === "moderator" || rawPath[0] === "judge";
  const path = rawPath[0] === "judge" ? rawPath.slice(1) : rawPath;
  const section = path[0] || (moderatorSurface ? "subjects" : "operations");
  const leaf = path[1] || "";
  const accessToken = auth.accessToken;
  const canViewReports = auth.isJudge;
  const canManageAdmin = auth.isAdmin;
  const navGroups = staffNavigation(auth.roles);
  const hasStaffAccess = navGroups.length > 0;
  const requiredRole = moderatorSurface ? "judge" : section === "moderator" ? "moderator" : section === "lanista" ? "lanista" : "admin";
  const hasSurfaceAccess = auth.roles.includes(requiredRole);
  const consoleTitle = "Staff Console";
  const consoleEyebrow = "Team";

  useEffect(() => {
    if (!router.isReady || !hasStaffAccess) return;
    if (router.pathname === "/admin") void router.replace(navGroups[0].items[0].href);
    if (surface === "moderator") void router.replace(`/admin/judge/${rawPath.join("/")}`);
  }, [router.isReady, router.pathname, surface, hasStaffAccess, rawPath.join("/"), auth.roles.join(",")]);

  const refreshAdminData = async () => {
    await Promise.all([
	  queryClient.invalidateQueries({ queryKey: ["admin-players"] }), queryClient.invalidateQueries({ queryKey: ["moderator-signals"] }),
      queryClient.invalidateQueries({ queryKey: ["moderator-subject"] }),
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
        <title>GeoDuels | Staff</title>
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
            {view.auth.userId && !hasStaffAccess ? (
              <Panel className="border-status-warning/40 bg-status-warning/10 p-5 text-body-sm text-status-warning">
                This account does not have staff access.
              </Panel>
            ) : null}
            {view.auth.userId && hasStaffAccess && !hasSurfaceAccess ? (
              <Panel className="border-status-warning/40 bg-status-warning/10 p-5 text-body-sm text-status-warning">
                The {requiredRole} role is required for this page.
              </Panel>
            ) : null}
            {hasSurfaceAccess ? (
              <>
                {section === "moderator" ? <MapOfTheWeekRoute /> : null}
                {section === "lanista" ? <Panel className="p-6"><Heading as="h2" variant="heading-md">Events</Heading><p className="mt-3 text-content-secondary">Event management is coming soon.</p></Panel> : null}
                {moderatorSurface && section === "subjects" && !leaf ? (
                  <PlayersRoute
                    accessToken={accessToken}
                    canManageAdmin={canManageAdmin}
                    basePath="/admin/judge/subjects"
                    title="Subject Search"
                    eyebrow="Moderation"
                    searchPlaceholder="Search user ID or display name"
                  />
                ) : null}
                {moderatorSurface && section === "subjects" && leaf ? (
                  <PlayerDetailRoute
                    accessToken={accessToken}
                    userId={leaf}
                    canManageAdmin={canManageAdmin}
                    basePath="/admin/judge/subjects"
                    titleEyebrow="Moderation Subject"
                    refreshAdminData={refreshAdminData}
                  />
                ) : null}
                {moderatorSurface && section === "log" ? (
                  <EnforcementRoute accessToken={accessToken} canViewEnforcement={canViewReports} />
                ) : null}
                {moderatorSurface && section === "signals" ? (
                  <SignalsRoute accessToken={accessToken} />
                ) : null}
                {!moderatorSurface && (section === "operations" || section === "content") ? (
                  <OperationsRoute
                    accessToken={accessToken}
                    leaf={leaf || path[1] || path[0]}
                    canManageAdmin={canManageAdmin}
                    refreshAdminData={refreshAdminData}
                  />
                ) : null}
                {!moderatorSurface && section === "access" ? (
                  leaf === "badges" ? (
                    <BadgeGrantsRoute accessToken={accessToken} canManageAdmin={canManageAdmin} />
                  ) : (
                    <AccessRoute
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
