import { Loader2, UserPlus } from "lucide-react";
import AppModalShell from "../../../../components/ui/AppModalShell";
import {
  LobbyActionButton,
  LobbyFieldLabel,
  LobbyInput,
  LobbyInset,
} from "../lobby-primitives";

type InviteModalProps = {
  inviteCodeInput: string;
  setInviteCodeInput: (value: string) => void;
  busy: boolean;
  authLoading: boolean;
  maintenanceIsActive: boolean;
  playPaused: boolean;
  authError: string;
  createParty: () => Promise<boolean>;
  joinParty: (inviteCode?: string) => Promise<boolean>;
  onClose: () => void;
};

export function InviteModal({
  inviteCodeInput,
  setInviteCodeInput,
  busy,
  authLoading,
  maintenanceIsActive,
  playPaused,
  authError,
  createParty,
  joinParty,
  onClose,
}: InviteModalProps) {
  const normalizedInviteCode = inviteCodeInput.trim().toUpperCase();
  const inviteActionsDisabled = busy || authLoading || maintenanceIsActive;

  const join = async () => {
    if (!normalizedInviteCode) return;
    if (await joinParty(normalizedInviteCode)) onClose();
  };

  return (
    <AppModalShell title="Private Party" onClose={onClose}>
      <div className="space-y-4">
        <LobbyActionButton
          type="button"
          onClick={() => {
            void (async () => {
              if (await createParty()) onClose();
            })();
          }}
          disabled={inviteActionsDisabled || playPaused}
          className="min-h-12 w-full rounded-xl"
        >
          {busy ? <Loader2 className="mr-2 animate-spin" size={18} /> : <UserPlus className="mr-2" size={18} />}
          Create Party
        </LobbyActionButton>
        {authError ? <p className="text-center text-xs font-semibold text-red-300">{authError}</p> : null}

        <LobbyInset className="rounded-2xl">
          <LobbyFieldLabel htmlFor="invite-code-input" className="mb-2 block">
            Join With Code
          </LobbyFieldLabel>
          <div className="flex flex-col gap-2 sm:flex-row">
            <LobbyInput
              id="invite-code-input"
              value={inviteCodeInput}
              onChange={(event) => setInviteCodeInput(event.target.value.toUpperCase())}
              onKeyDown={(event) => {
                if (event.key === "Enter") void join();
              }}
              disabled={inviteActionsDisabled}
              className="min-h-[46px] min-w-0 flex-1 rounded-xl font-mono text-[15px] font-black uppercase tracking-[0.16em]"
              placeholder="CODE"
              maxLength={16}
              autoComplete="off"
            />
            <LobbyActionButton
              type="button"
              variant="secondary"
              onClick={() => void join()}
              disabled={inviteActionsDisabled || !normalizedInviteCode}
              className="min-h-[46px] rounded-xl px-5 text-xs"
            >
              Join
            </LobbyActionButton>
          </div>
        </LobbyInset>
      </div>
    </AppModalShell>
  );
}
