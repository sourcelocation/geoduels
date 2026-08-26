import AppModalShell from "../ui/AppModalShell";
import { Button } from "../ui/button";
import { Input } from "../ui/input";

type RequiredNicknameModalProps = {
  open: boolean;
  nicknameInput: string;
  nicknameError: string;
  nicknameSaving: boolean;
  onChangeNickname: (value: string) => void;
  onSubmit: () => void;
};

export default function RequiredNicknameModal({
  open,
  nicknameInput,
  nicknameError,
  nicknameSaving,
  onChangeNickname,
  onSubmit
}: RequiredNicknameModalProps) {
  if (!open) return null;
  return (
    <AppModalShell
      title="Choose Your Nickname"
      placement="center"
      zIndexClassName="z-modal-critical"
      maxWidthClassName="max-w-md"
    >
      <div className="space-y-5">
        <p className="-mt-4 text-body-sm leading-prose text-content-secondary">
          Nicknames use 2–14 letters, numbers, dots, or underscores.
        </p>
        <Input
          variant="game"
          value={nicknameInput}
          onChange={(e) => onChangeNickname(e.target.value)}
          className="w-full"
          maxLength={14}
          minLength={2}
          pattern="(?!.*\.\.)(?!.*__)[A-Za-z0-9._]{2,14}"
          autoComplete="nickname"
          autoFocus
          placeholder="Suggested nickname"
        />
        {nicknameError ? (
          <p className="text-body-sm font-semibold text-status-danger">{nicknameError}</p>
        ) : null}
        <Button
          type="button"
          variant="primary"
          size="lg"
          onClick={onSubmit}
          disabled={nicknameSaving}
          className="w-full"
        >
          {nicknameSaving ? "Saving..." : "Continue"}
        </Button>
      </div>
    </AppModalShell>
  );
}
