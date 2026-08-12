import { forwardRef, useState } from "react";
import { ChevronDown, ChevronUp, Copy, Crown, Loader2, LogOut, Map as MapIcon, Play, SlidersHorizontal, UserMinus, UserPlus, Users, Link2, Sparkles } from "lucide-react";
import { motion } from "framer-motion";
import { toPublicEntityId } from "../../../lib/entity-id";
import PlayerProfileLink from "../../../components/ui/PlayerProfileLink";
import type { MatchConfig, GameRuleset } from "../../matchmaking/lib/queue-client";
import type { PartyRuntimeStatus } from "../controllers/party-controller";
import type { PartySnapshot, PartyTeamId, PartyMode } from "../lib/party-client";
import {
  CLOCK_OPTIONS,
  PRESSURE_OPTIONS,
  lobbyTeamLabel,
  lobbyTeamPillClass,
  lobbyTeamTextClass,
} from "../lib/lobby-ui";
import type { PartyPanelState } from "../hooks/usePartyPanelState";
import {
  LobbyActionButton,
  LobbyActionLink,
  LobbyFieldLabel,
  LobbyInset,
  LobbyLoadingState,
  LobbyPanel,
  LobbyPill,
} from "./lobby-primitives";

type PartyView = {
  status: PartyRuntimeStatus;
  snapshot: PartySnapshot | null;
  inviteCode: string;
  isMember: boolean;
  isOwner: boolean;
  busy: boolean;
  error: string;
};

type PartyPanelProps = {
  authError: string;
  authLoading: boolean;
  inviteCopied: boolean;
  party: PartyView;
  setMapPickerOpen: (open: boolean) => void;
  state: PartyPanelState;
  userId: string;
  joinParty: (inviteCode?: string) => Promise<boolean>;
  leaveParty: () => Promise<void>;
  kickPartyMember: (userId: string) => Promise<void>;
  transferPartyOwner: (userId: string) => Promise<void>;
  startParty: () => Promise<void>;
  switchPartyTeam: (teamId: PartyTeamId) => Promise<void>;
};

const panelMotion = {
  initial: { opacity: 0, y: 20 },
  animate: { opacity: 1, y: 0 },
  exit: { opacity: 0, y: -14 },
  transition: { duration: 0.22, ease: "easeOut" },
} as const;

export const PartyPanel = forwardRef<HTMLDivElement, PartyPanelProps>(function PartyPanel({
  authError,
  authLoading,
  inviteCopied,
  joinParty,
  kickPartyMember,
  leaveParty,
  party,
  setMapPickerOpen,
  startParty,
  state,
  switchPartyTeam,
  transferPartyOwner,
  userId,
}, ref) {
  const {
    activeMatchId,
    canStart,
    clockOn,
    config,
    copyInvite,
    currentMember,
    loading,
    matchInProgress,
    members,
    mode,
    pressureOn,
    pressureSeconds,
    roundSeconds,
    saveConfig,
    saveMode,
  } = state;

  return (
    <motion.div
      ref={ref}
      key="party"
      {...panelMotion}
      className="w-full max-w-[1240px] pointer-events-auto"
    >
      <LobbyPanel radius="3xl" density="none" className="overflow-hidden">
        <div className="relative min-h-[220px] p-5 sm:p-7">
          <div className="absolute inset-0 pointer-events-none bg-[linear-gradient(180deg,rgba(42,209,143,0.16)_0%,rgba(10,23,26,0.74)_100%)]" />
          <div className="relative z-10 flex flex-col gap-5">
            <PartyHeader
              inviteCopied={inviteCopied}
              matchInProgress={matchInProgress}
              onCopyInvite={copyInvite}
              onLeave={() => void leaveParty()}
              party={party}
            />

            {matchInProgress ? (
              <LobbyInset tone="subtle" className="border-[#77f0be]/20 bg-[#22d385]/10">
                <p className="text-[11px] font-black uppercase tracking-[0.14em] text-[#77f0be]">
                  Game In Progress
                </p>
                <p className="mt-1 text-sm font-semibold text-[#d8f7e9]">
                  {!party.isMember
                    ? "Join the party now and you will be ready for the next game."
                    : currentMember?.inActiveMatch
                      ? "You are part of this game and can reconnect whenever you are ready."
                      : "You joined after this game started and will be able to play in the next one."}
                </p>
                {currentMember?.inActiveMatch && activeMatchId ? (
                  <LobbyActionLink
                    href={`/match/${encodeURIComponent(toPublicEntityId(activeMatchId))}`}
                    size="md"
                    className="mt-3"
                  >
                    <Play size={16} fill="currentColor" />
                    Reconnect to Game
                  </LobbyActionLink>
                ) : null}
              </LobbyInset>
            ) : null}

            {party.snapshot && party.isOwner && party.snapshot.state === "open" && !matchInProgress ? (
              <HostPartyLayout
                busy={party.busy}
                canStart={canStart}
                config={config}
                inviteCode={party.inviteCode}
                inviteCopied={inviteCopied}
                isOwner={party.isOwner}
                members={members}
                mode={mode}
                onCopyInvite={copyInvite}
                setMapPickerOpen={setMapPickerOpen}
                snapshot={party.snapshot}
                startParty={startParty}
                state={state}
                switchPartyTeam={switchPartyTeam}
                transferPartyOwner={transferPartyOwner}
                kickPartyMember={kickPartyMember}
                userId={userId}
              />
            ) : party.snapshot ? (
              <PartySettings
                busy={party.busy}
                clockOn={clockOn}
                config={config}
                isOwner={party.isOwner}
                mode={mode}
                pressureOn={pressureOn}
                pressureSeconds={pressureSeconds}
                roundSeconds={roundSeconds}
                saveConfig={saveConfig}
                saveMode={saveMode}
                setMapPickerOpen={setMapPickerOpen}
                snapshot={party.snapshot}
              />
            ) : null}

            {loading ? (
              <LobbyInset>
                <LobbyLoadingState className="min-h-[64px] justify-center">
                  <Loader2 className="animate-spin text-[#77f0be]" size={18} />
                  Connecting to party
                </LobbyLoadingState>
              </LobbyInset>
            ) : !party.isMember ? (
              <LobbyInset>
                <LobbyActionButton
                  type="button"
                  onClick={() => void joinParty()}
                  disabled={party.busy || authLoading}
                  size="lg"
                  className="w-full"
                >
                  {party.busy ? <Loader2 className="animate-spin" size={18} /> : <UserPlus size={18} />}
                  Join Party
                </LobbyActionButton>
                {authError ? (
                  <p className="mt-3 text-center text-xs font-semibold text-red-300">{authError}</p>
                ) : null}
              </LobbyInset>
            ) : null}

            {party.snapshot && !(party.isOwner && party.snapshot.state === "open" && !matchInProgress) ? (
              <PartyMemberList
                busy={party.busy}
                isOwner={party.isOwner}
                members={members}
                mode={mode}
                snapshot={party.snapshot}
                switchPartyTeam={switchPartyTeam}
                transferPartyOwner={transferPartyOwner}
                kickPartyMember={kickPartyMember}
                userId={userId}
              />
            ) : null}

            {party.isOwner && party.snapshot?.state === "open" && !matchInProgress && !(party.snapshot && party.isOwner) ? (
              <LobbyActionButton
                type="button"
                onClick={() => void startParty()}
                disabled={!canStart || party.busy}
                size="lg"
                className="w-full"
              >
                {party.busy ? <Loader2 className="animate-spin" size={18} /> : <Play size={18} fill="currentColor" />}
                {mode === "team_duel" ? "Start Team Duel" : mode === "free_for_all" ? "Start FFA" : "Start Duel"}
              </LobbyActionButton>
            ) : party.isMember && party.snapshot?.state === "open" && !party.isOwner ? (
              <LobbyInset className="text-center text-sm font-semibold text-[#a9bfd4]">
                Waiting for the leader to start.
              </LobbyInset>
            ) : null}
          </div>
        </div>
      </LobbyPanel>
    </motion.div>
  );
});

function PartyHeader({
  inviteCopied,
  matchInProgress,
  onCopyInvite,
  onLeave,
  party,
}: {
  inviteCopied: boolean;
  matchInProgress: boolean;
  onCopyInvite: () => void;
  onLeave: () => void;
  party: PartyView;
}) {
  return (
    <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
      <div>
        <span className="mb-2 block text-[12px] font-black uppercase tracking-[0.16em] text-[#77f0be]">
          CUSTOM
        </span>
        <h2 className="text-[34px] font-black leading-tight tracking-tight text-white sm:text-[42px]">
          Private Party
        </h2>
        <p className="mt-2 max-w-[48ch] text-[14px] leading-6 text-[#a9bfd4]">
          {matchInProgress
            ? "A game is in progress. Friends can still join the party and wait for the next one."
            : "Share the invite, wait for one opponent, then the leader starts the match."}
        </p>
      </div>
      <div className="flex flex-wrap gap-2">
        {party.inviteCode ? (
          <LobbyActionButton type="button" variant="secondary" onClick={onCopyInvite}>
            <Copy className="text-[#77f0be]" size={16} />
            {inviteCopied ? "Copied" : "Copy Invite"}
          </LobbyActionButton>
        ) : null}
        {party.isMember && !(party.isOwner && matchInProgress) ? (
          <LobbyActionButton
            type="button"
            variant="secondary"
            onClick={onLeave}
            disabled={party.busy}
          >
            <LogOut size={16} />
            Leave
          </LobbyActionButton>
        ) : null}
      </div>
    </div>
  );
}

function HostPartyLayout({
  busy,
  canStart,
  config,
  inviteCode,
  inviteCopied,
  isOwner,
  kickPartyMember,
  members,
  mode,
  onCopyInvite,
  setMapPickerOpen,
  snapshot,
  startParty,
  state,
  switchPartyTeam,
  transferPartyOwner,
  userId,
}: {
  busy: boolean;
  canStart: boolean;
  config: MatchConfig;
  inviteCode: string;
  inviteCopied: boolean;
  isOwner: boolean;
  kickPartyMember: (userId: string) => Promise<void>;
  members: PartySnapshot["members"];
  mode: PartyMode;
  onCopyInvite: () => void;
  setMapPickerOpen: (open: boolean) => void;
  snapshot: PartySnapshot;
  startParty: () => Promise<void>;
  state: PartyPanelState;
  switchPartyTeam: (teamId: PartyTeamId) => Promise<void>;
  transferPartyOwner: (userId: string) => Promise<void>;
  userId: string;
}) {
  const memberCount = members.length;
  const readyToLaunch = canStart && memberCount > 1;

  return (
    <div className="grid gap-4 lg:grid-cols-[minmax(0,1.35fr)_minmax(360px,0.85fr)]">
      <LobbyInset density="lg" className="border-white/10 bg-[#071714]/55">
        <div className="mb-5 flex items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-[#77f0be]">
              <SlidersHorizontal size={15} />
              <p className="text-[11px] font-black uppercase tracking-[0.16em]">Match setup</p>
            </div>
            <h3 className="mt-1 text-xl font-black tracking-tight text-white">Shape the next duel</h3>
            <p className="mt-1 text-xs leading-5 text-[#8fa9a1]">Choose the map and rules before you invite everyone in.</p>
          </div>
          <LobbyPill tone="success">Host controls</LobbyPill>
        </div>
        <PartySettings
          busy={busy}
          clockOn={state.clockOn}
          config={config}
          isOwner={isOwner}
          mode={mode}
          pressureOn={state.pressureOn}
          pressureSeconds={state.pressureSeconds}
          roundSeconds={state.roundSeconds}
          saveConfig={state.saveConfig}
          saveMode={state.saveMode}
          setMapPickerOpen={setMapPickerOpen}
          snapshot={snapshot}
        />
      </LobbyInset>

      <LobbyInset density="lg" className="flex flex-col border-[#77f0be]/20 bg-[#0b2420]/70">
        <div className="flex items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-[#77f0be]">
              <Link2 size={15} />
              <p className="text-[11px] font-black uppercase tracking-[0.16em]">Your invite</p>
            </div>
            <h3 className="mt-1 text-xl font-black tracking-tight text-white">Bring your people in</h3>
          </div>
          <Sparkles className="text-[#77f0be]" size={20} />
        </div>
        <div className="mt-5 rounded-2xl border border-white/10 bg-black/20 px-4 py-4">
          <p className="text-[10px] font-black uppercase tracking-[0.16em] text-[#6b8b80]">Party code</p>
          <div className="mt-2 flex items-center justify-between gap-3">
            <p className="font-mono text-3xl font-black tracking-[0.2em] text-white">{inviteCode || "------"}</p>
            <LobbyActionButton type="button" variant="secondary" size="sm" onClick={onCopyInvite} disabled={!inviteCode}>
              <Copy size={14} className="text-[#77f0be]" />
              {inviteCopied ? "Copied" : "Copy"}
            </LobbyActionButton>
          </div>
          <p className="mt-2 text-xs leading-5 text-[#8fa9a1]">Share the code with friends. They can join while you keep setup in your hands.</p>
        </div>

        <div className="mt-5 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Users size={15} className="text-[#77f0be]" />
            <p className="text-[11px] font-black uppercase tracking-[0.14em] text-[#a9bfd4]">Players</p>
          </div>
          <LobbyPill tone={memberCount > 1 ? "success" : "warning"}>{memberCount} joined</LobbyPill>
        </div>
        <div className="mt-3 grid gap-2">
          <PartyMemberList
            busy={busy}
            isOwner={isOwner}
            members={members}
            mode={mode}
            snapshot={snapshot}
            switchPartyTeam={switchPartyTeam}
            transferPartyOwner={transferPartyOwner}
            kickPartyMember={kickPartyMember}
            userId={userId}
          />
        </div>

        <div className="mt-5 border-t border-white/10 pt-4">
          <LobbyActionButton type="button" onClick={() => void startParty()} disabled={!readyToLaunch || busy} size="lg" className="w-full">
            {busy ? <Loader2 className="animate-spin" size={18} /> : <Play size={18} fill="currentColor" />}
            {mode === "team_duel" ? "Start Team Duel" : mode === "free_for_all" ? "Start FFA" : "Start Duel"}
          </LobbyActionButton>
          <p className="mt-2 text-center text-[11px] font-semibold text-[#78958b]">
            {readyToLaunch ? "Everyone is ready to play." : "Invite at least one more player to launch."}
          </p>
        </div>
      </LobbyInset>
    </div>
  );
}

function PartySettings({
  busy,
  clockOn,
  config,
  isOwner,
  mode,
  pressureOn,
  pressureSeconds,
  roundSeconds,
  saveConfig,
  saveMode,
  setMapPickerOpen,
  snapshot,
}: {
  busy: boolean;
  clockOn: boolean;
  config: MatchConfig;
  isOwner: boolean;
  mode: PartyMode;
  pressureOn: boolean;
  pressureSeconds: number;
  roundSeconds: number;
  saveConfig: (patch: MatchConfig) => void;
  saveMode: (mode: PartyMode) => void;
  setMapPickerOpen: (open: boolean) => void;
  snapshot: PartySnapshot;
}) {
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [perPlayerHpEnabled, setPerPlayerHpEnabled] = useState(
    config.playerHpOverrides && Object.keys(config.playerHpOverrides).length > 0
  );
  const [playerHpDraft, setPlayerHpDraft] = useState<Record<string, number>>(() => ({
    ...(config.playerHpOverrides || {}),
  }));
  const rulesetLabel =
    config.ruleset === "nmpz"
      ? "NMPZ"
      : config.ruleset === "no_move"
        ? "No Move"
        : "Moving";

  const initialHp = config.initialHp || 6000;
  const roundCount = config.roundCount || 5;
  const startRound = config.multiplierStartRound || 3;
  const increment = config.multiplierIncrement || 0.5;
  const hasCustomAdvanced = roundCount !== 5 || initialHp !== 6000 || startRound !== 3 || increment !== 0.5;

  if (!(isOwner && snapshot.state === "open")) {
    return (
      <LobbyInset>
        <p className="text-[11px] font-bold uppercase tracking-[0.14em] text-[#6b8b80]">
          Game Settings
        </p>
        <p className="mt-1 text-sm font-semibold text-white">
          {config.mapName} · {rulesetLabel} ·{" "}
          {clockOn ? `${roundSeconds}s clock` : "Infinite clock"} ·{" "}
          {pressureOn ? `${pressureSeconds}s pressure` : "No pressure"} ·{" "}
          {roundCount} rounds · {initialHp.toLocaleString()} HP · Multipliers start R{startRound} (+{increment}x)
        </p>
      </LobbyInset>
    );
  }

  return (
    <LobbyInset>
      <div className="flex flex-col gap-3">
        <div className="grid w-full gap-2 sm:grid-cols-3">
          <label className="grid gap-1.5">
            <LobbyFieldLabel>Mode</LobbyFieldLabel>
            <PartyChoiceMenu
              label="Mode"
              value={mode}
              options={[
                { value: "duel", label: "Duel" },
                { value: "team_duel", label: "Team Duel" },
                { value: "free_for_all", label: "Free For All" },
              ]}
              onChange={(value) => saveMode(value as PartyMode)}
              disabled={busy}
            />
          </label>
          <label className="grid gap-1.5">
            <LobbyFieldLabel>Map</LobbyFieldLabel>
            <LobbyActionButton
              type="button"
              variant="secondary"
              size="md"
              onClick={() => setMapPickerOpen(true)}
              disabled={busy}
              className="h-10 min-h-10 w-full min-w-0 justify-center rounded-xl normal-case tracking-normal"
            >
              <MapIcon className="shrink-0 text-[#77f0be]" size={14} />
              <span className="truncate">{config.mapName}</span>
            </LobbyActionButton>
          </label>
          <label className="grid gap-1.5">
            <LobbyFieldLabel>Rules</LobbyFieldLabel>
            <PartyChoiceMenu
              label="Rules"
              value={config.ruleset || "moving"}
              options={[
                { value: "moving", label: "Moving" },
                { value: "no_move", label: "No Move" },
                { value: "nmpz", label: "NMPZ" },
              ]}
              onChange={(value) => saveConfig({ ruleset: value as GameRuleset })}
              disabled={busy}
            />
          </label>
        </div>

        <div className="grid w-full gap-2 border-t border-white/10 pt-3 sm:grid-cols-2">
          <label className="grid gap-1.5">
            <LobbyFieldLabel>Clock</LobbyFieldLabel>
            <PartyOptionSlider
              value={clockOn ? String(roundSeconds) : "infinite"}
              options={CLOCK_OPTIONS.map((option) => ({ ...option, shortLabel: option.value === "infinite" ? "∞" : option.label }))}
              onChange={(value) => saveConfig(
                value === "infinite"
                  ? { roundTimerMode: "none", roundTimeLimitMs: undefined }
                  : { roundTimerMode: "fixed", roundTimeLimitMs: Number(value) * 1000 },
              )}
              disabled={busy}
            />
          </label>
          <label className="grid gap-1.5">
            <LobbyFieldLabel>Pressure</LobbyFieldLabel>
            <PartyOptionSlider
              value={pressureOn ? String(pressureSeconds) : "none"}
              options={PRESSURE_OPTIONS.map((option) => ({ ...option, shortLabel: option.label }))}
              onChange={(value) => saveConfig({ pressureTimeLimitMs: value === "none" ? undefined : Number(value) * 1000 })}
              disabled={busy}
            />
          </label>
        </div>

        <div className="border-t border-white/10 pt-2 flex flex-col gap-2">
          <button
            type="button"
            onClick={() => setShowAdvanced((prev) => !prev)}
            className="inline-flex items-center gap-2 text-[12px] font-extrabold uppercase tracking-[0.14em] text-[#77f0be] hover:text-white transition cursor-pointer self-start"
          >
            <SlidersHorizontal size={14} />
            <span>Advanced Settings</span>
            {hasCustomAdvanced ? (
              <span className="h-2 w-2 rounded-full bg-[#77f0be] animate-pulse" />
            ) : null}
            {showAdvanced ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
          </button>

          {showAdvanced && (
            <>
              <div className="grid w-full gap-2 pt-1 sm:grid-cols-2 xl:grid-cols-4">
                <label className="grid gap-1.5">
                  <LobbyFieldLabel>Number of Rounds</LobbyFieldLabel>
                  <PartyOptionSlider
                    value={String(roundCount)}
                    options={Array.from({ length: 10 }, (_, index) => {
                      const value = index + 1;
                      return { value: String(value), label: `${value} ${value === 1 ? "round" : "rounds"}`, shortLabel: String(value) };
                    })}
                    onChange={(value) => saveConfig({ roundCount: Number(value) })}
                    disabled={busy}
                  />
                </label>
                <label className="grid gap-1.5">
                  <LobbyFieldLabel>Starting HP</LobbyFieldLabel>
                <PartyOptionSlider
                  value={String(initialHp)}
                  options={[
                    { value: "1", label: "1 HP (One-Shot)", shortLabel: "1" },
                    { value: "500", label: "500 HP", shortLabel: "500" },
                    { value: "1000", label: "1,000 HP", shortLabel: "1k" },
                    { value: "3000", label: "3,000 HP", shortLabel: "3k" },
                    { value: "6000", label: "6,000 HP (Standard)", shortLabel: "6k" },
                    { value: "10000", label: "10,000 HP", shortLabel: "10k" },
                    { value: "15000", label: "15,000 HP", shortLabel: "15k" },
                    { value: "20000", label: "20,000 HP", shortLabel: "20k" },
                    { value: "50000", label: "50,000 HP", shortLabel: "50k" },
                  ]}
                  onChange={(value) => saveConfig({ initialHp: Number(value) })}
                  disabled={busy}
                />
              </label>

              <label className="grid gap-1.5">
                <LobbyFieldLabel>Multipliers Start</LobbyFieldLabel>
                <PartyOptionSlider
                  value={String(startRound)}
                  options={[1, 2, 3, 4, 5, 6, 8, 10].map((value) => ({ value: String(value), label: `Round ${value}`, shortLabel: `R${value}` }))}
                  onChange={(value) => saveConfig({ multiplierStartRound: Number(value) })}
                  disabled={busy}
                />
              </label>

              <label className="grid gap-1.5">
                <LobbyFieldLabel>Multiplier Step</LobbyFieldLabel>
                <PartyOptionSlider
                  value={String(increment)}
                  options={[0.1, 0.25, 0.5, 1, 2].map((value) => ({ value: String(value), label: `+${value.toFixed(2).replace(/0$/, "")}x / round`, shortLabel: `+${value}x` }))}
                  onChange={(value) => saveConfig({ multiplierIncrement: Number(value) })}
                  disabled={busy}
                />
              </label>
            </div>
            
            <div className="mt-3 border-t border-white/10 pt-3">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <p className="text-[12px] font-extrabold uppercase tracking-[0.1em] text-[#a9bfd4]">Per-Player HP</p>
                  <p className="mt-1 text-[11px] font-semibold text-[#78958b]">Give each player a custom starting pool.</p>
                </div>
                <button
                  type="button"
                  role="switch"
                  aria-checked={perPlayerHpEnabled}
                  aria-label="Per-Player HP"
                  onClick={() => {
                    const next = !perPlayerHpEnabled;
                    setPerPlayerHpEnabled(next);
                    if (next) {
                      const seeded = Object.fromEntries(
                        snapshot.members.map((member) => [
                          member.userId,
                          playerHpDraft[member.userId] || config.initialHp || 6000,
                        ]),
                      );
                      setPlayerHpDraft(seeded);
                      saveConfig({ playerHpOverrides: seeded });
                    } else {
                      setPlayerHpDraft({});
                      saveConfig({ playerHpOverrides: {} });
                    }
                  }}
                  disabled={busy}
                  className={`relative inline-flex h-6 w-11 shrink-0 items-center rounded-full border transition focus:outline-none focus:ring-2 focus:ring-[#77f0be]/60 disabled:cursor-not-allowed disabled:opacity-50 ${perPlayerHpEnabled ? "border-[#77f0be]/40 bg-[#2ad18f]" : "border-white/15 bg-white/[0.08]"}`}
                >
                  <span className={`h-4 w-4 rounded-full bg-white shadow-sm transition-transform ${perPlayerHpEnabled ? "translate-x-6" : "translate-x-1"}`} />
                </button>
              </div>
              
              {perPlayerHpEnabled && (
                <div className="mt-3 grid gap-2 sm:grid-cols-2">
                  {snapshot.members.map((member) => (
                    <div key={member.userId} className="flex items-center justify-between rounded-xl bg-white/[0.03] p-2 border border-white/5">
                      <span className="truncate text-[13px] font-bold text-white px-2">
                        {member.displayName || member.userId}
                      </span>
                      <input
                        type="number"
                        min="1"
                        value={playerHpDraft[member.userId] || config.initialHp || 6000}
                        onChange={(e) => {
                          const val = Math.max(1, parseInt(e.target.value) || 1);
                          setPlayerHpDraft((current) => ({ ...current, [member.userId]: val }));
                          saveConfig({
                            playerHpOverrides: {
                              ...playerHpDraft,
                              [member.userId]: val
                            }
                          });
                        }}
                        disabled={busy}
                        className="w-24 rounded-lg bg-black/40 border border-white/15 px-2 py-1 text-right font-mono text-[13px] font-extrabold text-[#77f0be] focus:border-[#77f0be] focus:outline-none"
                      />
                    </div>
                  ))}
                </div>
              )}
            </div>
          </>
          )}
        </div>
      </div>
    </LobbyInset>
  );
}

type PartyOption = { value: string; label: string; shortLabel?: string };

function PartyChoiceMenu({
  disabled,
  label,
  onChange,
  options,
  value,
}: {
  disabled: boolean;
  label: string;
  onChange: (value: string) => void;
  options: readonly PartyOption[];
  value: string;
}) {
  const [open, setOpen] = useState(false);
  const selected = options.find((option) => option.value === value) || options[0];

  return (
    <div className="relative">
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={label}
        disabled={disabled}
        onClick={() => setOpen((current) => !current)}
        className="inline-flex h-10 min-h-10 w-full items-center justify-between gap-2 rounded-xl border border-white/10 bg-white/[0.08] px-3 text-left text-sm font-bold text-white transition hover:bg-white/[0.13] disabled:cursor-not-allowed disabled:opacity-50"
      >
        <span className="truncate">{selected.label}</span>
        <ChevronDown size={15} className={`shrink-0 text-[#77f0be] transition-transform ${open ? "rotate-180" : ""}`} />
      </button>
      {open ? (
        <div role="menu" aria-label={`${label} options`} className="absolute inset-x-0 top-[calc(100%+0.35rem)] z-30 grid gap-1 rounded-xl border border-white/15 bg-black p-1.5 shadow-2xl">
          {options.map((option) => (
            <button
              key={option.value}
              type="button"
              role="menuitemradio"
              aria-checked={option.value === value}
              onClick={() => {
                onChange(option.value);
                setOpen(false);
              }}
              className={`min-h-9 rounded-lg px-3 text-left text-xs font-extrabold transition ${option.value === value ? "bg-[#2ad18f]/20 text-[#77f0be]" : "text-white hover:bg-white/10"}`}
            >
              {option.label}
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function PartyOptionSlider({
  disabled,
  onChange,
  options,
  value,
}: {
  disabled: boolean;
  onChange: (value: string) => void;
  options: readonly PartyOption[];
  value: string;
}) {
  const selectedIndex = Math.max(0, options.findIndex((option) => option.value === value));
  const selected = options[selectedIndex] || options[0];

  return (
    <div className="min-w-0 rounded-xl border border-white/10 bg-white/[0.08] px-3 py-2.5 transition hover:bg-white/[0.12]">
      <div className="flex min-w-0 items-center justify-between gap-2">
        <span className="truncate text-sm font-bold text-white">{selected?.label}</span>
      </div>
      <div className="relative mt-2 px-1">
        <input
          type="range"
          min={0}
          max={Math.max(0, options.length - 1)}
          step={1}
          value={selectedIndex}
          onChange={(event) => onChange(options[Number(event.target.value)]?.value || options[0].value)}
          disabled={disabled}
          aria-label={selected?.label}
          className="relative z-10 h-3 w-full cursor-pointer accent-[#2ad18f] disabled:cursor-not-allowed"
        />
        <div className="pointer-events-none absolute inset-x-1 top-[5px] flex justify-between">
          {options.map((option) => (
            <span key={option.value} className="h-2 w-0.5 rounded-full bg-white/35" />
          ))}
        </div>
      </div>
      <div className="mt-1 grid min-w-0" style={{ gridTemplateColumns: `repeat(${options.length}, minmax(0, 1fr))` }}>
        {options.map((option) => (
          <span key={option.value} className="min-w-0 truncate text-center text-[9px] font-bold text-[#78958b]">
            {option.shortLabel || option.label}
          </span>
        ))}
      </div>
    </div>
  );
}

function PartyMemberList({
  busy,
  isOwner,
  kickPartyMember,
  members,
  mode,
  snapshot,
  switchPartyTeam,
  transferPartyOwner,
  userId,
}: {
  busy: boolean;
  isOwner: boolean;
  kickPartyMember: (userId: string) => Promise<void>;
  members: PartySnapshot["members"];
  mode: PartyMode;
  snapshot: PartySnapshot;
  switchPartyTeam: (teamId: PartyTeamId) => Promise<void>;
  transferPartyOwner: (userId: string) => Promise<void>;
  userId: string;
}) {
  return (
    <div className="grid gap-3">
      {members.map((member) => {
        const isLeader = member.userId === snapshot.ownerUserId;
        const isSelf = member.userId === userId;
        const presenceStatus = member.presenceStatus || (member.connected ? "online" : "offline");
        const lobbyStatus =
          presenceStatus === "online" ? "Online" : presenceStatus === "away" ? "Reconnecting" : "Offline";
        return (
          <LobbyInset
            key={member.userId}
            className={`flex min-h-[72px] flex-col gap-3 sm:flex-row sm:items-center sm:justify-between ${
              presenceStatus === "offline" ? "opacity-50" : ""
            }`}
          >
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <PlayerProfileLink userId={member.userId} nickname={member.displayName} disabled={member.isGuest} className="truncate text-[16px] font-extrabold text-white hover:text-emerald-200">
                  {member.displayName || member.userId}
                </PlayerProfileLink>
                {isLeader ? (
                  <LobbyPill tone="success">
                    <Crown className="mr-1" size={12} />
                    Leader
                  </LobbyPill>
                ) : null}
              </div>
              <p className="mt-1 text-[12px] font-semibold text-[#a9bfd4]">
                {isSelf ? `You · ${lobbyStatus}` : lobbyStatus}
              </p>
              {mode === "team_duel" ? (
                <p className="mt-1 text-[12px] font-semibold uppercase tracking-[0.12em]">
                  <span className={lobbyTeamTextClass(member.teamId)}>{lobbyTeamLabel(member.teamId)}</span>
                </p>
              ) : null}
            </div>
            {mode === "team_duel" && isSelf && snapshot.state === "open" ? (
              <div className="flex gap-2">
                {(["a", "b"] as const).map((teamId) => (
                  <button
                    key={teamId}
                    type="button"
                    onClick={() => void switchPartyTeam(teamId)}
                    aria-pressed={(member.teamId || "a") === teamId}
                    disabled={busy}
                    style={{
                      backgroundColor:
                        (member.teamId || "a") === teamId
                          ? teamId === "a"
                            ? "rgba(220, 38, 38, 0.72)"
                            : "rgba(37, 99, 235, 0.72)"
                          : teamId === "a"
                            ? "rgba(220, 38, 38, 0.25)"
                            : "rgba(37, 99, 235, 0.25)",
                    }}
                    className={`inline-flex min-h-[38px] items-center rounded-[10px] px-3 text-[11px] font-extrabold uppercase tracking-[0.08em] transition disabled:opacity-100 ${lobbyTeamPillClass(
                      teamId,
                      (member.teamId || "a") === teamId,
                    )}`}
                  >
                    {lobbyTeamLabel(teamId)}
                  </button>
                ))}
              </div>
            ) : null}
            {isOwner && snapshot.state === "open" && !isSelf ? (
              <div className="flex flex-wrap gap-2">
                <LobbyActionButton
                  type="button"
                  variant="secondary"
                  size="sm"
                  onClick={() => void transferPartyOwner(member.userId)}
                  disabled={busy}
                >
                  <Crown size={14} />
                  Make Leader
                </LobbyActionButton>
                <LobbyActionButton
                  type="button"
                  variant="danger"
                  size="sm"
                  onClick={() => void kickPartyMember(member.userId)}
                  disabled={busy}
                >
                  <UserMinus size={14} />
                  Kick
                </LobbyActionButton>
              </div>
            ) : null}
          </LobbyInset>
        );
      })}
    </div>
  );
}
