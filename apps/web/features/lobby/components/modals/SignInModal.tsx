import type React from "react";
import AppModalShell from "../../../../components/ui/AppModalShell";

type SignInModalProps = {
  googleProviderButton: React.ReactNode;
  discordProviderButton: React.ReactNode;
  fallbackButton: React.ReactNode;
  authError: string;
  onClose: () => void;
};

export function SignInModal({
  googleProviderButton,
  discordProviderButton,
  fallbackButton,
  authError,
  onClose,
}: SignInModalProps) {
  return (
    <AppModalShell title="Sign In" onClose={onClose} placement="center">
      <div className="space-y-3">
        {googleProviderButton ? (
          <div onClick={onClose} className="flex justify-center">
            {googleProviderButton}
          </div>
        ) : null}
        {discordProviderButton ? (
          <div onClick={onClose} className="flex justify-center">
            {discordProviderButton}
          </div>
        ) : null}
        {!googleProviderButton && !discordProviderButton ? (
          <div className="flex justify-center">{fallbackButton}</div>
        ) : null}
        {authError ? <p className="text-center text-xs font-semibold text-red-300">{authError}</p> : null}
      </div>
    </AppModalShell>
  );
}
