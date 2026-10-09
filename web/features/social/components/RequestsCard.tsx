import { useQuery } from "@tanstack/react-query";
import { ArrowUpRight, Check, X } from "lucide-react";
import Link from "next/link";
import { useShowAppNoticeWhen } from "../../../components/ui/AppNotice";
import { IconButton } from "../../../components/ui/button";
import { AppPanel } from "../../../components/ui/compositions";
import { InsetList } from "../../../components/ui/patterns";
import { CardTitle, Eyebrow, MutedText } from "../../../components/ui/typography";
import { useRuntimeConfig } from "../../../lib/runtime-config-context";
import { useFriendsPage } from "../hooks/useFriendsPage";
import { useRequestActions, type RequestAction } from "../hooks/useRequestActions";
import { socialClient } from "../lib/social-client";
import type { CompactPlayer } from "../types";
import { CompactPlayerRow } from "./CompactPlayerRow";

const SHOWN_REQUESTS = 3;

type PendingRequest = {
  key: string;
  player: CompactPlayer;
  meta: string;
  accept: RequestAction;
  decline: RequestAction;
  acceptLabel: string;
};

/** The friend requests and party invitations waiting for an answer, newest first. */
export function RequestsCard({ accessToken }: { accessToken: string }) {
  const config = useRuntimeConfig();
  const friends = useFriendsPage(accessToken);
  const invitations = useQuery({
    queryKey: ["social", "party-invitations"],
    queryFn: () => socialClient.partyInvitations(config, accessToken),
    refetchInterval: 60_000,
  });
  const answer = useRequestActions(accessToken);
  useShowAppNoticeWhen(answer.isError, "Could not answer the request. Try again.", answer.failureCount);

  const now = Date.now();
  const pending: Array<PendingRequest & { createdAt: string }> = [
    ...(invitations.data?.invitations || [])
      .filter((invitation) => Date.parse(invitation.expiresAt) > now)
      .map((invitation) => ({
        key: `party:${invitation.id}`,
        player: invitation.inviter,
        meta: `Party invite · ${invitation.mode.replaceAll("_", " ")} · ${invitation.memberCount} in party`,
        accept: { kind: "party" as const, invitationId: invitation.id, response: "accept" as const },
        decline: { kind: "party" as const, invitationId: invitation.id, response: "decline" as const },
        acceptLabel: "Join",
        createdAt: invitation.createdAt,
      })),
    ...(friends.data?.requests.incoming || []).map((request) => ({
      key: `friend:${request.id}`,
      player: request.player,
      meta: "Friend request",
      accept: { kind: "friend" as const, requestId: request.id, response: "accept" as const },
      decline: { kind: "friend" as const, requestId: request.id, response: "decline" as const },
      acceptLabel: "Accept",
      createdAt: request.createdAt,
    })),
  ].sort((left, right) => right.createdAt.localeCompare(left.createdAt));

  return (
    <AppPanel className="flex h-full min-w-0 flex-col rounded-2xl p-5 sm:p-6">
      <Eyebrow className="mb-1">Social</Eyebrow>
      <CardTitle>Requests</CardTitle>
      {pending.length ? (
        <InsetList className="mt-4">
          {pending.slice(0, SHOWN_REQUESTS).map((request) => (
            <CompactPlayerRow
              key={request.key}
              player={request.player}
              meta={request.meta}
              actions={
                <>
                  <IconButton
                    aria-label={`${request.acceptLabel}: ${request.player.displayName}`}
                    onClick={() => answer.mutate(request.accept)}
                    disabled={answer.isPending}
                  >
                    <Check size={15} />
                  </IconButton>
                  <IconButton
                    aria-label={`Decline: ${request.player.displayName}`}
                    onClick={() => answer.mutate(request.decline)}
                    disabled={answer.isPending}
                  >
                    <X size={15} />
                  </IconButton>
                </>
              }
            />
          ))}
        </InsetList>
      ) : (
        <MutedText className="mt-2">Friend requests and party invites show up here.</MutedText>
      )}
      <div className="mt-auto flex items-center justify-between gap-4 pt-4">
        <MutedText>{pending.length > SHOWN_REQUESTS ? `${pending.length - SHOWN_REQUESTS} more waiting` : ""}</MutedText>
        <Link
          href="/friends"
          className="inline-flex shrink-0 items-center gap-1 rounded-full border border-border-default bg-surface-fill px-3 py-2 text-label font-strong text-status-success transition hover:border-border-strong hover:bg-surface-grouped hover:text-content-primary"
        >
          Friends
          <ArrowUpRight size={13} />
        </Link>
      </div>
    </AppPanel>
  );
}
