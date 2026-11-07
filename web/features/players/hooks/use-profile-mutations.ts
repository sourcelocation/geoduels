import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  requestUpdateNickname,
  requestUpdateSelectedBadge,
} from "../../auth/lib/auth-client";
import { authGateway } from "../../auth/auth-gateway";

export function useProfileOwnerActions(accessToken: string) {
  const queryClient = useQueryClient();
  const refresh = () => Promise.all([
    authGateway.bootstrap({ force: true }),
    queryClient.invalidateQueries({ queryKey: ["player-profile"] }),
  ]);

  return {
    nicknameMutation: useMutation({
      mutationFn: (nickname: string) =>
        requestUpdateNickname(accessToken, nickname),
      onSuccess: refresh,
    }),
    badgeMutation: useMutation({
      mutationFn: (badgeId: string) =>
        requestUpdateSelectedBadge(accessToken, badgeId),
      onSuccess: refresh,
    }),
  };
}
