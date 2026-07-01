import AppModalShell from "../ui/AppModalShell";

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
      zIndexClassName="z-[2100]"
      maxWidthClassName="max-w-md"
    >
      <div className="space-y-5">
        <p className="-mt-4 text-[15px] leading-relaxed text-[#a9bfd4]">
          Nicknames use 2–14 letters, numbers, dots, or underscores.
        </p>
        <input
          value={nicknameInput}
          onChange={(e) => onChangeNickname(e.target.value)}
          className="w-full rounded-xl border border-white/10 bg-white/5 px-4 py-3 text-[15px] text-white outline-none transition-colors placeholder:text-white/30 focus:border-[#2ad18f]/50 focus:bg-white/10"
          maxLength={14}
          minLength={2}
          pattern="(?!.*\.\.)(?!.*__)[A-Za-z0-9._]{2,14}"
          autoComplete="nickname"
          autoFocus
          placeholder="Suggested nickname"
        />
        {nicknameError ? (
          <p className="text-xs font-semibold text-red-400">{nicknameError}</p>
        ) : null}
        <button
          type="button"
          onClick={onSubmit}
          disabled={nicknameSaving}
          className="flex w-full items-center justify-center rounded-[16px] bg-accentPrimary py-[14px] text-[16px] font-extrabold uppercase tracking-[0.08em] text-white shadow-[0_4px_16px_rgba(42,209,143,0.3)] transition-all duration-200 hover:scale-[1.01] hover:bg-accentPrimaryDeep hover:shadow-[0_6px_24px_rgba(42,209,143,0.4)] active:scale-[0.98] disabled:opacity-50 disabled:hover:scale-100 disabled:hover:shadow-[0_4px_16px_rgba(42,209,143,0.3)]"
        >
          {nicknameSaving ? "Saving..." : "Continue"}
        </button>
      </div>
    </AppModalShell>
  );
}
