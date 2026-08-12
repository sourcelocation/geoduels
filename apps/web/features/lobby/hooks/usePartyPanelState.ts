import { useCallback, useEffect, useMemo, useRef } from "react";
import type { MatchConfig } from "../../matchmaking/lib/queue-client";
import type { PartyRuntimeStatus } from "../controllers/party-controller";
import type { PartySnapshot, PartyMode } from "../lib/party-client";

type PartyView = {
  status: PartyRuntimeStatus;
  snapshot: PartySnapshot | null;
  inviteCode: string;
  isMember: boolean;
  isOwner: boolean;
  busy: boolean;
  error: string;
};

type UsePartyPanelStateInput = {
  party: PartyView;
  userId: string;
  updateSettings: (config: MatchConfig, mode?: PartyMode) => Promise<void>;
  setInviteCopied: (copied: boolean) => void;
};

const pressureTimeLimitsMs = new Set([15000, 30000, 60000, 90000]);

const defaultPartyConfig: MatchConfig = {
  ruleset: "moving",
  roundTimerMode: "none",
  pressureTimeLimitMs: 15000,
  roundCount: 5,
  initialHp: 6000,
  multiplierStartRound: 3,
  multiplierIncrement: 0.5,
};

export function usePartyPanelState({
  party,
  userId,
  updateSettings,
  setInviteCopied,
}: UsePartyPanelStateInput) {
  const inviteResetTimer = useRef<number | null>(null);
  const inviteURL =
    typeof window !== "undefined" && party.inviteCode
      ? `${window.location.origin}/party/${party.inviteCode}`
      : "";
  const loading =
    !party.snapshot &&
    ["creating", "joining", "connecting", "reconnecting"].includes(party.status);
  const active = !!party.snapshot || party.status !== "idle";
  const members = party.snapshot?.members ?? [];
  const activeMatchId = party.snapshot?.activeMatchId || party.snapshot?.startedMatchId || "";
  const matchInProgress =
    party.snapshot?.state === "in_match" || party.snapshot?.state === "started";
  const config = party.snapshot?.config ?? defaultPartyConfig;
  const mode = party.snapshot?.mode || "duel";
  const clockOn = config.roundTimerMode === "fixed";
  const pressureOn =
    (typeof config.pressureTimeLimitMs === "number" && config.pressureTimeLimitMs > 0) ||
    config.roundTimerMode === "pressure";
  const roundSeconds = Math.round((config.roundTimeLimitMs || 45000) / 1000);
  const pressureSeconds = pressureOn ? Math.round((config.pressureTimeLimitMs || 15000) / 1000) : 0;
  const { currentMember, missingMembers, teamACount, teamBCount } = useMemo(() => {
    const missingMembers = [] as typeof members;
    let currentMember: (typeof members)[number] | undefined;
    let teamACount = 0;
    let teamBCount = 0;
    for (const member of members) {
      if (member.userId === userId) {
        currentMember = member;
      }
      if ((member.presenceStatus || (member.connected ? "online" : "offline")) !== "online") {
        missingMembers.push(member);
      }
      if ((member.teamId || "a") === "a") {
        teamACount += 1;
      } else if (member.teamId === "b") {
        teamBCount += 1;
      }
    }
    return { currentMember, missingMembers, teamACount, teamBCount };
  }, [members, userId]);
  const canStart =
    party.isOwner &&
    party.snapshot?.state === "open" &&
    ((mode === "duel" && members.length === 2) ||
      (mode === "team_duel" &&
        members.length >= 2 &&
        members.length <= 8 &&
        teamACount > 0 &&
        teamBCount > 0) ||
      (mode === "free_for_all" && members.length >= 2 && members.length <= 8)) &&
    missingMembers.length === 0;

  const saveConfig = useCallback((patch: MatchConfig) => {
    const next: MatchConfig = {
      ...config,
      ...patch,
    };
    if (next.roundTimerMode !== "fixed") {
      next.roundTimerMode = "none";
      next.roundTimeLimitMs = undefined;
    } else {
      next.roundTimeLimitMs = Math.max(10000, Math.min(120000, next.roundTimeLimitMs || 45000));
    }
    if (!pressureTimeLimitsMs.has(next.pressureTimeLimitMs || 0)) {
      next.pressureTimeLimitMs = undefined;
    }
    void updateSettings(next);
  }, [config, updateSettings]);

  const saveMode = useCallback((nextMode: PartyMode) => {
    void updateSettings(config, nextMode);
  }, [config, updateSettings]);

  const copyInvite = useCallback(() => {
    if (!inviteURL) return;
    void navigator.clipboard?.writeText(inviteURL);
    setInviteCopied(true);
    if (inviteResetTimer.current !== null) {
      window.clearTimeout(inviteResetTimer.current);
    }
    if (typeof window !== "undefined") {
      inviteResetTimer.current = window.setTimeout(() => {
        inviteResetTimer.current = null;
        setInviteCopied(false);
      }, 1600);
    }
  }, [inviteURL, setInviteCopied]);

  useEffect(() => () => {
    if (inviteResetTimer.current !== null) {
      window.clearTimeout(inviteResetTimer.current);
    }
  }, []);

  return {
    active,
    activeMatchId,
    canStart,
    clockOn,
    config,
    copyInvite,
    currentMember,
    inviteURL,
    loading,
    matchInProgress,
    members,
    missingMembers,
    mode,
    pressureOn,
    pressureSeconds,
    roundSeconds,
    saveConfig,
    saveMode,
    teamACount,
    teamBCount,
  };
}

export type PartyPanelState = ReturnType<typeof usePartyPanelState>;
