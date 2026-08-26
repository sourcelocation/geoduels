import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useState } from "react";
import {
  AuthActionsProvider,
  AuthProvider,
  AuthSignInModal,
  AuthStateProvider,
  authIntentForState,
  deriveAuthState,
  useAuthActions,
  useAuthState,
  type AuthActions,
  type AuthState,
} from "./AuthProvider";
import { getAuthGateway } from "../auth-gateway";
import { getRuntimeConfig } from "../../../lib/runtime-config";

afterEach(() => cleanup());

beforeEach(() => {
  vi.restoreAllMocks();
  window.history.replaceState({}, "", "/");
});

describe("deriveAuthState", () => {
  it("keeps bootstrap distinct until the canonical session query settles", () => {
    expect(deriveAuthState(undefined, false).status).toBe("bootstrapping");
  });

  it("derives anonymous state without inventing a guest", () => {
    const auth = deriveAuthState(null, true);
    expect(auth.status).toBe("anonymous");
    expect(auth.userId).toBe("");
    expect(auth.canPlayUnranked).toBe(false);
  });

  it("keeps guests visually signed out while retaining guest capabilities", () => {
    const auth = deriveAuthState(
      { accessToken: "guest-token", user: { id: "guest-1", isGuest: true } },
      true,
    );
    expect(auth.status).toBe("guest");
    expect(auth.isRegistered).toBe(false);
    expect(auth.canPlayUnranked).toBe(true);
    expect(auth.canPlayRanked).toBe(false);
    expect(auth.canUseSocial).toBe(false);
  });

  it("blocks play capabilities when the session needs a nickname or migration", () => {
    const auth = deriveAuthState(
      {
        accessToken: "token",
        nicknameRequired: true,
        canPlay: false,
        user: { id: "user-1" },
      },
      true,
    );
    expect(auth.status).toBe("registered");
    expect(auth.canPlayUnranked).toBe(false);
    expect(auth.canPlayRanked).toBe(false);
    expect(auth.canUseSocial).toBe(true);
  });

  it("derives registered identity and profile enrichment from one state", () => {
    const auth = deriveAuthState(
      {
        accessToken: "user-token",
        user: { id: "user-1", email: "user@example.com", display_name: "Session Name" },
      },
      true,
      { display_name: "Profile Name", mmr: 1420, isAdmin: true },
    );
    expect(auth.status).toBe("registered");
    expect(auth.displayName).toBe("Profile Name");
    expect(auth.mmr).toBe(1420);
    expect(auth.isAdmin).toBe(true);
    expect(auth.canUseSocial).toBe(true);
  });
});

describe("global sign-in surface", () => {
  const state = (isGuest: boolean): AuthState => deriveAuthState(
    isGuest
      ? { accessToken: "guest-token", user: { id: "guest-1", isGuest: true } }
      : null,
    true,
  );

  const actions = (overrides: Partial<AuthActions> = {}): AuthActions => ({
    signInOpen: true,
    authLoading: false,
    authError: "",
    googleEnabled: true,
    discordEnabled: true,
    openSignIn: vi.fn(),
    closeSignIn: vi.fn(),
    startProvider: vi.fn(async () => {}),
    devLogin: vi.fn(async () => {}),
    ...overrides,
  });

  it("uses upgrade_guest intent for guests and signin for anonymous users", () => {
    expect(authIntentForState({ isGuest: true })).toBe("upgrade_guest");
    expect(authIntentForState({ isGuest: false })).toBe("signin");
  });

  it("renders provider actions globally and restores focus when closed", async () => {
    const startProvider = vi.fn(async () => {});
    const trigger = document.createElement("button");
    trigger.textContent = "Sign In";
    document.body.appendChild(trigger);
    trigger.focus();
    function Harness() {
      const [open, setOpen] = useState(true);
      return (
        <AuthStateProvider value={state(false)}>
          <AuthActionsProvider value={actions({
            signInOpen: open,
            closeSignIn: () => setOpen(false),
            startProvider,
          })}>
            <AuthSignInModal />
          </AuthActionsProvider>
        </AuthStateProvider>
      );
    }
    render(<Harness />);

    expect(screen.getByRole("dialog", { name: "Sign In" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Continue With Google/ }));
    expect(startProvider).toHaveBeenCalledWith("google");
    fireEvent.click(screen.getByRole("button", { name: "Close Sign In" }));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Sign In" })).not.toBeInTheDocument());
    expect(document.activeElement).toBe(trigger);
    trigger.remove();
  });

  it("keeps guest and anonymous provider controls identical", () => {
    const renderSurface = (isGuest: boolean) => render(
      <AuthStateProvider value={state(isGuest)}>
        <AuthActionsProvider value={actions()}>
          <AuthSignInModal />
        </AuthActionsProvider>
      </AuthStateProvider>,
    );
    const anonymous = renderSurface(false);
    expect(screen.getByRole("button", { name: /Continue With Discord/ })).toBeInTheDocument();
    anonymous.unmount();
    renderSurface(true);
    expect(screen.getByRole("button", { name: /Continue With Google/ })).toBeInTheDocument();
  });
});

describe("AuthProvider oauth callback ownership", () => {
  function AuthProbe() {
    const auth = useAuthState();
    const actions = useAuthActions();
    return (
      <div>
        <span data-testid="status">{auth.status}</span>
        <span data-testid="loading">{String(actions.authLoading)}</span>
        <span data-testid="error">{actions.authError}</span>
        <span data-testid="user">{auth.userId}</span>
      </div>
    );
  }

  it("restores through the OAuth transition and clears callback params without sticking loading", async () => {
    window.history.replaceState({}, "", "/?auth=success&provider=google");
    const gateway = getAuthGateway(getRuntimeConfig());
    const oauthCompleted = vi.spyOn(gateway, "oauthCompleted").mockResolvedValue({
      userId: "user-1",
      accessToken: "token",
      nicknameRequired: false,
      authMigrationRequired: false,
      recoveryAvailable: false,
      linkedProviders: [],
      canPlay: true,
      nicknameInput: "Player",
    });
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <AuthProbe />
        </AuthProvider>
      </QueryClientProvider>,
    );

    await waitFor(() => expect(oauthCompleted).toHaveBeenCalled());
    await waitFor(() => expect(screen.getByTestId("loading")).toHaveTextContent("false"));
    expect(window.location.search).toBe("");
    expect(screen.getByTestId("error")).toHaveTextContent("");
  });

  it("surfaces oauth error callbacks on the sign-in surface", async () => {
    window.history.replaceState({}, "", "/?auth=error&authError=Denied");
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <AuthProbe />
        </AuthProvider>
      </QueryClientProvider>,
    );

    await waitFor(() => expect(screen.getByTestId("error")).toHaveTextContent("Denied"));
    expect(screen.getByRole("dialog", { name: "Sign In" })).toBeInTheDocument();
    expect(window.location.search).toBe("");
  });
});
