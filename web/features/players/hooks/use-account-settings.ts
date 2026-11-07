import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/router";
import { useState } from "react";
import {
  requestDeleteAccount,
  requestDiscordStart,
  requestGoogleStart,
  requestUnlinkAuthProvider,
} from "../../auth/lib/auth-client";
import { useAuthState } from "../../auth/components/AuthProvider";
import { authGateway } from "../../auth/auth-gateway";

type Provider = "google" | "discord";
type AccountData = {
  accessToken: string;
  email: string;
  isGuest: boolean;
  linkedProviders: string[];
};

export function useAccountSettings(profilePath: string) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const auth = useAuthState();
  const [error, setError] = useState("");
  const accountQuery = useQuery({
    queryKey: ["profile-account-settings", auth.userId || "anonymous"],
    enabled: auth.status !== "bootstrapping" && !!auth.accessToken,
    queryFn: async (): Promise<AccountData | null> => {
      if (!auth.accessToken || !auth.userId) return null;
      return {
        accessToken: auth.accessToken,
        email: auth.bootstrap?.viewer?.email || auth.email || "Guest account",
        isGuest: auth.bootstrap?.viewer?.accountType === "guest",
        linkedProviders: auth.bootstrap?.viewer?.linkedProviders || auth.session?.linkedProviders || [],
      };
    },
    staleTime: 30_000,
  });
  const account = accountQuery.data;
  const refresh = async () => {
    await authGateway.bootstrap({ force: true });
    await queryClient.invalidateQueries({ queryKey: ["profile-account-settings", auth.userId || "anonymous"] });
  };
  const fail = (fallback: string) => (value: unknown) =>
    setError(value instanceof Error ? value.message : fallback);
  const unlinkMutation = useMutation({
    mutationFn: (provider: Provider) =>
      requestUnlinkAuthProvider(account?.accessToken || "", provider),
    onSuccess: refresh,
    onError: fail("Failed to unlink sign-in method"),
  });
  const deleteMutation = useMutation({
    mutationFn: () =>
      requestDeleteAccount(account?.accessToken || ""),
    onSuccess: async () => {
      await authGateway.clear();
      await exit();
    },
    onError: fail("Failed to delete account"),
  });
  const exit = async () => {
    queryClient.removeQueries({ queryKey: ["profile-account-settings", auth.userId || "anonymous"] });
    await router.push("/");
  };
  const signOut = async () => {
    await authGateway.logout();
    await exit();
  };
  const startProvider = async (provider: Provider) => {
    if (!account) return;
    setError("");
    try {
      const request = provider === "google" ? requestGoogleStart : requestDiscordStart;
      const payload = await request({
        accessToken: account.accessToken,
        intent: account.isGuest ? "upgrade_guest" : "link",
        returnTo: `${profilePath}?settings=account`,
      });
      if (payload.authURL) window.location.assign(payload.authURL);
    } catch (providerError) {
      fail("Failed to start sign-in method")(providerError);
    }
  };
  const { googleClientId, discordClientId } = authGateway.getClientKeys();
  const providers = (["google", "discord"] as Provider[]).filter((provider) =>
    provider === "google" ? googleClientId : discordClientId,
  );

  return {
    accountQuery,
    account,
    providers,
    error,
    unlinkMutation,
    deleteMutation,
    startProvider,
    signOut,
  };
}
