import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Button } from "../../../components/ui/button";
import { getRuntimeConfig } from "../../../lib/runtime-config";
import { socialClient } from "../lib/social-client";
import type { CompactPlayer, RelationshipState } from "../types";

export function RelationshipActions({
  accessToken,
  player,
  state = player.relationship,
  requestId = player.requestId,
}: {
  accessToken: string;
  player: CompactPlayer;
  state?: RelationshipState;
  requestId?: string;
}) {
  const queryClient = useQueryClient();
  const config = getRuntimeConfig();
  const mutation = useMutation({
    mutationFn: async (action: string) => {
      if (action === "add") return socialClient.sendRequest(config, accessToken, player.userId);
      if (action === "accept" || action === "decline" || action === "cancel") {
        if (!requestId) throw new Error("Request is no longer available");
        return socialClient.respondRequest(config, accessToken, requestId, action);
      }
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["social"] });
      void queryClient.invalidateQueries({ queryKey: ["relationship"] });
    },
  });

  if (!accessToken || state === "blocked_by_viewer") return null;
  if (state === "incoming_request") {
    return (
      <>
        <Button size="sm" onClick={() => mutation.mutate("accept")} disabled={mutation.isPending}>Accept</Button>
        <Button size="sm" variant="secondary" onClick={() => mutation.mutate("decline")} disabled={mutation.isPending}>Decline</Button>
      </>
    );
  }
  if (state === "outgoing_request") {
    return <Button size="sm" variant="secondary" onClick={() => mutation.mutate("cancel")} disabled={mutation.isPending}>Requested</Button>;
  }
  if (state === "friends") return null;
  return <Button size="sm" onClick={() => mutation.mutate("add")} disabled={mutation.isPending}>Add friend</Button>;
}
