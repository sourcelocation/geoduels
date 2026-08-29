import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { Button } from "../../../components/ui/button";
import { getRuntimeConfig } from "../../../lib/runtime-config";
import { partyInviteCanResend, partyInviteResendInMs } from "../lib/party-invite";
import { socialClient } from "../lib/social-client";
import type { PartyInviteStatus } from "../types";

export function InviteToPartyButton({
  accessToken,
  partyId,
  userId,
  displayName,
  partyInvite,
  disabled,
}: {
  accessToken: string;
  partyId: string;
  userId: string;
  displayName: string;
  partyInvite?: PartyInviteStatus;
  disabled?: boolean;
}) {
  const queryClient = useQueryClient();
  const config = getRuntimeConfig();
  const [now, setNow] = useState(() => Date.now());
  const [optimisticCreatedAt, setOptimisticCreatedAt] = useState<string | undefined>();
  const sentAt = partyInvite?.createdAt || optimisticCreatedAt;
  const waiting = !!sentAt && !partyInviteCanResend(sentAt, now);
  useEffect(() => {
    if (!sentAt || partyInviteCanResend(sentAt)) return;
    const timer = window.setTimeout(() => setNow(Date.now()), partyInviteResendInMs(sentAt) + 25);
    return () => window.clearTimeout(timer);
  }, [sentAt]);
  const mutation = useMutation({
    mutationFn: () => socialClient.inviteToParty(config, accessToken, partyId, userId),
    onSuccess: (invitation) => {
      setOptimisticCreatedAt(invitation.createdAt || new Date().toISOString());
      setNow(Date.now());
      void queryClient.invalidateQueries({ queryKey: ["social"] });
    },
  });
  const sent = waiting;

  return (
    <Button
      type="button"
      size="sm"
      onClick={() => mutation.mutate()}
      disabled={disabled || mutation.isPending || waiting}
      aria-label={sent ? `Invite sent to ${displayName}` : `Invite ${displayName} to party`}
    >
      {sent ? "Invite Sent" : "Invite"}
    </Button>
  );
}
