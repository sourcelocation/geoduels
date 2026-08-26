import { forwardRef } from "react";
import { ArrowLeftRight, Copy, Crown, LogOut, Map as MapIcon, Play, UserMinus } from "lucide-react";
import { Spinner } from "../../../components/ui/Spinner";
import { motion } from "framer-motion";
import { toPublicEntityId } from "../../../lib/entity-id";
import PlayerProfileLink from "../../players/components/PlayerProfileLink";
import { Button, ButtonLink } from "../../../components/ui/button";
import { Badge } from "../../../components/ui/Badge";
import { AppPanel } from "../../../components/ui/compositions";
import { DiscreteSlider } from "../../../components/ui/DiscreteSlider";
import type { MatchConfig, GameRuleset } from "../../matchmaking/lib/queue-client";
import type { PartyRuntimeStatus } from "../controllers/party-controller";
import type { PartySnapshot, PartyTeamId, PartyMode } from "../lib/party-client";
import {
  CLOCK_OPTIONS,
  PRESSURE_OPTIONS,
  lobbyTeamLabel,
} from "../lib/lobby-ui";
import type { PartyPanelState } from "../hooks/usePartyPanelState";
import {
  LobbyFieldLabel,
  LobbySection,
  LobbySelect,
} from "./lobby-primitives";

type PartyView = {
  status: PartyRuntimeStatus;
  snapshot: PartySnapshot | null;
  inviteCode: string;
  isOwner: boolean;
  busy: boolean;
  error: string;
};

type PartyPanelProps = {
  inviteCopied: boolean;
  party: PartyView;
  setMapPickerOpen: (open: boolean) => void;
  state: PartyPanelState;
  userId: string;
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
  inviteCopied,
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
      className="pointer-events-auto relative flex h-auto w-full max-w-[1180px] flex-col gap-4 md:h-full md:min-h-0"
    >
      <PartyActions
        inviteCopied={inviteCopied}
        matchInProgress={matchInProgress}
        onCopyInvite={copyInvite}
        onLeave={() => void leaveParty()}
        party={party}
      />

      <div className="flex flex-none items-stretch justify-center pt-12 md:min-h-0 md:flex-1">
        <div className="flex w-full max-w-[900px] flex-col md:min-h-0">
            {matchInProgress ? (
              <LobbySection className="mb-4">
                <p className="text-label font-strong text-status-success">
                  Game In Progress
                </p>
                <p className="mt-1 text-body-sm font-semibold text-content-primary">
                  {currentMember?.inActiveMatch
                      ? "You are part of this game and can reconnect whenever you are ready."
                      : "You joined after this game started and will be able to play in the next one."}
                </p>
                {currentMember?.inActiveMatch && activeMatchId ? (
                  <ButtonLink
                    variant="primary"
                    href={`/match/${encodeURIComponent(toPublicEntityId(activeMatchId))}`}
                    size="md"
                    className="mt-3"
                  >
                    <Play size={16} fill="currentColor" />
                    Reconnect to Game
                  </ButtonLink>
                ) : null}
              </LobbySection>
            ) : null}

            {party.snapshot ? (
              <PartyMemberList
                busy={party.busy}
                isOwner={party.isOwner}
                members={members}
                mode={mode}
                snapshot={party.snapshot}
                fadeAtBottom={party.isOwner && party.snapshot.state === "open"}
                switchPartyTeam={switchPartyTeam}
                transferPartyOwner={transferPartyOwner}
                kickPartyMember={kickPartyMember}
                userId={userId}
              />
            ) : null}
        </div>
      </div>

      <div className="mx-auto grid w-full max-w-[1040px] shrink-0 gap-3 pt-3">
            {party.snapshot ? (
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

            {party.isOwner && party.snapshot?.state === "open" ? (
              <Button
                variant="primary"
                type="button"
                onClick={() => void startParty()}
                disabled={!canStart || party.busy}
                size="lg"
                className="min-h-14 w-full text-body"
              >
                {party.busy ? <Spinner size="sm" label="Starting match" color="current" /> : <Play size={18} fill="currentColor" />}
                {mode === "team_duel" ? "Start Team Duel" : mode === "free_for_all" ? "Start FFA" : "Start Duel"}
              </Button>
            ) : party.snapshot?.state === "open" ? (
              <LobbySection className="text-center text-body-sm font-semibold text-content-secondary">
                Waiting for the leader to start.
              </LobbySection>
            ) : null}

      </div>
    </motion.div>
  );
});

function PartyActions({
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
    <div className="absolute left-1/2 top-0 z-content flex -translate-x-1/2 gap-2">
        {party.snapshot && !(party.isOwner && matchInProgress) ? (
          <Button
            type="button"
            variant="secondary"
            onClick={onLeave}
            disabled={party.busy}
          >
            <LogOut size={16} />
            Leave
          </Button>
        ) : null}
        {party.inviteCode ? (
          <Button type="button" variant="secondary" onClick={onCopyInvite}>
            <Copy className="text-status-success" size={16} />
            {inviteCopied ? "Copied" : `Invite ${party.inviteCode}`}
          </Button>
        ) : null}
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
  if (!(isOwner && snapshot.state === "open")) {
    return (
      <LobbySection>
        <p className="text-label font-strong text-content-secondary">
          Game Settings
        </p>
        <p className="mt-1 text-body-sm font-semibold text-content-primary">
          {config.mapName} · {config.ruleset === "nmpz" ? "NMPZ" : "Moving"} ·{" "}
          {clockOn ? `${roundSeconds}s clock` : "Infinite clock"} ·{" "}
          {pressureOn ? `${pressureSeconds}s pressure` : "No pressure"} ·{" "}
          {config.multiplierMode === "individual" ? "Individual multipliers" : "Shared multiplier"}
        </p>
      </LobbySection>
    );
  }

  return (
    <LobbySection>
      <div className="grid w-full gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <label className="grid gap-1.5">
          <LobbyFieldLabel>Multipliers</LobbyFieldLabel>
          <LobbySelect
            value={config.multiplierMode || "shared"}
            onChange={(event) =>
              saveConfig({ multiplierMode: event.target.value as "shared" | "individual" })
            }
            disabled={busy}
          >
            <option value="shared">Shared</option>
            <option value="individual">Individual</option>
          </LobbySelect>
        </label>
        <label className="grid gap-1.5">
          <LobbyFieldLabel>Mode</LobbyFieldLabel>
          <LobbySelect
            value={mode}
            onChange={(event) => saveMode(event.target.value as PartyMode)}
            disabled={busy}
          >
            <option value="duel">Duel</option>
            <option value="team_duel">Team Duel</option>
            <option value="free_for_all">Free For All</option>
          </LobbySelect>
        </label>
        <label className="grid gap-1.5">
          <LobbyFieldLabel>Map</LobbyFieldLabel>
          <Button
            type="button"
            variant="secondary"
            size="md"
            onClick={() => setMapPickerOpen(true)}
            disabled={busy}
            className="min-w-0 justify-center normal-case tracking-body"
          >
            <MapIcon className="shrink-0 text-status-success" size={14} />
            <span className="truncate">{config.mapName}</span>
          </Button>
        </label>
        <label className="grid gap-1.5">
          <LobbyFieldLabel>Rules</LobbyFieldLabel>
          <LobbySelect
            value={config.ruleset || "moving"}
            onChange={(event) => saveConfig({ ruleset: event.target.value as GameRuleset })}
            disabled={busy}
          >
            <option value="moving">Moving</option>
            <option value="nmpz">NMPZ</option>
          </LobbySelect>
        </label>
        <div className="grid gap-2 sm:col-span-2">
          <LobbyFieldLabel>Clock</LobbyFieldLabel>
          <p className="text-heading-sm font-strong text-content-primary">{clockOn ? `${roundSeconds}s` : "Infinite"}</p>
          <DiscreteSlider
            aria-label="Round clock"
            value={clockOn ? String(roundSeconds) : "infinite"}
            options={CLOCK_OPTIONS}
            onValueChange={(value) => {
              saveConfig(
                value === "infinite"
                  ? { roundTimerMode: "none", roundTimeLimitMs: undefined }
                  : { roundTimerMode: "fixed", roundTimeLimitMs: Number(value) * 1000 },
              );
            }}
            disabled={busy}
          />
        </div>
        <div className="grid gap-2 sm:col-span-2">
          <LobbyFieldLabel>Pressure</LobbyFieldLabel>
          <p className="text-heading-sm font-strong text-content-primary">{pressureOn ? `${pressureSeconds}s` : "None"}</p>
          <DiscreteSlider
            aria-label="Guess pressure"
            value={pressureOn ? String(pressureSeconds) : "none"}
            options={PRESSURE_OPTIONS}
            onValueChange={(value) => {
              saveConfig({ pressureTimeLimitMs: value === "none" ? undefined : Number(value) * 1000 });
            }}
            disabled={busy}
          />
        </div>
      </div>
    </LobbySection>
  );
}

function PartyMemberList({
  busy,
  fadeAtBottom,
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
  fadeAtBottom: boolean;
  isOwner: boolean;
  kickPartyMember: (userId: string) => Promise<void>;
  members: PartySnapshot["members"];
  mode: PartyMode;
  snapshot: PartySnapshot;
  switchPartyTeam: (teamId: PartyTeamId) => Promise<void>;
  transferPartyOwner: (userId: string) => Promise<void>;
  userId: string;
}) {
  const renderMember = (member: PartySnapshot["members"][number]) => {
    const isLeader = member.userId === snapshot.ownerUserId;
    const isSelf = member.userId === userId;
    const presenceStatus = member.presenceStatus || (member.connected ? "online" : "offline");
    const lobbyStatus =
      presenceStatus === "online" ? "Online" : presenceStatus === "away" ? "Reconnecting" : "Offline";

    return (
      <LobbySection
        key={member.userId}
        className={`flex min-h-[76px] items-center justify-between gap-3 ${presenceStatus === "offline" ? "opacity-50" : ""}`}
      >
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <PlayerProfileLink userId={member.userId} nickname={member.displayName} disabled={member.isGuest} className="truncate text-body font-strong text-content-primary hover:text-status-success">
              {member.displayName || member.userId}
            </PlayerProfileLink>
            {isLeader ? (
              <Badge tone="success"><Crown size={12} />Leader</Badge>
            ) : null}
          </div>
          <p className="mt-1 text-body-sm font-semibold text-content-secondary">{isSelf ? `You · ${lobbyStatus}` : lobbyStatus}</p>
        </div>
        {isOwner && snapshot.state === "open" && !isSelf ? (
          <div className="flex shrink-0 gap-2">
            <Button type="button" variant="secondary" size="sm" onClick={() => void transferPartyOwner(member.userId)} disabled={busy} aria-label={`Make ${member.displayName || member.userId} leader`}>
              <Crown size={14} />
              <span className="hidden sm:inline">Leader</span>
            </Button>
            <Button type="button" variant="danger" size="sm" onClick={() => void kickPartyMember(member.userId)} disabled={busy} aria-label={`Kick ${member.displayName || member.userId}`}>
              <UserMinus size={14} />
            </Button>
          </div>
        ) : null}
      </LobbySection>
    );
  };

  if (mode === "team_duel") {
    const blueMembers = members.filter((member) => member.teamId === "b");
    const redMembers = members.filter((member) => (member.teamId || "a") === "a");
    const selfTeam = (members.find((member) => member.userId === userId)?.teamId || "a") as PartyTeamId;
    const targetTeam: PartyTeamId = selfTeam === "a" ? "b" : "a";

    const teamColumn = (teamId: PartyTeamId, teamMembers: typeof members) => (
      <AppPanel className={teamId === "b" ? "flex flex-col overflow-hidden border-status-info/35 bg-status-info/10 p-4 md:min-h-0 md:flex-1" : "flex flex-col overflow-hidden border-status-danger/35 bg-status-danger/10 p-4 md:min-h-0 md:flex-1"}>
        <div className="mb-3 flex items-center justify-between">
          <div>
            <p className={teamId === "b" ? "text-label font-strong text-status-info" : "text-label font-strong text-status-danger"}>{lobbyTeamLabel(teamId)}</p>
            <p className="mt-1 text-body-sm text-content-secondary">{teamMembers.length} {teamMembers.length === 1 ? "player" : "players"}</p>
          </div>
        </div>
        <div className={`grid content-start gap-2 overflow-visible md:min-h-0 md:flex-1 md:overflow-y-auto ${fadeAtBottom ? "party-player-list-fade pb-8" : ""}`}>
          {teamMembers.length ? teamMembers.map(renderMember) : (
            <div className="rounded-lg border border-dashed border-border-default p-6 text-center text-body-sm font-semibold text-content-secondary">No players yet</div>
          )}
        </div>
      </AppPanel>
    );

    return (
      <div className="party-team-grid grid items-stretch gap-3 md:min-h-0 md:flex-1">
        {teamColumn("b", blueMembers)}
        <Button
          type="button"
          variant="secondary"
          disabled={busy || snapshot.state !== "open"}
          onClick={() => void switchPartyTeam(targetTeam)}
          className="mx-auto whitespace-nowrap"
        >
          <ArrowLeftRight size={16} />
          Switch teams
        </Button>
        {teamColumn("a", redMembers)}
      </div>
    );
  }

  return (
    <div className="mx-auto flex w-full max-w-[600px] flex-none flex-col gap-3 md:min-h-0 md:flex-1">
      <div className="mb-1 text-center">
        <p className="text-label font-strong text-status-success">Players</p>
        <p className="mt-1 text-body-sm text-content-secondary">{members.length} in party</p>
      </div>
      <div className={`grid content-start gap-3 overflow-visible md:min-h-0 md:flex-1 md:overflow-y-auto ${fadeAtBottom ? "party-player-list-fade pb-8" : ""}`}>
        {members.map(renderMember)}
      </div>
    </div>
  );
}
