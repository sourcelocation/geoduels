import { useMutation, useQueryClient } from "@tanstack/react-query";
import { getRuntimeConfig } from "../../../lib/runtime-config";
import {
  requestUpdateNickname,
  requestUpdateSelectedBadge,
} from "../../auth/lib/auth-client";
import { getAuthGateway } from "../../auth/auth-gateway";

export function useProfileOwnerActions(accessToken: string) {
  const config = getRuntimeConfig();
  const queryClient = useQueryClient();
  const refresh = () => Promise.all([
    getAuthGateway(config).bootstrap({ force: true }),
    queryClient.invalidateQueries({ queryKey: ["player-profile"] }),
  ]);

  return {
    nicknameMutation: useMutation({
      mutationFn: (nickname: string) =>
        requestUpdateNickname(config, accessToken, nickname),
      onSuccess: refresh,
    }),
    badgeMutation: useMutation({
      mutationFn: (badgeId: string) =>
        requestUpdateSelectedBadge(config, accessToken, badgeId),
      onSuccess: refresh,
    }),
  };
}
