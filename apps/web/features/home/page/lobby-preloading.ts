import type { NextRouter } from "next/router";
import { NAV_ITEMS, type LobbyContentRoute } from "../../lobby/lib/lobby-ui";

export function loadHomePageGame() {
  return import("./HomePageGame");
}

export function scheduleLobbyPreloading(
  router: Pick<NextRouter, "prefetch">,
  currentRoute: LobbyContentRoute,
) {
  // Warm the substantial gameplay chunk as soon as the lobby has mounted.
  // Unlike router.prefetch(), this also works in development.
  void loadHomePageGame().catch(() => {
    // The dynamic component will retry through Next.js if it is needed later.
  });

  const prefetchLobbyRoutes = () => {
    for (const item of NAV_ITEMS) {
      if (item.route !== currentRoute) {
        void router.prefetch(item.href).catch(() => {
          // Navigation can still load the route normally if speculative loading fails.
        });
      }
    }
  };

  if ("requestIdleCallback" in window) {
    const idleCallbackId = window.requestIdleCallback(prefetchLobbyRoutes, {
      timeout: 1_500,
    });
    return () => window.cancelIdleCallback(idleCallbackId);
  }

  const timeoutId = setTimeout(prefetchLobbyRoutes, 250);
  return () => clearTimeout(timeoutId);
}
