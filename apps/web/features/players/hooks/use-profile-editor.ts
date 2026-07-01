import { useEffect, useState } from "react";
import type { PublicPlayerProfile } from "../types";
import { useProfileOwnerActions } from "./use-profile-mutations";

export function useProfileEditor(
  profile: PublicPlayerProfile | undefined,
  accessToken: string,
  onNicknameSaved?: (nickname: string) => void,
) {
  const actions = useProfileOwnerActions(accessToken);
  const [editingName, setEditingName] = useState(false);
  const [nickname, setNickname] = useState(profile?.displayName || "");
  const [choosingBadge, setChoosingBadge] = useState(false);
  const [badgeId, setBadgeId] = useState(profile?.selectedBadge?.id || "");

  useEffect(() => {
    if (!profile) return;
    if (!editingName) setNickname(profile.displayName);
    if (!choosingBadge) setBadgeId(profile.selectedBadge?.id || "");
  }, [choosingBadge, editingName, profile]);

  const cancelName = () => {
    setNickname(profile?.displayName || "");
    setEditingName(false);
  };
  const saveName = () =>
    actions.nicknameMutation.mutate(nickname.trim(), {
      onSuccess: () => {
        setEditingName(false);
        onNicknameSaved?.(nickname.trim());
      },
    });
  const cancelBadge = () => {
    setBadgeId(profile?.selectedBadge?.id || "");
    setChoosingBadge(false);
  };
  const saveBadge = () =>
    actions.badgeMutation.mutate(badgeId, {
      onSuccess: () => setChoosingBadge(false),
    });

  return {
    ...actions,
    editingName,
    setEditingName,
    nickname,
    setNickname,
    cancelName,
    saveName,
    choosingBadge,
    setChoosingBadge,
    badgeId,
    setBadgeId,
    cancelBadge,
    saveBadge,
  };
}
