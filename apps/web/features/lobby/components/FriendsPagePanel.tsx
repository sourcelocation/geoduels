import React, { useCallback, useState } from "react";
import { ArrowRight, Compass, Shield, Sliders, Users, UserPlus, Zap, Key } from "lucide-react";
import { LobbyActionButton, LobbyPanel } from "./lobby-primitives";
import type { PartyMode } from "../lib/party-client";
import type { MatchConfig } from "../../matchmaking/lib/queue-client";

type Props = {
  disabled: boolean;
  partyBusy: boolean;
  authError: string;
  createParty: (mode?: PartyMode, config?: MatchConfig) => Promise<boolean>;
  joinParty: (inviteCode?: string) => Promise<boolean>;
};

const partyFeatures = [
  {
    icon: <Sliders size={20} className="text-emerald-200" />,
    title: "Custom HP & Multipliers",
    desc: "Set HP from 1 HP sudden death up to 50k HP, plus custom multiplier round acceleration.",
  },
  {
    icon: <Zap size={20} className="text-emerald-200" />,
    title: "No Move Mode",
    desc: "Full support for Moving, No Move, or NMPZ rulesets in any private party game.",
  },
  {
    icon: <Compass size={20} className="text-emerald-200" />,
    title: "Custom Maps",
    desc: "Play official maps or select any community-created map from the map library.",
  },
  {
    icon: <Shield size={20} className="text-emerald-200" />,
    title: "2v2 Teams & FFA",
    desc: "Battle in 1v1 duels, split into Red vs Blue teams, or play in up to 8-player FFA.",
  },
] as const;

export function FriendsPagePanel({
  disabled,
  partyBusy,
  authError,
  createParty,
  joinParty,
}: Props) {
  const [inviteCodeInput, setInviteCodeInput] = useState("");
  const [errorText, setErrorText] = useState("");

  const handleCreate = useCallback(async () => {
    setErrorText("");
    const ok = await createParty("duel");
    if (!ok) {
      setErrorText("Failed to create party. Please try again.");
    }
  }, [createParty]);

  const handleJoin = useCallback(async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const code = inviteCodeInput.trim().toUpperCase();
    if (!code) {
      setErrorText("Please enter a valid party code.");
      return;
    }
    setErrorText("");
    const ok = await joinParty(code);
    if (!ok) {
      setErrorText("Could not find or join party with that code.");
    }
  }, [inviteCodeInput, joinParty]);

  return (
    <div className="flex w-full max-w-[1040px] flex-col gap-6 pointer-events-auto">
      <h1 className="sr-only">Play GeoDuels with Friends</h1>
      {/* Main Party Actions: Create & Join */}
      <div className="grid w-full gap-5 md:grid-cols-2">
        {/* Create Party Card */}
        <LobbyPanel radius="2xl" density="none" className="flex flex-col justify-between p-6">
          <div>
            <div className="flex items-center justify-between">
              <span className="text-[11px] font-extrabold uppercase tracking-[0.16em] text-emerald-200">
                CREATE MATCH
              </span>
              <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-emerald-400/15 text-emerald-200">
                <UserPlus size={18} />
              </div>
            </div>
            <h2 className="mt-2 text-[22px] font-extrabold tracking-tight text-white">
              Create a Party
            </h2>
            <p className="mt-1 text-[13px] font-medium leading-relaxed text-slate-300">
              Create your party first, then choose the match format and rules in the host menu.
            </p>
          </div>

          <div className="mt-6 pt-4 border-t border-white/10">
            <LobbyActionButton
              type="button"
              onClick={() => void handleCreate()}
              disabled={disabled || partyBusy}
              size="lg"
              className="w-full"
            >
              Create Party &amp; Invite
              <ArrowRight size={16} />
            </LobbyActionButton>
          </div>
        </LobbyPanel>

        {/* Join Party Card */}
        <LobbyPanel radius="2xl" density="none" className="flex flex-col justify-between p-6">
          <div>
            <div className="flex items-center justify-between">
              <span className="text-[11px] font-extrabold uppercase tracking-[0.16em] text-emerald-200">
                JOIN MATCH
              </span>
              <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-white/10 text-white">
                <Users size={18} />
              </div>
            </div>
            <h2 className="mt-2 text-[22px] font-extrabold tracking-tight text-white">
              Join with Code
            </h2>
            <p className="mt-1 text-[13px] font-medium leading-relaxed text-slate-300">
              Enter the 6-character party invite code shared by your friend.
            </p>

            <form onSubmit={(e) => void handleJoin(e)} className="mt-5 flex flex-col gap-3">
              <div className="relative">
                <div className="absolute inset-y-0 left-0 flex items-center pl-4 pointer-events-none text-emerald-200/60">
                  <Key size={16} />
                </div>
                <input
                  type="text"
                  value={inviteCodeInput}
                  onChange={(e) => setInviteCodeInput(e.target.value.toUpperCase())}
                  placeholder="ENTER CODE"
                  maxLength={10}
                  className="w-full rounded-xl border border-white/15 bg-black/40 pl-11 pr-4 py-3 font-mono text-[16px] font-extrabold tracking-widest text-white placeholder-emerald-200/40 transition focus:border-emerald-200 focus:outline-none focus:ring-1 focus:ring-emerald-200 shadow-inner"
                />
              </div>
              <LobbyActionButton
                type="submit"
                variant="secondary"
                disabled={disabled || partyBusy || !inviteCodeInput.trim()}
                size="lg"
                className="w-full"
              >
                Join Party
              </LobbyActionButton>
            </form>
          </div>

          {(errorText || authError) && (
            <p className="mt-3 text-center text-xs font-semibold text-red-300">
              {errorText || authError}
            </p>
          )}
        </LobbyPanel>
      </div>

      {/* Feature Showcase Grid */}
      <div className="grid w-full gap-4 sm:grid-cols-4">
        {partyFeatures.map((feature) => (
          <div
            key={feature.title}
            className="transition-transform duration-150 hover:-translate-y-0.5"
          >
            <LobbyPanel radius="xl" density="sm" className="p-4 h-full border border-white/[0.05] hover:border-emerald-200/20 hover:bg-white/[0.04] transition-all">
              <div className="mb-3 flex h-11 w-11 items-center justify-center rounded-xl bg-emerald-400/10 shadow-sm">
                {feature.icon}
              </div>
              <h3 className="text-[14px] font-extrabold text-white">{feature.title}</h3>
              <p className="mt-1.5 text-[12px] font-medium leading-relaxed text-slate-400">
                {feature.desc}
              </p>
            </LobbyPanel>
          </div>
        ))}
      </div>
    </div>
  );
}
