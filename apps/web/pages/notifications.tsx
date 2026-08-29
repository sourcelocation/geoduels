import { useInfiniteQuery, useMutation, useQueryClient, type InfiniteData } from "@tanstack/react-query";
import Head from "next/head";
import { useMemo } from "react";
import { AppContentRail } from "../features/app-shell/components/AppContentRail";
import { AppShell } from "../features/app-shell/components/AppShell";
import { NotificationGroup, NotificationItem, type NotificationFeedEntry } from "../features/notifications/components/NotificationItem";
import { useNotificationActions } from "../features/notifications/hooks/useNotificationActions";
import { useAuthState } from "../features/auth/components/AuthProvider";
import { AsyncState, Notice, PageHeader } from "../components/ui/patterns";
import { Button } from "../components/ui/button";
import { AppPanel } from "../components/ui/compositions";
import { requestUserNotifications, markAllUserNotificationsRead, markUserNotificationRead, type UserNotification } from "../features/auth/lib/auth-client";
import { getRuntimeConfig } from "../lib/runtime-config";

const PAGE_SIZE = 30;
type NotificationInbox = InfiniteData<{ notifications: UserNotification[] }, number | undefined>;

function dateGroup(date: string) {
  const day = new Date(date);
  const today = new Date();
  const startToday = new Date(today.getFullYear(), today.getMonth(), today.getDate());
  const startYesterday = new Date(startToday); startYesterday.setDate(startYesterday.getDate() - 1);
  if (day >= startToday) return "Today";
  if (day >= startYesterday) return "Yesterday";
  return new Intl.DateTimeFormat(undefined, { month: "long", day: "numeric", year: day.getFullYear() === today.getFullYear() ? undefined : "numeric" }).format(day);
}

export default function NotificationsPage() {
  const auth = useAuthState();
  const config = getRuntimeConfig();
  const queryClient = useQueryClient();
  const inbox = useInfiniteQuery({
    queryKey: ["notifications", "inbox"],
    enabled: auth.isRegistered,
    initialPageParam: undefined as number | undefined,
    queryFn: ({ pageParam }) => requestUserNotifications(config, auth.accessToken, "all", { limit: PAGE_SIZE, beforeId: pageParam }),
    getNextPageParam: (lastPage) => lastPage.notifications.length === PAGE_SIZE ? lastPage.notifications[lastPage.notifications.length - 1]?.id : undefined,
  });
  const notifications = inbox.data?.pages.flatMap((page) => page.notifications) || [];
  const notificationAction = useNotificationActions(auth.isRegistered ? auth.accessToken : undefined);
  const groups = useMemo(() => notifications.reduce<Record<string, UserNotification[]>>((all, item) => {
    const group = dateGroup(item.createdAt);
    (all[group] ||= []).push(item);
    return all;
  }, {}), [notifications]);
  const readAll = useMutation({
    mutationFn: () => markAllUserNotificationsRead(config, auth.accessToken),
    onMutate: async () => {
      await queryClient.cancelQueries({ queryKey: ["notifications"] });
      const previous = queryClient.getQueriesData<NotificationInbox>({ queryKey: ["notifications"] });
      queryClient.setQueriesData<NotificationInbox>({ queryKey: ["notifications"] }, (current) => current ? { ...current, pages: current.pages.map((page) => ({ ...page, notifications: page.notifications.map((item) => ({ ...item, readAt: new Date().toISOString() })) })) } : current);
      return { previous };
    },
    onError: (_, __, context) => {
      context?.previous.forEach(([key, value]) => queryClient.setQueryData(key, value));
    },
    onSettled: () => { void queryClient.invalidateQueries({ queryKey: ["notifications"] }); },
  });
  const read = useMutation({
    mutationFn: (id: number) => markUserNotificationRead(config, auth.accessToken, id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: ["notifications"] });
      const previous = queryClient.getQueriesData<NotificationInbox>({ queryKey: ["notifications"] });
      queryClient.setQueriesData<NotificationInbox>({ queryKey: ["notifications"] }, (current) => current ? { ...current, pages: current.pages.map((page) => ({ ...page, notifications: page.notifications.map((item) => item.id === id ? { ...item, readAt: new Date().toISOString() } : item) })) } : current);
      return { previous };
    },
    onError: (_, __, context) => {
      context?.previous.forEach(([key, value]) => queryClient.setQueryData(key, value));
    },
    onSettled: () => { void queryClient.invalidateQueries({ queryKey: ["notifications"] }); },
  });
  const unreadCount = notifications.filter((item) => !item.readAt).length;
  return (
    <AppShell activeNavRoute={null}>
      <Head><title>Notifications | GeoDuels</title></Head>
      <AppContentRail as="main" size="standard" className="relative z-content pb-20 pt-6">
        <AppPanel className="grid gap-6 p-5 sm:p-6">
          <PageHeader eyebrow="Activity" title="Notifications" description="Requests and account activity in one place." actions={unreadCount ? <Button type="button" variant="secondary" size="sm" onClick={() => readAll.mutate()} disabled={readAll.isPending}>Mark all read</Button> : undefined} />
          {readAll.isError || read.isError || notificationAction.isError ? <Notice tone="danger">A notification could not be updated. Try again.</Notice> : null}
          {inbox.isLoading ? <AsyncState status="loading" message="Loading notification history" /> : null}
          {inbox.isError ? <AsyncState status="error" message="Notification history could not be loaded." onRetry={() => void inbox.refetch()} /> : null}
          {!inbox.isLoading && !inbox.isError && !notifications.length ? <AsyncState status="empty" title="You’re all caught up" message="New activity will appear here." /> : null}
          {!inbox.isLoading && !inbox.isError ? <div className="grid gap-5">{Object.entries(groups).map(([title, items]) => <NotificationGroup title={title} key={title}>{items.map((item) => {
            const entry = { kind: "notification", notification: item } as NotificationFeedEntry;
            return <NotificationItem key={item.id} entry={entry} onOpen={() => { if (!item.readAt) read.mutate(item.id); }} onAction={(action) => notificationAction.mutate(action)} actionPending={notificationAction.isPending} />;
          })}</NotificationGroup>)}</div> : null}
          {inbox.hasNextPage ? <Button type="button" variant="secondary" onClick={() => void inbox.fetchNextPage()} disabled={inbox.isFetchingNextPage}>{inbox.isFetchingNextPage ? "Loading older notifications…" : "Load older notifications"}</Button> : null}
        </AppPanel>
      </AppContentRail>
    </AppShell>
  );
}
