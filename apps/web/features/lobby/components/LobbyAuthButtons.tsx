import type React from "react";
import { LobbyActionButton } from "./lobby-primitives";

export function DiscordProviderButton({
  authLoading,
  onClick,
}: {
  authLoading: boolean;
  onClick: () => void;
}) {
  return (
    <LobbyActionButton
      type="button"
      onClick={onClick}
      disabled={authLoading}
      variant="secondary"
      className="rounded-2xl px-3 py-2.5 text-xs sm:px-4"
    >
      <span className="flex h-6 w-6 items-center justify-center rounded-full bg-[#5865f2] text-white shadow-sm">
        <DiscordIcon />
      </span>
      {authLoading ? "Signing In..." : "Continue With Discord"}
    </LobbyActionButton>
  );
}

export function GoogleProviderButton({
  authLoading,
  label = "Continue With Google",
  onClick,
}: {
  authLoading: boolean;
  label?: string;
  onClick: () => void;
}) {
  return (
    <LobbyActionButton
      type="button"
      onClick={onClick}
      disabled={authLoading}
      variant="secondary"
      className="rounded-2xl px-3 py-2.5 text-xs sm:px-4"
    >
      <span className="flex h-6 w-6 items-center justify-center rounded-full bg-white text-[#111827] shadow-sm">
        <GoogleIcon />
      </span>
      {authLoading ? "Opening Google..." : label}
    </LobbyActionButton>
  );
}

export function SignInButton({
  authLoading,
  children,
  onClick,
  rounded = "2xl",
}: {
  authLoading: boolean;
  children: React.ReactNode;
  onClick: () => void;
  rounded?: "full" | "2xl";
}) {
  return (
    <LobbyActionButton
      type="button"
      onClick={onClick}
      variant="secondary"
      className={rounded === "full" ? "rounded-full px-4 py-2 text-xs" : "rounded-2xl px-3 py-2.5 text-xs sm:px-4"}
    >
      {authLoading ? "Signing In..." : children}
    </LobbyActionButton>
  );
}

function DiscordIcon() {
  return (
    <svg viewBox="0 0 127.14 96.36" className="h-3.5 w-4" aria-hidden="true">
      <path
        fill="currentColor"
        d="M107.7 8.07A105.15 105.15 0 0 0 81.47 0a72.06 72.06 0 0 0-3.36 6.83 97.68 97.68 0 0 0-29.11 0A72.37 72.37 0 0 0 45.64 0 105.89 105.89 0 0 0 19.39 8.09C2.79 32.65-1.71 56.6.54 80.21a105.73 105.73 0 0 0 32.17 16.15 77.7 77.7 0 0 0 6.89-11.11 68.42 68.42 0 0 1-10.85-5.18c.91-.66 1.8-1.34 2.66-2.04a75.57 75.57 0 0 0 64.32 0c.87.71 1.76 1.39 2.66 2.04a68.68 68.68 0 0 1-10.87 5.19 77 77 0 0 0 6.89 11.1 105.25 105.25 0 0 0 32.19-16.14c2.64-27.38-4.52-51.11-18.9-72.15ZM42.45 65.69c-6.27 0-11.43-5.73-11.43-12.78s5.05-12.79 11.43-12.79 11.54 5.78 11.43 12.79-5.06 12.78-11.43 12.78Zm42.24 0c-6.27 0-11.43-5.73-11.43-12.78s5.05-12.79 11.43-12.79 11.54 5.78 11.43 12.79-5.05 12.78-11.43 12.78Z"
      />
    </svg>
  );
}

function GoogleIcon() {
  return (
    <svg viewBox="0 0 24 24" className="h-3.5 w-3.5" aria-hidden="true">
      <path
        fill="#4285F4"
        d="M21.805 10.023h-9.81v3.955h5.627c-.242 1.272-.967 2.35-2.06 3.073v2.55h3.332c1.95-1.796 3.073-4.44 3.073-7.578 0-.662-.06-1.298-.162-1.999Z"
      />
      <path
        fill="#34A853"
        d="M11.995 22c2.79 0 5.132-.924 6.842-2.5l-3.332-2.55c-.924.62-2.102.987-3.51.987-2.699 0-4.985-1.822-5.805-4.272H2.758v2.63A10.329 10.329 0 0 0 11.995 22Z"
      />
      <path
        fill="#FBBC05"
        d="M6.19 13.665a6.214 6.214 0 0 1-.324-1.967c0-.684.118-1.347.324-1.967v-2.63H2.758A10.329 10.329 0 0 0 1.663 11.7c0 1.66.398 3.232 1.095 4.598l3.432-2.633Z"
      />
      <path
        fill="#EA4335"
        d="M11.995 5.463c1.518 0 2.88.523 3.95 1.55l2.962-2.962C17.122 2.397 14.782 1.4 11.995 1.4 7.958 1.4 4.47 3.707 2.758 7.101l3.432 2.63c.82-2.45 3.106-4.268 5.805-4.268Z"
      />
    </svg>
  );
}
