import type { ReactNode } from "react";
import { AnimatePresence, motion } from "framer-motion";
import { AppShell } from "../../app-shell/components/AppShell";
import { AppContentRail } from "../../app-shell/components/AppContentRail";
import type { AppNavRoute } from "../../app-shell/navigation";
import { CenteredSpinner } from "../../../components/ui/Spinner";
import type { LobbyContentRoute } from "../lib/lobby-ui";

const tabPanelMotion = {
  initial: { opacity: 0, y: 16, scale: 0.97 },
  animate: { opacity: 1, y: 0, scale: 1 },
  exit: { opacity: 0, y: 10, scale: 0.97 },
  transition: { duration: 0.22, ease: [0.16, 1, 0.3, 1] as const },
};

export type LobbyScreenViewProps = {
  activeNavRoute: AppNavRoute;
  backgroundBlurred: boolean;
  contentClassName?: string;
  contentRoute: LobbyContentRoute;
  routeLoading: boolean;
  maintenanceBanner: ReactNode;
  maintenanceOverlay: ReactNode;
  modalNodes: ReactNode;
  onlinePlayers: number;
  partyErrorNotice: ReactNode;
  showPartyPanel: boolean;
  partyPanel: ReactNode;
  mapRouteSurface: ReactNode;
  playPanel: ReactNode;
  leaderboardPanel: ReactNode;
  friendsDashboard: ReactNode;
  legalCard: ReactNode;
};

export function LobbyScreenView({
  activeNavRoute,
  backgroundBlurred,
  contentClassName,
  contentRoute,
  routeLoading,
  maintenanceBanner,
  maintenanceOverlay,
  modalNodes,
  onlinePlayers,
  partyErrorNotice,
  showPartyPanel,
  partyPanel,
  mapRouteSurface,
  playPanel,
  leaderboardPanel,
  friendsDashboard,
  legalCard,
}: LobbyScreenViewProps) {
  return (
    <>
      <AnimatePresence>{maintenanceOverlay}</AnimatePresence>
      <AnimatePresence>{modalNodes}</AnimatePresence>
      <AppShell
        activeNavRoute={activeNavRoute}
        backgroundBlurred={backgroundBlurred}
        contentClassName={contentClassName}
        maintenanceBanner={maintenanceBanner}
        onlinePlayers={onlinePlayers}
        viewportLocked={showPartyPanel}
      >
        <AppContentRail
          as="main"
          size="wide"
          className={`relative z-content flex flex-1 flex-col items-center justify-start pointer-events-none ${showPartyPanel ? "min-h-0 overflow-y-auto pb-4 pt-2 md:overflow-hidden sm:pb-6 sm:pt-3" : "pb-28 pt-4 sm:pb-12 sm:pt-8"}`}
        >
          {partyErrorNotice}

          <div
            className={`flex w-full justify-center ${
              showPartyPanel
                ? "items-start md:min-h-0 md:flex-1 md:items-stretch"
                : contentRoute === "play"
                  ? "md:min-h-[calc(100svh-21.25rem)] md:items-center"
                  : ""
            }`}
          >
            {routeLoading ? (
              <CenteredSpinner label="Loading page" className="min-h-[18rem]" />
            ) : (
              <AnimatePresence mode="popLayout">
                {showPartyPanel ? partyPanel : null}
                {!showPartyPanel && contentRoute === "play" ? playPanel : null}
                {!showPartyPanel && contentRoute === "top" ? (
                  <motion.div
                    key="top"
                    {...tabPanelMotion}
                    className="flex w-full justify-center pointer-events-auto"
                  >
                    {leaderboardPanel}
                  </motion.div>
                ) : null}
                {!showPartyPanel ? mapRouteSurface : null}
                {!showPartyPanel &&
                (contentRoute === "friends" || contentRoute === "party") ? (
                  <motion.div
                    key="friends"
                    {...tabPanelMotion}
                    className="flex w-full justify-center pointer-events-auto"
                  >
                    {friendsDashboard}
                  </motion.div>
                ) : null}
              </AnimatePresence>
            )}
          </div>

          {!routeLoading && !showPartyPanel && contentRoute === "play" ? (
            <div className="mt-10 w-full sm:mt-16">{legalCard}</div>
          ) : null}
        </AppContentRail>
      </AppShell>
    </>
  );
}
