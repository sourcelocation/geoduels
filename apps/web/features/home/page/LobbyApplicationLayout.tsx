import type { ReactElement, ReactNode } from "react";
import { useCallback, useEffect, useRef } from "react";
import { useRouter } from "next/router";
import type { LobbyContentRoute } from "../../../components/ui/LobbyScreen";
import {
  normalizeEntityRouteId,
  toPublicEntityId,
} from "../../../lib/entity-id";
import { useHomeModel } from "../model/useHomeModel";
import HomePageView from "./HomePageView";
import { scheduleLobbyPreloading } from "./lobby-preloading";

function resolveLobbyRoute(pathname: string): LobbyContentRoute {
  if (pathname === "/friends") return "friends";
  if (pathname === "/top") return "top";
  if (pathname === "/maps/upload") return "map-upload";
  if (pathname === "/maps/[id]") return "map-details";
  if (pathname === "/maps") return "maps";
  if (pathname === "/party/[code]") return "party";
  return "play";
}

export function LobbyApplicationLayout({ children }: { children: ReactNode }) {
  const router = useRouter();
  const lobbyRoute = resolveLobbyRoute(router.pathname);
  const rawPartyCode =
    router.isReady && typeof router.query.code === "string"
      ? router.query.code
      : "";
  const partyInviteCode = rawPartyCode.trim().toUpperCase();
  const legacyLobbyCode =
    lobbyRoute === "play" &&
    router.isReady &&
    typeof router.query.lobby === "string"
      ? router.query.lobby.trim().toUpperCase()
      : "";
  const routedPartyCode =
    lobbyRoute === "party" && rawPartyCode.toLowerCase() !== "new"
      ? partyInviteCode
      : legacyLobbyCode;
  const mapId =
    lobbyRoute === "map-details" &&
    router.isReady &&
    typeof router.query.id === "string"
      ? normalizeEntityRouteId(router.query.id)
      : "";
  const prevMatchIdRef = useRef("");

  const handlePartyEntered = useCallback(
    (inviteCode: string) => {
      const nextPath = `/party/${encodeURIComponent(inviteCode)}`;
      if (router.asPath.split("?")[0] !== nextPath) {
        void router.replace(nextPath, undefined, { shallow: true });
      }
    },
    [router],
  );
  const handlePartyLeft = useCallback(() => {
    void router.push("/");
  }, [router]);

  const model = useHomeModel({
    routeContext: "home",
    partyInviteCode: routedPartyCode,
    onPartyEntered: handlePartyEntered,
    onPartyLeft: handlePartyLeft,
  });

  useEffect(
    () => scheduleLobbyPreloading(router, lobbyRoute),
    [lobbyRoute, router],
  );

  useEffect(() => {
    if (!router.isReady || lobbyRoute !== "party" || !rawPartyCode) return;
    if (rawPartyCode.toLowerCase() === "new") {
      void router.replace("/");
      return;
    }
    if (rawPartyCode !== partyInviteCode) {
      void router.replace(
        `/party/${encodeURIComponent(partyInviteCode)}`,
        undefined,
        { shallow: true },
      );
    }
  }, [lobbyRoute, partyInviteCode, rawPartyCode, router]);

  useEffect(() => {
    const nextMatchId = model.view.meta.activeMatchId;
    const prevMatchId = prevMatchIdRef.current;
    prevMatchIdRef.current = nextMatchId;
    if (!nextMatchId || nextMatchId === prevMatchId) return;
    void router.push(
      `/match/${encodeURIComponent(toPublicEntityId(nextMatchId))}`,
    );
  }, [model.view.meta.activeMatchId, router]);

  return (
    <>
      {children}
      <HomePageView model={model} lobbyRoute={lobbyRoute} mapId={mapId} />
    </>
  );
}

export function getLobbyLayout(page: ReactElement) {
  return <LobbyApplicationLayout>{page}</LobbyApplicationLayout>;
}
