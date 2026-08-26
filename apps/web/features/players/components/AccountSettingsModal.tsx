import { LogOut, Trash2 } from "lucide-react";
import { Spinner } from "../../../components/ui/Spinner";
import { useState } from "react";
import AppModalShell from "../../../components/ui/AppModalShell";
import { Button } from "../../../components/ui/button";
import { DangerZoneDisclosure, SettingsGroup } from "../../../components/ui/compositions";
import { Field } from "../../../components/ui/Field";
import { Input } from "../../../components/ui/input";
import { AsyncState, EntityRow, Notice } from "../../../components/ui/patterns";
import { useAccountSettings } from "../hooks/use-account-settings";

export function AccountSettingsModal({
  onClose,
  profilePath,
}: {
  onClose: () => void;
  profilePath: string;
}) {
  return (
    <AppModalShell
      title="Account settings"
      onClose={onClose}
      maxWidthClassName="max-w-xl"
    >
      <AccountSettings profilePath={profilePath} />
    </AppModalShell>
  );
}

export function AccountSettings({ profilePath }: { profilePath: string }) {
  const [deleteConfirmation, setDeleteConfirmation] = useState("");
  const state = useAccountSettings(profilePath);
  const { account, accountQuery, deleteMutation, unlinkMutation } = state;

  return (
    <>
      {accountQuery.isLoading ? (
        <AsyncState status="loading" message="Loading account…" />
      ) : accountQuery.isError ? (
        <AsyncState status="error" message="Account settings are temporarily unavailable." />
      ) : !account ? (
        <AsyncState status="empty" message="Sign in to manage account settings." />
      ) : (
        <div className="space-y-5">
          <section aria-labelledby="account-details-heading">
            <h3 id="account-details-heading" className="mb-2 px-1 text-label font-strong text-content-secondary">Account</h3>
            <SettingsGroup>
              <EntityRow title="Email" description={account.email} />
              {state.providers.map((provider) => {
                const linked = account.linkedProviders.includes(provider);
                return (
                  <EntityRow
                    key={provider}
                    title={<span className="capitalize">{provider}</span>}
                    description={linked ? "Linked" : "Not linked"}
                    actions={linked ? (
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
                  />
                );
              })}
            </SettingsGroup>
            {account.isGuest ? <p className="mt-2 px-1 text-body-sm text-content-secondary">Link a sign-in method to keep your progress on every device.</p> : null}
          </section>

          {state.error ? (
            <Notice tone="danger">{state.error}</Notice>
          ) : null}

          <Button type="button" variant="secondary" onClick={() => void state.signOut()} className="min-h-11 w-full rounded-xl"><LogOut className="h-4 w-4" />Sign out</Button>

          <section aria-labelledby="danger-zone-heading">
            <h3 id="danger-zone-heading" className="mb-2 px-1 text-label font-strong text-content-secondary">Danger zone</h3>
            <DangerZoneDisclosure
              title="Delete account"
              description="Permanently remove your account and public profile."
            >
              <p className="mb-4 text-body-sm leading-label text-content-secondary">Deleting your account removes sign-in links and clears the public profile. Match and moderation records may be retained.</p>
              <Field
                label="Confirmation"
                htmlFor="delete-account-confirmation"
                helper="Type DELETE to enable account deletion."
              >
                <Input
                  id="delete-account-confirmation"
                  value={deleteConfirmation}
                  onChange={(event) => setDeleteConfirmation(event.target.value)}
                  placeholder="DELETE"
                  autoComplete="off"
                  spellCheck={false}
                  className="w-full"
                />
              </Field>
              <div className="mt-3">
                <Button
                type="button"
                variant="danger"
                disabled={
                  deleteConfirmation !== "DELETE" ||
                  deleteMutation.isPending
                }
                onClick={() => deleteMutation.mutate()}
              >
                {deleteMutation.isPending ? (
                  <Spinner size="sm" label="Deleting account" color="current" />
                ) : (
                  <Trash2 className="h-4 w-4" />
                )}
                Delete account
                </Button>
              </div>
            </DangerZoneDisclosure>
          </section>
        </div>
      )}
    </>
  );
}
