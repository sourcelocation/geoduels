import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AccountSettingsModal } from "./AccountSettingsModal";

const mocks = vi.hoisted(() => ({
  requestSession: vi.fn(),
  requestMe: vi.fn(),
  requestUnlinkAuthProvider: vi.fn(),
  requestDeleteAccount: vi.fn(),
  requestLogout: vi.fn(),
  requestGoogleStart: vi.fn(),
  requestDiscordStart: vi.fn(),
  routerPush: vi.fn(),
  authState: {
    status: "registered",
    session: { accessToken: "access-token", linkedProviders: ["google", "discord"] },
    accessToken: "access-token",
    userId: "player-1",
    isGuest: false,
    isRegistered: true,
    isAdmin: false,
    isModerator: false,
    displayName: "Atlas",
    avatarUrl: "",
    email: "atlas@example.com",
    canPlayUnranked: true,
    canPlayRanked: true,
    canUseSocial: true,
    canManageMaps: true,
  },
}));

vi.mock("../../auth/lib/auth-client", () => mocks);
vi.mock("../../auth/components/AuthProvider", () => ({
  useAuthState: () => mocks.authState,
}));
vi.mock("next/router", () => ({
  useRouter: () => ({ push: mocks.routerPush }),
}));
vi.mock("../../../lib/runtime-config", () => ({
  getRuntimeConfig: () => ({
    googleClientId: "google-client",
    discordClientId: "discord-client",
  }),
}));

function renderModal() {
  mocks.requestSession.mockResolvedValue({
    accessToken: "access-token",
    linkedProviders: ["google", "discord"],
    user: { id: "player-1", email: "atlas@example.com" },
  });
  mocks.requestMe.mockResolvedValue(
    new Response(
      JSON.stringify({
        email: "atlas@example.com",
        isGuest: false,
      }),
      { status: 200 },
    ),
  );
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  render(
    <QueryClientProvider client={client}>
      <AccountSettingsModal
        onClose={vi.fn()}
        profilePath="/players/player-1"
      />
    </QueryClientProvider>,
  );
}

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("AccountSettingsModal", () => {
  it("loads confidential account details only inside the modal", async () => {
    renderModal();

    expect(
      screen.getByRole("dialog", { name: "Account settings" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("atlas@example.com")).toBeInTheDocument();
    expect(mocks.requestSession).not.toHaveBeenCalled();
    expect(mocks.requestMe).toHaveBeenCalledWith(
      expect.any(Object),
      "access-token",
    );
  });

  it("supports provider unlinking", async () => {
    mocks.requestUnlinkAuthProvider.mockResolvedValue({});
    renderModal();
    await screen.findByText("atlas@example.com");

    fireEvent.click(screen.getAllByRole("button", { name: "Unlink" })[0]);
    await waitFor(() =>
      expect(mocks.requestUnlinkAuthProvider).toHaveBeenCalledWith(
        expect.any(Object),
        "access-token",
        "google",
      ),
    );
  });

  it("supports sign out", async () => {
    mocks.requestLogout.mockResolvedValue(undefined);
    renderModal();
    await screen.findByText("atlas@example.com");
    fireEvent.click(screen.getByRole("button", { name: "Sign out" }));
    await waitFor(() => expect(mocks.requestLogout).toHaveBeenCalled());
    expect(mocks.routerPush).toHaveBeenCalledWith("/");
  });

  it("requires confirmation before deleting the account", async () => {
    mocks.requestDeleteAccount.mockResolvedValue(undefined);
    renderModal();
    await screen.findByText("atlas@example.com");
    const disclosure = screen.getByRole("button", { name: /Delete account Permanently remove/ });
    expect(disclosure).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByLabelText("Confirmation")).not.toBeInTheDocument();
    fireEvent.click(disclosure);
    expect(disclosure).toHaveAttribute("aria-expanded", "true");
    expect(
      screen.getByRole("button", { name: "Delete account" }),
    ).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Confirmation"), {
      target: { value: "DELETE" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Delete account" }));
    await waitFor(() =>
      expect(mocks.requestDeleteAccount).toHaveBeenCalledWith(
        expect.any(Object),
        "access-token",
      ),
    );
  });
});
