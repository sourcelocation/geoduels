import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { ClipboardList, Shield } from "lucide-react";
import Link from "next/link";
import { useEffect, useState, type ReactNode } from "react";
import AvatarBadge from "../../../components/ui/AvatarBadge";
import { circularIconButtonClassName } from "../../../components/ui/button";
import { MmrDisplay } from "../../../components/ui/MmrDisplay";
import PlayerBadge, { type PlayerBadgeInfo } from "../../../components/ui/PlayerBadge";
import PlayerNameWithBadge from "../../../components/ui/PlayerNameWithBadge";
import { Tooltip } from "../../../components/ui/Tooltip";
import { cn } from "../../../lib/cn";
import {
  APP_NAV_ITEMS,
  appNavRouteStorageKey,
  isAppNavRoute,
  type AppNavRoute,
} from "../navigation";

const backgroundImage = "/bg3.v2.webp";
const backgroundPlaceholder = "/bg3.placeholder.v2.webp";
const backgroundOverlay =
  "linear-gradient(rgba(18, 56, 41, 0.4), rgba(0, 0, 0, 0.9))";

let backgroundLoaded = false;
let backgroundLoadPromise: Promise<void> | null = null;

function loadBackground() {
  if (backgroundLoaded) return Promise.resolve();
  if (backgroundLoadPromise) return backgroundLoadPromise;

  backgroundLoadPromise = new Promise<void>((resolve) => {
    const image = new Image();
    image.onload = async () => {
      try {
        await image.decode();
      } catch {
        // onload is sufficient when decode is unavailable.
      }
      backgroundLoaded = true;
      resolve();
    };
    image.onerror = () => {
      backgroundLoadPromise = null;
      resolve();
    };
    image.src = backgroundImage;
  });
  return backgroundLoadPromise;
}

export type AppShellViewer = {
  userId: string;
  displayName: string;
  avatarUrl?: string;
  avatarFallback: string;
  mmr?: number;
  selectedBadge?: PlayerBadgeInfo | null;
};

type AppShellProps = {
  activeNavRoute: AppNavRoute | null;
  children: ReactNode;
  navigationDisabled?: boolean;
  navigationHidden?: boolean;
  onlinePlayers?: number;
  viewer?: AppShellViewer | null;
  signedOutAction?: ReactNode;
  isAdmin?: boolean;
  isModerator?: boolean;
  maintenanceBanner?: ReactNode;
  contentClassName?: string;
};

export function AppShell({
  activeNavRoute,
  children,
  navigationDisabled = false,
  navigationHidden = false,
  onlinePlayers,
  viewer,
  signedOutAction,
  isAdmin = false,
  isModerator = false,
  maintenanceBanner,
  contentClassName,
}: AppShellProps) {
  const [highQualityBackgroundReady, setHighQualityBackgroundReady] =
    useState(backgroundLoaded);

  useEffect(() => {
    let cancelled = false;
    if (backgroundLoaded) {
      setHighQualityBackgroundReady(true);
      return;
    }
    const timer = window.setTimeout(() => {
      void loadBackground().then(() => {
        if (!cancelled && backgroundLoaded) {
          setHighQualityBackgroundReady(true);
        }
      });
    }, 350);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, []);

  return (
    <div className="relative flex min-h-screen flex-col overflow-hidden font-sans text-[#f4f9ff] selection:bg-accentPrimary/30">
      <AppBackground ready={highQualityBackgroundReady} />
      <AppShellHeader
        isAdmin={isAdmin}
        isModerator={isModerator}
        maintenanceBanner={maintenanceBanner}
        signedOutAction={signedOutAction}
        viewer={viewer}
      />
      {!navigationHidden ? (
        <AppNavigation
          activeRoute={activeNavRoute}
          disabled={navigationDisabled}
          onlinePlayers={onlinePlayers}
        />
      ) : null}
      <div
        className={cn(
          "relative z-10 flex min-h-0 flex-1 flex-col",
          contentClassName,
        )}
      >
        {children}
      </div>
    </div>
  );
}

function AppBackground({ ready }: { ready: boolean }) {
  return (
    <div className="pointer-events-none fixed inset-0 z-0 overflow-hidden">
      <div
        className="absolute inset-0"
        style={{
          backgroundImage: `${backgroundOverlay}, url('${backgroundPlaceholder}')`,
          backgroundPosition: "center",
          backgroundRepeat: "no-repeat",
          backgroundSize: "cover",
          filter: "blur(14px)",
          transform: "scale(1.06)",
        }}
      />
      <div
        className={cn(
          "absolute inset-0 transition-opacity duration-700 ease-out",
          ready ? "opacity-100" : "opacity-0",
        )}
        style={{
          backgroundImage: `${backgroundOverlay}, url('${backgroundImage}')`,
          backgroundPosition: "center",
          backgroundRepeat: "no-repeat",
          backgroundSize: "cover",
          transform: "scale(1.06)",
        }}
      />
    </div>
  );
}

function AppShellHeader({
  isAdmin,
  isModerator,
  maintenanceBanner,
  signedOutAction,
  viewer,
}: {
  isAdmin: boolean;
  isModerator: boolean;
  maintenanceBanner?: ReactNode;
  signedOutAction?: ReactNode;
  viewer?: AppShellViewer | null;
}) {
  return (
    <header className="sticky top-0 z-20 px-4 pb-4 pt-4 sm:px-6 sm:pb-5 sm:pt-5 lg:px-8 lg:pb-6 lg:pt-6">
      <AnimatePresence>{maintenanceBanner}</AnimatePresence>
      <div className="flex items-center justify-between gap-4">
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
          {isAdmin ? (
            <Tooltip content="Admin" side="bottom">
              <Link
                href="/admin"
                prefetch={false}
                aria-label="Admin"
                className={circularIconButtonClassName()}
              >
                <Shield size={17} aria-hidden="true" />
              </Link>
            </Tooltip>
          ) : null}
          {isAdmin || isModerator ? (
            <Tooltip content="Moderator" side="bottom">
              <Link
                href="/moderator"
                prefetch={false}
                aria-label="Moderator"
                className={circularIconButtonClassName()}
              >
                <ClipboardList size={17} aria-hidden="true" />
              </Link>
            </Tooltip>
          ) : null}
        </div>

        {viewer ? (
          <Link
            href={`/players/${encodeURIComponent(viewer.displayName || viewer.userId)}`}
            className="group flex min-w-0 cursor-pointer items-center justify-self-end gap-2.5 sm:gap-3"
          >
            <div className="hidden min-w-0 max-w-[7.5rem] flex-col items-end justify-center sm:flex sm:max-w-none">
              <PlayerNameWithBadge
                name={viewer.displayName || "Player"}
                isAdmin={isAdmin}
                selectedBadge={null}
                nameClassName="text-[12px] font-bold leading-tight text-white transition-colors group-hover:text-emerald-100 sm:text-[15px]"
              />
              {typeof viewer.mmr === "number" ? (
                <div className="mt-0.5 flex items-center">
                  <MmrDisplay
                    value={viewer.mmr}
                    size="sm"
                    className="bg-transparent p-0 shadow-none"
                  />
                  <PlayerBadge
                    badge={viewer.selectedBadge}
                    size="sm"
                    className="ml-1"
                  />
                </div>
              ) : null}
            </div>
            <AvatarBadge
              avatarUrl={viewer.avatarUrl}
              fallback={viewer.avatarFallback}
              alt={viewer.displayName || "Player"}
              size="sm"
              className="h-9 w-9 border-[1.5px] border-white/20 bg-[#162130] transition-colors group-hover:border-white/40 sm:h-[42px] sm:w-[42px]"
            />
          </Link>
        ) : (
          <div className="pointer-events-auto justify-self-end">
            {signedOutAction}
          </div>
        )}
      </div>
    </header>
  );
}

function AppNavigation({
  activeRoute,
  disabled,
  onlinePlayers,
}: {
  activeRoute: AppNavRoute | null;
  disabled: boolean;
  onlinePlayers?: number;
}) {
  const reduceMotion = useReducedMotion();
  const [visualRoute, setVisualRoute] = useState<AppNavRoute | null>(() => {
    if (activeRoute === null || typeof window === "undefined") return activeRoute;
    const stored = window.sessionStorage.getItem(appNavRouteStorageKey) || "";
    return isAppNavRoute(stored) ? stored : activeRoute;
  });

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
      className="pointer-events-none fixed inset-x-3 bottom-[max(0.75rem,env(safe-area-inset-bottom))] z-30 mx-auto flex max-w-[430px] items-stretch justify-center gap-2 md:inset-x-0 md:bottom-auto md:top-5 md:grid md:max-w-none md:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] md:items-center md:gap-3"
    >
      <nav
        aria-label="Primary navigation"
        className="glass-panel pointer-events-auto grid min-w-0 flex-1 grid-cols-4 rounded-[20px] p-1.5 md:col-start-2 md:max-w-[520px] md:flex-none md:gap-1.5 md:rounded-full md:p-2"
      >
        {APP_NAV_ITEMS.map((item) => (
          <AppNavLink
            key={item.route}
            item={item}
            active={item.route === visualRoute}
            disabled={disabled}
            reduceMotion={!!reduceMotion}
            onNavigate={rememberCurrentRoute}
          />
        ))}
      </nav>
      {typeof onlinePlayers === "number" ? (
        <div
          aria-label={`${onlinePlayers} players online`}
          className="glass-panel pointer-events-auto flex w-14 shrink-0 flex-col items-center justify-center gap-1 rounded-[20px] text-[11px] font-semibold text-[#2ad18f] md:col-start-3 md:min-h-[52px] md:w-auto md:justify-self-start md:flex-row md:justify-start md:gap-3 md:rounded-full md:px-5 md:text-xs"
        >
          <span className="status-dot-wrap relative flex h-4 w-4 shrink-0 items-center justify-center">
            <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-accentPrimary" />
          </span>
          <span>{onlinePlayers.toLocaleString()}</span>
        </div>
      ) : null}
    </div>
  );
}

function AppNavLink({
  active,
  disabled,
  item,
  onNavigate,
  reduceMotion,
}: {
  active: boolean;
  disabled: boolean;
  item: (typeof APP_NAV_ITEMS)[number];
  onNavigate: () => void;
  reduceMotion: boolean;
}) {
  const Icon = item.icon;
  const content = (
    <>
      {active ? (
        <motion.span
          layoutId="app-nav-selection"
          className="absolute inset-0 rounded-[14px] border border-white/15 bg-white/[0.12]"
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
      <span className="relative">{item.label}</span>
    </>
  );
  const className = cn(
    "relative flex min-h-[52px] flex-col items-center justify-center gap-1 rounded-[14px] text-[10px] font-extrabold transition-colors md:min-h-9 md:flex-row md:gap-2.5 md:px-4 md:text-[13px]",
    active ? "text-white" : "text-[#8fa7af] hover:text-white",
    disabled && "cursor-not-allowed opacity-45",
  );

  if (disabled) {
    return (
      <span className={className} aria-disabled="true">
        {content}
      </span>
    );
  }

  return (
    <motion.div
      animate={active && !reduceMotion ? { scale: 1.025 } : { scale: 1 }}
      transition={{ type: "spring", stiffness: 380, damping: 34 }}
    >
      <Link
        href={item.href}
        onClick={onNavigate}
        className={className}
        aria-current={active ? "page" : undefined}
      >
        {content}
      </Link>
    </motion.div>
  );
}
