import { useMutation, useQueryClient } from "@tanstack/react-query";
import { getRuntimeConfig } from "../../../lib/runtime-config";
import {
  requestUpdateNickname,
  requestUpdateSelectedBadge,
} from "../../auth/lib/auth-client";

export function useProfileOwnerActions(accessToken: string) {
  const config = getRuntimeConfig();
  const queryClient = useQueryClient();
  const refresh = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: ["player-profile"] }),
      queryClient.invalidateQueries({ queryKey: ["auth", "profile"] }),
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
