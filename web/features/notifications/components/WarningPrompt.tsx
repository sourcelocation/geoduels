import { useEffect, useState, useSyncExternalStore } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { getHomeRuntime } from "../../home/state/home-runtime";
import AppModalShell from "../../../components/ui/AppModalShell";
import { Button } from "../../../components/ui/button";
import { useRuntimeConfig } from "../../../lib/runtime-config-context";
import { useAuthState } from "../../auth/components/AuthProvider";
import { getAuthGateway } from "../../auth/auth-gateway";
import { warningRequest, type ModerationWarning } from "../lib/warnings-client";

export function WarningPrompt() {
  const auth = useAuthState();
  const config = useRuntimeConfig();
  const runtime = getHomeRuntime(config);
  const match = useSyncExternalStore(runtime.matchController.subscribe, runtime.matchController.getState.bind(runtime.matchController), runtime.matchController.getState.bind(runtime.matchController));
  const [deferred, setDeferred] = useState<number[]>([]);
  const query = useQuery({
    queryKey: ["moderation-warnings", auth.userId],
    enabled: !!auth.accessToken,
    queryFn: () => warningRequest<{ warnings: ModerationWarning[] }>(config, auth.accessToken, "me/warnings"),
    refetchInterval: 30_000,
    refetchOnWindowFocus: true,
  });
  const warning = query.data?.warnings.find(w => !w.acknowledgedAt);
  useEffect(() => { setDeferred([]); }, [auth.userId]);
  const acknowledge = useMutation({
    mutationFn: (id: number) => warningRequest(config, auth.accessToken, `me/warnings/${id}/acknowledge`, "POST"),
    onSuccess: async () => {
      // A reset nickname shows up in the session once it is reloaded.
      if (warning?.resetNickname) await getAuthGateway(config).bootstrap({ force: true }).catch(() => undefined);
      await query.refetch();
    },
  });
  // Warnings must not cover a match in progress. They remain pending after leaving it.
  const inMatch = !!match.snapshot && match.snapshot.state !== "ended";
  if (!warning || inMatch) return null;
  if (deferred.includes(warning.id)) return (
    <div className="fixed bottom-4 right-4 z-modal max-w-sm rounded-md border border-status-warning bg-surface-panel p-3 text-body-sm shadow-elev-2">
      <p>You have a moderation warning to acknowledge.</p>
      <Button variant="ghost" size="sm" onClick={() => setDeferred(ids => ids.filter(id => id !== warning.id))}>Review warning</Button>
    </div>
  );
  return (
    <AppModalShell title="Moderation warning" role="alertdialog" zIndexClassName="z-modal"
      onClose={acknowledge.isPending ? undefined : () => setDeferred(ids => [...ids, warning.id])}>
      <div className="space-y-4">
        <p className="whitespace-pre-wrap break-words text-body">{warning.message}</p>
        {warning.resetNickname ? (
          <p className="text-body-sm text-content-secondary">
            Your nickname was changed to {warning.resetNickname}. You can choose a different one from your profile.
          </p>
        ) : null}
        <Button disabled={acknowledge.isPending} onClick={() => acknowledge.mutate(warning.id)}>I understand</Button>
        {acknowledge.error ? <p role="alert" className="text-body-sm text-status-danger">{acknowledge.error.message}</p> : null}
      </div>
    </AppModalShell>
  );
}
