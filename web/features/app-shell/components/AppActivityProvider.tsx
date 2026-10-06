import { createContext, useContext, useEffect, useMemo, useRef, useState, useSyncExternalStore, type ReactNode } from "react";
import { useRouter } from "next/router";
import { useRuntimeConfig } from "../../../lib/runtime-config-context";
import { formatQueueElapsed } from "../../lobby/lib/lobby-ui";
import { getHomeRuntime, startHomeRuntime } from "../../home/state/home-runtime";
import type { AppNavTask } from "./AppNavTasks";
import type { MatchState } from "../../matchmaking/controllers/match-controller";
import type { PartyRuntimeState } from "../../lobby/controllers/party-controller";
import { useAuthState } from "../../auth/components/AuthProvider";
import { normalizeEntityRouteId, toPublicEntityId } from "../../../lib/entity-id";

const AppActivityContext = createContext<AppNavTask[]>([]);

export function deriveAppActivities({
  party,
  match,
  pathname,
  nowMs,
  cancelQueue,
}: {
  party: PartyRuntimeState;
  match: MatchState;
  pathname: string;
  nowMs: number;
  cancelQueue: () => void;
}): AppNavTask[] {
  const tasks: AppNavTask[] = [];
  if (match.matchmaking.status === "queueing") {
    const elapsed = formatQueueElapsed(
      match.matchmaking.queueStartedAt && nowMs
        ? nowMs - match.matchmaking.queueStartedAt
        : 0,
    );
    tasks.push({
      kind: "queue",
      label: elapsed ? `${elapsed}` : "0:00",
      onCancel: cancelQueue,
    });
  }
  const inviteCode = party.inviteCode || party.snapshot?.inviteCode || "";
  const hasActiveParty =
    !!party.snapshot &&
    !!party.self &&
    (party.status === "ready" || party.status === "reconnecting");
  if (hasActiveParty && pathname !== "/party/[code]") {
    tasks.push({
      kind: "party",
      label: inviteCode ? `In party — ${inviteCode}` : "In party",
      href: `/party/${encodeURIComponent(inviteCode)}`,
    });
  }
  return tasks;
}

export function AppActivityProvider({ children }: { children: ReactNode }) {
  const router = useRouter();
  const config = useRuntimeConfig();
  const auth = useAuthState();
  const runtime = useMemo(() => getHomeRuntime(config), [config]);
  const party = useSyncExternalStore(
    runtime.partyController.subscribe,
    runtime.partyController.getState.bind(runtime.partyController),
    runtime.partyController.getState.bind(runtime.partyController),
  );
  const match = useSyncExternalStore(
    runtime.matchController.subscribe,
    runtime.matchController.getState.bind(runtime.matchController),
    runtime.matchController.getState.bind(runtime.matchController),
  );
  const restoredParty = useRef("");
  const previousUserId = useRef(auth.userId);
  const navigatedMatch = useRef("");
  const [navigationAttempt, setNavigationAttempt] = useState(0);
  const [nowMs, setNowMs] = useState(0);
  const isQueueing = match.matchmaking.status === "queueing";

  useEffect(() => {
    startHomeRuntime(runtime);
  }, [runtime]);

  useEffect(() => {
    if (previousUserId.current !== auth.userId) {
      if (previousUserId.current) runtime.partyController.reset();
      previousUserId.current = auth.userId;
      restoredParty.current = "";
      navigatedMatch.current = "";
    }
    const currentParty = auth.bootstrap?.activity.currentParty;
    if (!auth.userId || auth.bootstrap?.auth?.user?.id !== auth.userId || !currentParty) return;
    const key = `${auth.userId}:${currentParty.id}`;
    if (restoredParty.current === key) return;
    restoredParty.current = key;
    void runtime.partyController.restoreParty(currentParty);
  }, [auth.userId, auth.bootstrap, runtime]);

  useEffect(() => {
    const matchId = party.launchMatchId;
    if (!matchId) {
      navigatedMatch.current = "";
      return;
    }
    if (!router.isReady || navigatedMatch.current === matchId) return;
    const routedId = typeof router.query.id === "string" ? normalizeEntityRouteId(router.query.id) : "";
    if (router.pathname === "/match/[id]" && routedId === matchId) {
      navigatedMatch.current = matchId;
      return;
    }

    let active = true;
    let retryTimer: number | undefined;
    const retry = () => {
      if (!active) return;
      // Next also resolves cancelled transitions with false. Keep the launch
      // pending when another page navigation interrupts this one.
      retryTimer = window.setTimeout(() => setNavigationAttempt((attempt) => attempt + 1), 300);
    };
    void router.replace(`/match/${encodeURIComponent(toPublicEntityId(matchId))}`).then((completed) => {
      if (!active) return;
      if (completed) navigatedMatch.current = matchId;
      else retry();
    }).catch(retry);

    return () => {
      active = false;
      if (retryTimer !== undefined) window.clearTimeout(retryTimer);
    };
  }, [auth.userId, party.launchMatchId, router, router.isReady, router.pathname, router.query.id, navigationAttempt]);

  useEffect(() => {
    if (!isQueueing) return;
    setNowMs(Date.now());
    const timer = window.setInterval(() => setNowMs(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [isQueueing]);

  const tasks = useMemo(
    () => deriveAppActivities({
      party,
      match,
      pathname: router.pathname,
      nowMs,
      cancelQueue: runtime.matchController.cancelQueue,
    }),
    [match, nowMs, party, router.pathname, runtime],
  );

  return <AppActivityContext.Provider value={tasks}>{children}</AppActivityContext.Provider>;
}

export function useAppActivities() {
  return useContext(AppActivityContext);
}
