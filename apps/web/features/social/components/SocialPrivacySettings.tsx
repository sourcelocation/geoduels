import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { InsetList, Notice, SettingRow } from "../../../components/ui/patterns";
import { Switch } from "../../../components/ui/Switch";
import { CenteredSpinner } from "../../../components/ui/Spinner";
import { getRuntimeConfig } from "../../../lib/runtime-config";
import { socialClient } from "../lib/social-client";
import { useAuthState } from "../../auth/components/AuthProvider";

export function SocialPrivacySettings() {
  const auth = useAuthState();
  const config = getRuntimeConfig();
  const queryClient = useQueryClient();
  const enabled = auth.isRegistered;
  const settings = useQuery({
    queryKey: ["social", "settings"],
    enabled,
    queryFn: () => socialClient.settings(config, auth.accessToken),
  });
  const save = useMutation({
    mutationFn: (value: NonNullable<typeof settings.data>) =>
      socialClient.updateSettings(config, auth.accessToken, value),
    onMutate: async (value) => {
      await queryClient.cancelQueries({ queryKey: ["social", "settings"] });
      const previous = queryClient.getQueryData(["social", "settings"]);
      queryClient.setQueryData(["social", "settings"], value);
      return { previous };
    },
    onError: (_, __, context) => queryClient.setQueryData(["social", "settings"], context?.previous),
    onSettled: () => void queryClient.invalidateQueries({ queryKey: ["social", "settings"] }),
  });

  if (!enabled) {
    return <p className="text-body-sm text-content-secondary">Sign in with a registered account to manage social privacy.</p>;
  }
  if (settings.isLoading) {
    return <CenteredSpinner label="Loading social privacy settings" />;
  }
  if (!settings.data) {
    return <Notice tone="danger">Social privacy settings could not be loaded.</Notice>;
  }

  const update = (key: keyof typeof settings.data, checked: boolean) =>
    save.mutate({ ...settings.data!, [key]: checked });
  return (
    <>
      {save.isError ? <Notice tone="danger" className="mb-3">Your privacy change was not saved. Try again.</Notice> : null}
      <InsetList>
        <SocialPrivacyRow title="Appear in player search" description="Players can find your profile by display name." checked={settings.data.discoverable} disabled={save.isPending} onChange={(checked) => update("discoverable", checked)} />
        <SocialPrivacyRow title="Show presence and last seen" description="Friends can see whether you are online or when you last played." checked={settings.data.presenceVisible} disabled={save.isPending} onChange={(checked) => update("presenceVisible", checked)} />
        <SocialPrivacyRow title="Allow friend requests" description="Other players can send you a friend request." checked={settings.data.requestsEnabled} disabled={save.isPending} onChange={(checked) => update("requestsEnabled", checked)} />
        <SocialPrivacyRow title="Allow party invitations" description="Friends can invite you to their party." checked={settings.data.partyInvitesEnabled} disabled={save.isPending} onChange={(checked) => update("partyInvitesEnabled", checked)} />
      </InsetList>
    </>
  );
}

function SocialPrivacyRow(props: { title: string; description: string; checked: boolean; disabled: boolean; onChange: (checked: boolean) => void }) {
  return <SettingRow title={props.title} info={props.description} control={<Switch checked={props.checked} onCheckedChange={props.onChange} disabled={props.disabled} aria-label={props.title} />} />;
}
