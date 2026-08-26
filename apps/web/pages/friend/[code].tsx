import { useMutation, useQuery } from "@tanstack/react-query";
import Head from "next/head";
import { useRouter } from "next/router";
import { AppContentRail } from "../../features/app-shell/components/AppContentRail";
import { AppShell } from "../../features/app-shell/components/AppShell";
import { Button } from "../../components/ui/button";
import { AppPanel } from "../../components/ui/compositions";
import { PlayerRow } from "../../features/social/components/CompactPlayerRow";
import { useAuthState } from "../../features/auth/components/AuthProvider";
import { socialClient } from "../../features/social/lib/social-client";
import { getRuntimeConfig } from "../../lib/runtime-config";
import { AsyncState, Notice, PageHeader } from "../../components/ui/patterns";

export default function FriendCodePage() {
  const router = useRouter();
  const code = String(router.query.code || "").trim().toUpperCase();
  const auth = useAuthState();
  const config = getRuntimeConfig();
  const player = useQuery({
    queryKey: ["social", "friend-code", code],
    enabled: !!code && auth.isRegistered,
    queryFn: () => socialClient.resolveCode(config, auth.accessToken, code),
  });
  const request = useMutation({
    mutationFn: () => socialClient.requestByCode(config, auth.accessToken, code),
  });
  return (
    <AppShell activeNavRoute="friends">
      <Head><title>Add a friend | GeoDuels</title><meta name="robots" content="noindex" /></Head>
      <AppContentRail as="main" size="compact" className="relative z-content py-10">
        <AppPanel className="rounded-2xl p-6">
          <PageHeader eyebrow="Social" title="Add a friend" description="Use a player’s private friend link to send a request." />
          {!auth.isRegistered ? (
            <AsyncState className="mt-5" status="empty" message="Sign in with a registered account, then return to this link to send the request." />
          ) : player.isError ? (
            <AsyncState className="mt-5" status="error" message="This friend link is invalid or has expired." onRetry={() => void player.refetch()} />
          ) : player.data ? (
            <>
              <div className="mt-5"><PlayerRow player={player.data} /></div>
              <Button variant="primary" className="mt-5 w-full" onClick={() => request.mutate()} disabled={request.isPending || request.isSuccess}>
                {request.isSuccess ? "Request sent" : "Send friend request"}
              </Button>
              {request.isError ? <Notice tone="danger" className="mt-3">Could not send the friend request. Try again.</Notice> : null}
            </>
          ) : <AsyncState className="mt-5" status="loading" message="Loading player" />}
        </AppPanel>
      </AppContentRail>
    </AppShell>
  );
}
