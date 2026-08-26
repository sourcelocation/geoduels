import type { MatchConfig } from "../../matchmaking/lib/queue-client";
import type { PartyRuntimeStatus } from "../controllers/party-controller";
import type { PartySnapshot, PartyMode } from "../lib/party-client";

type PartyView = {
  status: PartyRuntimeStatus;
  snapshot: PartySnapshot | null;
  inviteCode: string;
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
  multiplierMode: "shared",
};

export function usePartyPanelState({
  party,
  userId,
  updateSettings,
  setInviteCopied,
}: UsePartyPanelStateInput) {
  const inviteURL =
    typeof window !== "undefined" && party.inviteCode
      ? `${window.location.origin}/party/${party.inviteCode}`
      : "";
  const members = party.snapshot?.members || [];
  const activeMatchId = party.snapshot?.activeMatchId || party.snapshot?.startedMatchId || "";
  const matchInProgress =
    party.snapshot?.state === "in_match" || party.snapshot?.state === "started";
  const currentMember = members.find((member) => member.userId === userId);
  const config = party.snapshot?.config || defaultPartyConfig;
  const mode = party.snapshot?.mode || "duel";
  const clockOn = config.roundTimerMode === "fixed";
  const pressureOn =
    (typeof config.pressureTimeLimitMs === "number" && config.pressureTimeLimitMs > 0) ||
    config.roundTimerMode === "pressure";
  const roundSeconds = Math.round((config.roundTimeLimitMs || 45000) / 1000);
  const pressureSeconds = pressureOn ? Math.round((config.pressureTimeLimitMs || 15000) / 1000) : 0;
  const missingMembers = members.filter(
    (member) => (member.presenceStatus || (member.connected ? "online" : "offline")) !== "online",
  );
  const teamACount = members.filter((member) => (member.teamId || "a") === "a").length;
  const teamBCount = members.filter((member) => member.teamId === "b").length;
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

  const saveConfig = (patch: MatchConfig) => {
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
  };

  const saveMode = (nextMode: PartyMode) => {
    void updateSettings(config, nextMode);
  };

  const copyInvite = () => {
    if (!inviteURL) return;
    void navigator.clipboard?.writeText(inviteURL);
    setInviteCopied(true);
    if (typeof window !== "undefined") {
      window.setTimeout(() => setInviteCopied(false), 1600);
    }
  };

  return {
    activeMatchId,
    canStart,
    clockOn,
    config,
    copyInvite,
    currentMember,
    inviteURL,
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
