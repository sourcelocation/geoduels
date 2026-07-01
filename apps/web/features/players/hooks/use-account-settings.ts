import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/router";
import { useState } from "react";
import { getRuntimeConfig } from "../../../lib/runtime-config";
import {
  requestDeleteAccount,
  requestDiscordStart,
  requestGoogleStart,
  requestLogout,
  requestMe,
  requestSession,
  requestUnlinkAuthProvider,
} from "../../auth/lib/auth-client";

type Provider = "google" | "discord";
type AccountData = {
  accessToken: string;
  email: string;
  isGuest: boolean;
  linkedProviders: string[];
};

export function useAccountSettings(profilePath: string) {
  const config = getRuntimeConfig();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [error, setError] = useState("");
  const accountQuery = useQuery({
    queryKey: ["profile-account-settings"],
    queryFn: async (): Promise<AccountData | null> => {
      const session = await requestSession(config);
      if (!session?.accessToken || !session.user?.id) return null;
      const response = await requestMe(config, session.accessToken);
      if (!response.ok) throw new Error("Failed to load account");
      const profile = await response.json();
      return {
        accessToken: session.accessToken,
        email: profile.email || session.user.email || "Guest account",
        isGuest: !!profile.isGuest,
        linkedProviders: session.linkedProviders || [],
      };
    },
    staleTime: 30_000,
  });
  const account = accountQuery.data;
  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ["profile-account-settings"] });
  const fail = (fallback: string) => (value: unknown) =>
    setError(value instanceof Error ? value.message : fallback);
  const unlinkMutation = useMutation({
    mutationFn: (provider: Provider) =>
      requestUnlinkAuthProvider(config, account?.accessToken || "", provider),
    onSuccess: refresh,
    onError: fail("Failed to unlink sign-in method"),
  });
  const deleteMutation = useMutation({
    mutationFn: () =>
      requestDeleteAccount(config, account?.accessToken || ""),
    onSuccess: () => exit(),
    onError: fail("Failed to delete account"),
  });
  const exit = async () => {
    queryClient.clear();
    await router.push("/");
  };
  const signOut = async () => {
    await requestLogout(config);
    await exit();
  };
  const startProvider = async (provider: Provider) => {
    if (!account) return;
    setError("");
    try {
      const request = provider === "google" ? requestGoogleStart : requestDiscordStart;
      const payload = await request(config, {
        accessToken: account.accessToken,
        intent: account.isGuest ? "upgrade_guest" : "link",
        returnTo: `${profilePath}?settings=account`,
      });
      if (payload.authURL) window.location.assign(payload.authURL);
    } catch (providerError) {
      fail("Failed to start sign-in method")(providerError);
    }
  };
  const providers = (["google", "discord"] as Provider[]).filter((provider) =>
    provider === "google" ? config.googleClientId : config.discordClientId,
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
