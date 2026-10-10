import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/router";
import { useCallback, useEffect, useMemo, useRef, useSyncExternalStore } from "react";
import AppModalShell from "../../../components/ui/AppModalShell";
import { useAppNotice, useShowAppNoticeWhen } from "../../../components/ui/AppNotice";
import { Button } from "../../../components/ui/button";
import { cn } from "../../../lib/cn";
import { useRuntimeConfig } from "../../../lib/runtime-config-context";
import { getAuthGateway } from "../../auth/auth-gateway";
import { useAuthState } from "../../auth/components/AuthProvider";
import { markNotificationSeen, requestUnseenNotifications, type UserNotification } from "../../auth/lib/auth-client";
import { getHomeRuntime } from "../../home/state/home-runtime";
import { useRequestActions } from "../../social/hooks/useRequestActions";
import { presentNotification, type NotificationActions, type NotificationModal } from "../lib/kinds";
import { unseenNotificationsKey } from "../lib/unseen";

/**
 * Shows each notification once, on whichever device sees it first: a toast as it arrives, or a
 * modal, one at a time, that is seen once closed. Mounted once. Nothing covers a match or its
 * results; what arrives meanwhile waits until the player is back.
 */
export function Notifications() {
  const auth = useAuthState();
  if (!auth.isRegistered || !auth.accessToken || !auth.userId) return null;
  return <Presenter accessToken={auth.accessToken} userId={auth.userId} />;
}

function Presenter({ accessToken, userId }: { accessToken: string; userId: string }) {
  const config = useRuntimeConfig();
  const router = useRouter();
  const queryClient = useQueryClient();
  const { toast } = useAppNotice();
  const runtime = getHomeRuntime(config);
  const match = useSyncExternalStore(
    runtime.matchController.subscribe,
    runtime.matchController.getState.bind(runtime.matchController),
    runtime.matchController.getState.bind(runtime.matchController),
  );
  const waiting = !!match.snapshot;
  const unseen = useQuery({
    queryKey: unseenNotificationsKey(userId),
    queryFn: () => requestUnseenNotifications(config, accessToken),
    refetchOnWindowFocus: true,
  });
  const answer = useRequestActions(accessToken);
  useShowAppNoticeWhen(answer.isError, "Could not answer the request. Try again.", answer.failureCount);

  // Shown on this device, until the list stops returning them. A notification
  // written again keeps its id but not its createdAt, and is shown again.
  const shown = useRef(new Set<string>());
  const seen = useCallback((notification: UserNotification) => {
    shown.current.add(showing(notification));
    queryClient.setQueryData<UserNotification[]>(unseenNotificationsKey(userId), (items) => items?.filter((item) => item.id !== notification.id));
    void markNotificationSeen(config, accessToken, notification.id).catch(() => undefined);
  }, [accessToken, config, queryClient, userId]);

  const actions = useMemo<NotificationActions>(() => ({
    answer: answer.mutate,
    open: (href) => void router.push(href),
  }), [answer.mutate, router]);

  const items = unseen.data;
  useEffect(() => {
    if (waiting || !items) return;
    for (const notification of items) {
      if (shown.current.has(showing(notification))) continue;
      const presentation = presentNotification(notification, actions);
      if (presentation.as === "modal") continue;
      if (presentation.as === "toast") toast(presentation.toast);
      seen(notification);
    }
  }, [actions, items, seen, toast, waiting]);

  const next = waiting ? null : (items ?? [])
    .filter((notification) => !shown.current.has(showing(notification)))
    .map((notification) => ({ notification, presentation: presentNotification(notification, actions) }))
    .find((item) => item.presentation.as === "modal");
  if (!next || next.presentation.as !== "modal") return null;

  const { notification } = next;
  const modal = next.presentation.modal;
  const close = () => {
    seen(notification);
    if (modal.refreshesAccount) void getAuthGateway(config).bootstrap({ force: true }).catch(() => undefined);
  };
  return <NotificationModalView key={notification.id} modal={modal} onClose={close} />;
}

function showing(notification: UserNotification) {
  return `${notification.id}@${notification.createdAt}`;
}

function NotificationModalView({ modal, onClose }: { modal: NotificationModal; onClose: () => void }) {
  return (
    <AppModalShell
      title={modal.title}
      placement="center"
      showHeader={false}
      zIndexClassName="z-modal"
      maxWidthClassName="max-w-sm"
      onClose={onClose}
    >
      <p className={cn("text-label font-strong uppercase", modal.tone === "danger" ? "text-status-danger" : "text-status-success")}>
        {modal.eyebrow}
      </p>
      {modal.content}
      <Button type="button" variant="primary" size="lg" onClick={onClose} className="mt-5 w-full">
        {modal.confirm}
      </Button>
    </AppModalShell>
  );
}
