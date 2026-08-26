import { createContext, useContext, useEffect, useMemo, useState, useSyncExternalStore, type ReactNode } from "react";
import { useRouter } from "next/router";
import { getRuntimeConfig } from "../../../lib/runtime-config";
import { formatQueueElapsed } from "../../lobby/lib/lobby-ui";
import { getHomeRuntime, startHomeRuntime } from "../../home/state/home-runtime";
import type { AppNavTask } from "./AppNavTasks";
import type { MatchState } from "../../matchmaking/controllers/match-controller";
import type { PartyRuntimeState } from "../../lobby/controllers/party-controller";

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
      label: elapsed ? `Finding a duel · ${elapsed}` : "Finding a duel…",
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
  const runtime = useMemo(() => getHomeRuntime(getRuntimeConfig()), []);
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
  const [nowMs, setNowMs] = useState(0);
  const isQueueing = match.matchmaking.status === "queueing";

  useEffect(() => {
    startHomeRuntime(runtime);
  }, [runtime]);

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
