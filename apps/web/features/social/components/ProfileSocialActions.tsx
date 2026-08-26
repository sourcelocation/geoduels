import { useQuery } from "@tanstack/react-query";
import { getRuntimeConfig } from "../../../lib/runtime-config";
import type { PublicPlayerProfile } from "../../players/types";
import { socialClient } from "../lib/social-client";
import { RelationshipActions } from "./RelationshipActions";
import { useAuthState } from "../../auth/components/AuthProvider";

export function ProfileSocialActions({
  profile,
}: {
  profile: PublicPlayerProfile;
}) {
  const auth = useAuthState();
  if (!auth.isRegistered || auth.userId === profile.userId) return null;
  return <ConnectedProfileSocialActions profile={profile} />;
}

function ConnectedProfileSocialActions({
  profile,
}: {
  profile: PublicPlayerProfile;
}) {
  const auth = useAuthState();
  const config = getRuntimeConfig();
  const relationship = useQuery({
    queryKey: ["relationship", profile.userId],
    enabled: auth.isRegistered && auth.userId !== profile.userId,
    queryFn: () => socialClient.relationship(config, auth.accessToken, profile.displayName),
  });
  if (!auth.isRegistered || auth.userId === profile.userId || !relationship.data) return null;
  const player = {
    userId: profile.userId,
    displayName: profile.displayName,
    avatarUrl: profile.avatarUrl,
    mmr: profile.mmr,
    selectedBadge: profile.selectedBadge,
    relationship: relationship.data.state,
    requestId: relationship.data.requestId,
  };
  return <RelationshipActions accessToken={auth.accessToken} player={player} />;
}
