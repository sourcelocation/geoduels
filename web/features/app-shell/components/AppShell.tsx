import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { Settings, Shield } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/router";
import { createContext, useCallback, useContext, useEffect, useId, useState, useSyncExternalStore, type ReactNode } from "react";
import AvatarBadge from "../../players/components/AvatarBadge";
import { MmrDisplay } from "../../players/components/MmrDisplay";
import PlayerBadge from "../../players/components/PlayerBadge";
import PlayerNameWithBadge from "../../players/components/PlayerNameWithBadge";
import { Button } from "../../../components/ui/button";
import { Tooltip } from "../../../components/ui/Tooltip";
import { AppChromeIconButton, AppChromeIconLink, AppNavigationSurface } from "../../../components/ui/compositions";
import { cn } from "../../../lib/cn";
import {
  APP_NAV_ITEMS,
  appNavRouteStorageKey,
  navRouteOf,
  type AppNavRoute,
} from "../navigation";
import { useOptionalHotkeys } from "../../hotkeys/components/HotkeyProvider";
import { AppBackground } from "./AppBackground";
import { socialStore } from "../../social/social-store";
import { AppNavTasks, type AppNavTask } from "./AppNavTasks";
import { useAuthActions, useAuthState } from "../../auth/components/AuthProvider";
import { useAppActivities } from "./AppActivityProvider";

export type AppShellOptions = {
  // A match takes the whole screen: nothing of the app shows around it.
  hidden?: boolean;
  backgroundBlurred?: boolean;
  contentClassName?: string;
  maintenanceBanner?: ReactNode;
  viewportLocked?: boolean;
};

type SetLayer = (id: string, options: AppShellOptions | null) => void;

const AppShellOptionsContext = createContext<SetLayer>(() => undefined);

/** Lets a page say how the frame around it should be, for as long as it's showing. */
export function useAppShellOptions({ hidden, backgroundBlurred, contentClassName, maintenanceBanner, viewportLocked }: AppShellOptions) {
  const setLayer = useContext(AppShellOptionsContext);
  const id = useId();
  useEffect(() => {
    setLayer(id, { hidden, backgroundBlurred, contentClassName, maintenanceBanner, viewportLocked });
    return () => setLayer(id, null);
  }, [hidden, backgroundBlurred, contentClassName, id, maintenanceBanner, setLayer, viewportLocked]);
}

/** Everything but the admin tools is a GeoDuels page, inside this: the header, navigation and background. */
export function AppShell({ children }: { children: ReactNode }) {
  const router = useRouter();
  const auth = useAuthState();
  const tasks = useAppActivities();
  const [layers, setLayers] = useState<ReadonlyMap<string, AppShellOptions>>(new Map());
  const setLayer = useCallback<SetLayer>((id, options) => {
    setLayers((current) => {
      const next = new Map(current);
      if (options) next.set(id, options);
      else next.delete(id);
      return next;
    });
  }, []);
  const { hidden, backgroundBlurred = false, contentClassName, maintenanceBanner, viewportLocked = false } = [...layers.values()].reduce<AppShellOptions>(
    (all, layer) => ({
      hidden: all.hidden || layer.hidden,
      backgroundBlurred: all.backgroundBlurred || layer.backgroundBlurred,
      contentClassName: all.contentClassName || layer.contentClassName,
      maintenanceBanner: all.maintenanceBanner || layer.maintenanceBanner,
      viewportLocked: all.viewportLocked || layer.viewportLocked,
    }),
    {},
  );
  return (
    <AppShellOptionsContext.Provider value={setLayer}>
      <div className={cn("relative flex flex-col overflow-hidden font-body text-content-primary selection:bg-status-success/30", viewportLocked ? "h-screen" : "min-h-screen")}>
        {hidden ? null : (
          <>
            <AppBackground blurred={backgroundBlurred} />
            <AppShellHeader
              auth={auth}
              maintenanceBanner={maintenanceBanner}
              activeNavRoute={navRouteOf(router.pathname)}
              onlinePlayers={auth.bootstrap?.global.onlinePlayers}
              tasks={tasks}
            />
            <div aria-hidden="true" className="h-[68px] shrink-0 sm:h-[82px] lg:h-[90px]" />
          </>
        )}
        <div
          className={cn(
            "relative flex min-h-0 flex-1 flex-col",
            viewportLocked && "app-shell-mobile-nav-safe-area",
            contentClassName,
          )}
        >
          {children}
        </div>
      </div>
    </AppShellOptionsContext.Provider>
  );
}

function AppShellHeader({
  auth,
  maintenanceBanner,
  activeNavRoute,
  onlinePlayers,
  tasks,
}: {
  auth: ReturnType<typeof useAuthState>;
  maintenanceBanner?: ReactNode;
  activeNavRoute: AppNavRoute | null;
  onlinePlayers?: number;
  tasks: AppNavTask[];
}) {
  const hotkeys = useOptionalHotkeys();
  const authActions = useAuthActions();
  return (
    <header className="fixed inset-x-0 top-0 z-sticky px-4 pb-4 pt-4 sm:px-6 sm:pb-5 sm:pt-5 lg:px-8 lg:pb-6 lg:pt-6">
      <AnimatePresence>{maintenanceBanner}</AnimatePresence>
      <div className="app-shell-header-layout">
        <div className="flex min-w-0 items-center gap-3 sm:gap-5">
          <Link href="/" aria-label="GeoDuels home" className="inline-flex shrink-0">
            <img
              src="/logo.v2.png"
              alt="GeoDuels"
              width={140}
              height={38}
              className="h-auto w-[112px] sm:w-[140px]"
            />
          </Link>
          {auth.status === "registered" && auth.roles.length > 0 ? (
            <Tooltip content="Staff" side="bottom">
              <AppChromeIconLink href="/admin" prefetch={false} aria-label="Staff" className="hidden sm:inline-flex">
                <Shield size={17} aria-hidden="true" />
              </AppChromeIconLink>
            </Tooltip>
          ) : null}
        </div>

        <AppNavigation
          activeRoute={activeNavRoute}
          onlinePlayers={onlinePlayers}
          tasks={tasks}
        />

        {auth.status === "bootstrapping" ? (
          <div aria-hidden="true" className="h-9 w-20 justify-self-end" />
        ) : auth.status === "registered" ? (
          <div className="flex items-center gap-2.5">
            <Tooltip content="Settings" side="bottom">
              <AppChromeIconButton
                aria-label="Open settings"
                onClick={() => hotkeys?.setSettingsOpen(true)}
              >
                <Settings size={18} aria-hidden="true" />
              </AppChromeIconButton>
            </Tooltip>
            <Link
              href={`/players/${encodeURIComponent(auth.displayName || auth.userId)}`}
              className="group flex min-w-0 cursor-pointer items-center justify-self-end gap-2.5 sm:gap-3"
            >
              <div className="hidden min-w-0 max-w-[7.5rem] flex-col items-end justify-center sm:flex sm:max-w-none">
                <PlayerNameWithBadge
                  name={auth.displayName || "Player"}
                  isAdmin={auth.isAdmin}
                  selectedBadge={null}
                  nameClassName="text-body-sm font-strong leading-heading text-content-primary transition-colors group-hover:text-content-primary"
                />
                {typeof auth.mmr === "number" ? (
                  <div className="mt-0.5 flex items-center">
                    <MmrDisplay
                      value={auth.mmr}
                      size="sm"
                      className="bg-transparent p-0 shadow-none"
                    />
                    <PlayerBadge
                      badge={auth.selectedBadge || null}
                      size="sm"
                      className="ml-1 hidden sm:inline-flex"
                    />
                  </div>
                ) : null}
              </div>
              <PlayerBadge
                badge={auth.selectedBadge || null}
                size="sm"
                className="sm:hidden"
              />
              <AvatarBadge
                avatarUrl={auth.avatarUrl}
                fallback={(auth.displayName || "P").slice(0, 1).toUpperCase()}
                alt={auth.displayName || "Player"}
                size="sm"
                className="h-9 w-9 border-border-strong bg-surface-raised transition-colors group-hover:border-content-primary/40 sm:h-[42px] sm:w-[42px]"
              />
            </Link>
          </div>
        ) : (
          <div className="pointer-events-auto justify-self-end">
            <Button type="button" variant="secondary" size="sm" onClick={authActions.openSignIn}>
              Sign In
            </Button>
          </div>
        )}
      </div>
    </header>
  );
}

function AppNavigation({
  activeRoute,
  onlinePlayers,
  tasks,
}: {
  activeRoute: AppNavRoute | null;
  onlinePlayers?: number;
  tasks: AppNavTask[];
}) {
  const reduceMotion = useReducedMotion();
  const hasTasks = tasks.length > 0;
  // The first client render must match SSR. Reading sessionStorage here made the
  // previously visited route active during hydration, moving the selection
  // span into a different link than the server rendered.
  const [visualRoute, setVisualRoute] = useState<AppNavRoute | null>(activeRoute);

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => {
      setVisualRoute(activeRoute);
      if (activeRoute) {
        window.sessionStorage.setItem(appNavRouteStorageKey, activeRoute);
      }
    });
    return () => window.cancelAnimationFrame(frame);
  }, [activeRoute]);

  const rememberCurrentRoute = () => {
    if (!activeRoute) return;
    try {
      window.sessionStorage.setItem(appNavRouteStorageKey, activeRoute);
    } catch {
      // Navigation still works when session storage is unavailable.
    }
  };

  return (
    <div
      className={cn(
        "app-shell-navigation pointer-events-none fixed inset-x-3 bottom-[max(0.75rem,env(safe-area-inset-bottom))] z-game-controls mx-auto flex max-w-2xl items-stretch justify-center gap-2 md:inset-x-0 md:bottom-auto md:top-5 md:max-w-none md:items-center md:gap-3",
        !hasTasks && "md:grid md:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)]",
      )}
    >
      <AppNavigationSurface
        aria-label="Primary navigation"
        className={cn(
          "pointer-events-auto grid min-w-0 flex-1 grid-cols-4 rounded-xl p-1.5 md:max-w-[520px] md:flex-none md:gap-1.5 md:p-2",
          !hasTasks && "md:col-start-2",
        )}
      >
        {APP_NAV_ITEMS.map((item) => (
          <AppNavLink
            key={item.route}
            item={item}
            active={item.route === visualRoute}
            reduceMotion={!!reduceMotion}
            compact={hasTasks}
            onNavigate={rememberCurrentRoute}
          />
        ))}
      </AppNavigationSurface>
      {hasTasks ? (
        <div className="min-w-0">
          <AppNavTasks tasks={tasks} />
        </div>
      ) : typeof onlinePlayers === "number" ? (
        <AppNavigationSurface
          as="div"
          aria-label={`${onlinePlayers} players online`}
          className="pointer-events-auto flex w-14 shrink-0 flex-col items-center justify-center gap-1 text-label font-semibold text-status-success md:col-start-3 md:min-h-[52px] md:w-auto md:justify-self-start md:flex-row md:justify-start md:gap-3 md:rounded-full md:px-5 md:text-body-sm"
        >
          <span className="status-dot-wrap relative flex h-4 w-4 shrink-0 items-center justify-center">
            <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-status-success" />
          </span>
          <span>{onlinePlayers.toLocaleString()}</span>
        </AppNavigationSurface>
      ) : null}
    </div>
  );
}

function AppNavLink({
  active,
  compact,
  item,
  onNavigate,
  reduceMotion,
}: {
  active: boolean;
  compact: boolean;
  item: (typeof APP_NAV_ITEMS)[number];
  onNavigate: () => void;
  reduceMotion: boolean;
}) {
  const Icon = item.icon;
  const social = useSyncExternalStore(socialStore.subscribe, socialStore.getState, socialStore.getState);
  const waiting = item.route === "friends" ? social.pending.length + social.friends.reduce((sum, friend) => sum + (friend.unread || 0), 0) : 0;
  const content = (
    <>
      {active ? (
        <motion.span
          layoutId="app-nav-selection"
          className="absolute inset-0 rounded-lg border border-border-strong bg-surface-fill"
          transition={
            reduceMotion
              ? { duration: 0 }
              : { type: "spring", stiffness: 380, damping: 34 }
          }
        />
      ) : null}
      <Icon
        size={18}
        fill={item.route === "play" && active ? "currentColor" : "none"}
        className="relative shrink-0"
        aria-hidden="true"
      />
      <motion.span
        className="relative overflow-hidden whitespace-nowrap"
        animate={{ width: compact ? 0 : "auto", opacity: compact ? 0 : 1 }}
        transition={{ duration: reduceMotion ? 0 : 0.18 }}
        aria-hidden={compact || undefined}
      >
        {item.label}
      </motion.span>
      {waiting ? (
        <span aria-label={`${waiting} waiting`} className="absolute right-1 top-1 rounded-full bg-status-danger px-1.5 text-caption font-strong text-content-on-action">
          {waiting}
        </span>
      ) : null}
    </>
  );
  const className = cn(
    "relative flex min-h-[52px] items-center justify-center rounded-lg text-label font-strong transition-colors md:min-h-9 md:flex-row md:text-body-sm",
    compact ? "gap-0 px-2 md:px-3" : "flex-col gap-1 md:flex-row md:gap-2.5 md:px-4",
    active ? "text-content-primary" : "text-content-secondary hover:text-content-primary",
  );

  return (
    <motion.div
      animate={active && !reduceMotion ? { scale: 1.025 } : { scale: 1 }}
      transition={{ type: "spring", stiffness: 380, damping: 34 }}
    >
      <Link
        href={item.href}
        onClick={onNavigate}
        className={className}
        aria-label={item.label}
        aria-current={active ? "page" : undefined}
      >
        {content}
      </Link>
    </motion.div>
  );
}
