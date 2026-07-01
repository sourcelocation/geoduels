import { Loader2, LogOut, Trash2 } from "lucide-react";
import { useState } from "react";
import AppModalShell from "../../../components/ui/AppModalShell";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import { Surface } from "../../../components/ui/Surface";
import { useAccountSettings } from "../hooks/use-account-settings";

export function AccountSettingsModal({
  onClose,
  profilePath,
}: {
  onClose: () => void;
  profilePath: string;
}) {
  const [deleteConfirmation, setDeleteConfirmation] = useState("");
  const state = useAccountSettings(profilePath);
  const { account, accountQuery, deleteMutation, unlinkMutation } = state;

  return (
    <AppModalShell
      title="Account settings"
      onClose={onClose}
      maxWidthClassName="max-w-xl"
    >
      {accountQuery.isLoading ? (
        <div className="flex min-h-48 items-center justify-center text-[#a9bfd4]">
          <Loader2 className="mr-2 h-5 w-5 animate-spin" />
          Loading account…
        </div>
      ) : accountQuery.isError ? (
        <p className="rounded-xl border border-red-400/20 bg-red-400/10 p-4 text-sm text-red-200">
          Account settings are temporarily unavailable.
        </p>
      ) : !account ? (
        <p className="py-8 text-center text-sm text-[#a9bfd4]">
          Sign in to manage account settings.
        </p>
      ) : (
        <div className="space-y-4">
          <Surface variant="subtle" className="rounded-xl p-4">
            <p className="text-[10px] font-black uppercase tracking-[0.14em] text-[#6b8b80]">
              Email
            </p>
            <p className="mt-1 truncate text-sm font-semibold text-white">
              {account.email}
            </p>
          </Surface>

          <Surface variant="subtle" className="rounded-xl p-4">
            <h3 className="text-sm font-black text-white">
              {account.isGuest ? "Save progress" : "Sign-in methods"}
            </h3>
            <div className="mt-3 space-y-2">
              {state.providers.map((provider) => {
                const linked = account.linkedProviders.includes(provider);
                return (
                  <div
                    key={provider}
                    className="flex items-center justify-between gap-3 rounded-xl border border-white/10 bg-black/15 p-3"
                  >
                    <div>
                      <p className="text-sm font-bold capitalize text-white">
                        {provider}
                      </p>
                      <p className="text-xs text-[#a9bfd4]">
                        {linked ? "Linked" : "Not linked"}
                      </p>
                    </div>
                    {linked ? (
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => unlinkMutation.mutate(provider)}
                        disabled={
                          unlinkMutation.isPending ||
                          account.linkedProviders.length <= 1
                        }
                      >
                        Unlink
                      </Button>
                    ) : (
                      <Button
                        size="sm"
                        onClick={() =>
                          void state.startProvider(provider)
                        }
                      >
                        Link
                      </Button>
                    )}
                  </div>
                );
              })}
            </div>
          </Surface>

          {state.error ? (
            <p className="rounded-xl border border-red-400/20 bg-red-400/10 p-3 text-sm text-red-200">
              {state.error}
            </p>
          ) : null}

          <Button
            type="button"
            onClick={() => void state.signOut()}
            className="min-h-11 w-full rounded-xl"
          >
            <LogOut className="h-4 w-4" />
            Sign out
          </Button>

          <Surface variant="danger" className="rounded-xl p-4">
            <h3 className="font-black text-red-100">Danger zone</h3>
            <p className="mt-1 text-xs leading-5 text-red-100/70">
              Deleting your account removes sign-in links and clears the public
              profile. Match and moderation records may be retained.
            </p>
            <Input
              variant="game"
              value={deleteConfirmation}
              onChange={(event) => setDeleteConfirmation(event.target.value)}
              placeholder="Type DELETE"
              className="mt-4 w-full border-red-300/20 bg-black/25 text-center font-bold tracking-[0.18em]"
            />
            <div className="mt-3">
              <Button
                type="button"
                variant="danger"
                disabled={
                  deleteConfirmation !== "DELETE" ||
                  deleteMutation.isPending
                }
                onClick={() => deleteMutation.mutate()}
                className="min-h-11 rounded-xl"
              >
                {deleteMutation.isPending ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Trash2 className="h-4 w-4" />
                )}
                Delete account
              </Button>
            </div>
          </Surface>
        </div>
      )}
    </AppModalShell>
  );
}
