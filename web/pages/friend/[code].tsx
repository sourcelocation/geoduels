import { useMutation, useQuery } from "@tanstack/react-query";
import Head from "next/head";
import { useRouter } from "next/router";
import { AppContentRail } from "../../features/app-shell/components/AppContentRail";
import { Button } from "../../components/ui/button";
import { AppPanel } from "../../components/ui/compositions";
import { PlayerRow } from "../../features/social/components/CompactPlayerRow";
import { useAuthState } from "../../features/auth/components/AuthProvider";
import { socialClient } from "../../features/social/lib/social-client";
import { AsyncState, PageHeader } from "../../components/ui/patterns";
import { useShowAppNoticeWhen } from "../../components/ui/AppNotice";

export default function FriendCodePage() {
  const router = useRouter();
  const code = String(router.query.code || "").trim().toUpperCase();
  const auth = useAuthState();
  const player = useQuery({
    queryKey: ["social", "friend-code", code],
    enabled: !!code && auth.isRegistered,
    queryFn: () => socialClient.resolveCode(auth.accessToken, code),
  });
  const request = useMutation({
    mutationFn: () => socialClient.requestByCode(auth.accessToken, code),
  });
  useShowAppNoticeWhen(request.isError, "Could not send the friend request. Try again.", request.failureCount);
  return (
    <>
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
              <Button variant="primary" className="mt-5 w-full" onClick={() => request.mutate()} disabled={request.isPending || request.isSuccess} loading={request.isPending}>
                {request.isSuccess ? "Request sent" : "Send friend request"}
              </Button>
            </>
          ) : <AsyncState className="mt-5" status="loading" message="Loading player" />}
        </AppPanel>
      </AppContentRail>
    </>
  );
}
