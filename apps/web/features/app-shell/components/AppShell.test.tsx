import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppShell } from "./AppShell";
import { AuthActionsProvider, AuthStateProvider, type AuthActions, type AuthState } from "../../auth/components/AuthProvider";

vi.mock("../../notifications/components/NotificationCenter", () => ({
  NotificationCenter: () => <button type="button" aria-label="Notifications" />,
}));

afterEach(() => {
  cleanup();
  window.sessionStorage.clear();
});

describe("AppShell", () => {
  it("keeps brand, navigation, and account controls in one header layout", () => {
    render(
      <AppShell activeNavRoute="play">
        <div>Content</div>
      </AppShell>,
    );

    const header = screen.getByRole("banner");
    const navigation = screen.getByRole("navigation", {
      name: "Primary navigation",
    });

    expect(header).toHaveClass("fixed", "inset-x-0", "top-0");
    expect(navigation.parentElement).toHaveClass(
      "app-shell-navigation",
      "fixed",
      "bottom-[max(0.75rem,env(safe-area-inset-bottom))]",
      "md:bottom-auto",
      "md:top-5",
    );
    expect(navigation.parentElement?.parentElement).toHaveClass(
      "app-shell-header-layout",
    );
  });

  it("reserves the mobile navigation footprint for viewport-locked content", () => {
    render(
      <AppShell activeNavRoute="play" viewportLocked>
        <div>Locked content</div>
      </AppShell>,
    );

    expect(screen.getByText("Locked content").parentElement).toHaveClass(
      "app-shell-mobile-nav-safe-area",
    );
  });

  it("orders navigation routes and renders separate online presence", () => {
    render(
      <AppShell activeNavRoute="play" onlinePlayers={42}>
        <div>Content</div>
      </AppShell>,
    );

    const navigation = screen.getByRole("navigation", {
      name: "Primary navigation",
    });
    expect(
      Array.from(navigation.querySelectorAll("a")).map((link) =>
        link.textContent?.trim(),
      ),
    ).toEqual(["Play", "Maps", "Friends", "Top"]);
    expect(screen.getByLabelText("42 players online")).toHaveTextContent("42");
    expect(screen.queryByText("42 Playing")).not.toBeInTheDocument();
  });

  it("marks the active route in desktop and mobile navigation", async () => {
    render(
      <AppShell activeNavRoute="maps">
        <div>Content</div>
      </AppShell>,
    );

    const mapLinks = await screen.findAllByRole("link", { name: "Maps" });
    expect(mapLinks).toHaveLength(1);
    expect(mapLinks.every((link) => link.getAttribute("aria-current") === "page")).toBe(true);
  });

  it("does not let a previously visited route replace the server-selected route during hydration", () => {
    window.sessionStorage.setItem("geoduels.lobbyRoute", "play");

    render(
      <AppShell activeNavRoute="friends">
        <div>Friends</div>
      </AppShell>,
    );

    expect(screen.getByRole("link", { name: "Friends" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByRole("link", { name: "Play" })).not.toHaveAttribute(
      "aria-current",
    );
  });

  it("supports contextual pages with no selected navigation item", () => {
    render(
      <AppShell activeNavRoute={null}>
        <div>Profile</div>
      </AppShell>,
    );

    expect(
      screen
        .getAllByRole("link")
        .filter((link) => link.hasAttribute("aria-current")),
    ).toHaveLength(0);
  });

  it("replaces online presence with queue and party tasks in one nav surface", () => {
    const onCancel = vi.fn();
    render(
      <AppShell
        activeNavRoute="maps"
        onlinePlayers={42}
        tasks={[
          {
            kind: "queue",
            label: "Finding a duel · 0:12",
            onCancel,
          },
          {
            kind: "party",
            label: "In party — ABCD12",
            href: "/party/ABCD12",
          },
        ]}
      >
        <div>Maps</div>
      </AppShell>,
    );

    expect(screen.getByLabelText("Active tasks")).toBeInTheDocument();
    expect(screen.queryByLabelText("42 players online")).not.toBeInTheDocument();
    expect(screen.getByText("Finding a duel · 0:12")).toBeInTheDocument();
    expect(screen.getByText("In party — ABCD12")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Cancel matchmaking" }));
    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("link", { name: "Return to party lobby" })).toHaveAttribute(
      "href",
      "/party/ABCD12",
    );
  });

  const authFixture = (overrides: Partial<AuthState>): AuthState => ({
    status: "anonymous",
    session: null,
    accessToken: "",
    userId: "",
    isGuest: false,
    isRegistered: false,
    isAdmin: false,
    isModerator: false,
    displayName: "",
    avatarUrl: "",
    email: "",
    canPlayUnranked: false,
    canPlayRanked: false,
    canUseSocial: false,
    canManageMaps: false,
    ...overrides,
  });

  it("keeps bootstrapping chrome neutral and anonymous and guest chrome identical", () => {
    const { container, unmount } = render(
      <AuthStateProvider value={authFixture({ status: "bootstrapping" })}>
        <AppShell activeNavRoute={null}><div /></AppShell>
      </AuthStateProvider>,
    );
    expect(screen.queryByRole("button", { name: "Sign In" })).not.toBeInTheDocument();
    const bootstrapHeader = container.querySelector("header")?.textContent;
    unmount();

    const anonymous = render(
      <AuthStateProvider value={authFixture({})}>
        <AppShell activeNavRoute={null}><div /></AppShell>
      </AuthStateProvider>,
    );
    const anonymousHeader = anonymous.container.querySelector("header")?.textContent;
    anonymous.unmount();
    render(
      <AuthStateProvider value={authFixture({ status: "guest", userId: "guest-1", accessToken: "guest", isGuest: true, canPlayUnranked: true })}>
        <AppShell activeNavRoute={null}><div /></AppShell>
      </AuthStateProvider>,
    );
    expect(screen.getByRole("button", { name: "Sign In" })).toBeInTheDocument();
    expect(screen.getByRole("banner").textContent).toBe(anonymousHeader);
    expect(bootstrapHeader).not.toContain("Sign In");
  });

  it("renders registered profile, settings, and notification chrome centrally", () => {
    render(
      <AuthStateProvider value={authFixture({
        status: "registered",
        userId: "user-1",
        accessToken: "token",
        isRegistered: true,
        displayName: "Atlas",
        canUseSocial: true,
      })}>
        <AppShell activeNavRoute={null}><div /></AppShell>
      </AuthStateProvider>,
    );
    expect(screen.getByRole("button", { name: "Open settings" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Notifications" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Atlas/ })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Sign In" })).not.toBeInTheDocument();
  });

  it("shows the equipped badge beside ELO on desktop and beside the avatar on mobile", () => {
    render(
      <AuthStateProvider value={authFixture({
        status: "registered",
        userId: "user-1",
        accessToken: "token",
        isRegistered: true,
        displayName: "Atlas",
        mmr: 1432,
        selectedBadge: {
          id: "badge-1",
          kind: "achievement",
          label: "World Walker",
          description: "Visited every continent.",
          imageUrl: "/badge.png",
        },
      })}>
        <AppShell activeNavRoute={null}><div /></AppShell>
      </AuthStateProvider>,
    );

    const badges = screen.getAllByLabelText("World Walker - Visited every continent.");
    expect(badges).toHaveLength(2);
    expect(badges.some((badge) => badge.className.includes("sm:hidden"))).toBe(true);
    const desktopBadge = badges.find((badge) => badge.className.includes("sm:inline-flex"));
    expect(desktopBadge).toBeTruthy();
    expect(screen.getByText("Atlas").parentElement).not.toHaveTextContent("World Walker");
    expect(desktopBadge?.parentElement).toHaveTextContent("1,432");
  });

  it.each(["home", "notifications", "profile/friend"]) (
    "uses the shared auth action when Sign In is pressed from the %s shell context",
    (context) => {
      const openSignIn = vi.fn();
      const actions: AuthActions = {
        signInOpen: false,
        authLoading: false,
        authError: "",
        googleEnabled: false,
        discordEnabled: false,
        openSignIn,
        closeSignIn: vi.fn(),
        startProvider: vi.fn(async () => {}),
        devLogin: vi.fn(async () => {}),
      };
      const view = render(
        <AuthStateProvider value={authFixture({})}>
          <AuthActionsProvider value={actions}>
            <AppShell activeNavRoute={null}><div>{context}</div></AppShell>
          </AuthActionsProvider>
        </AuthStateProvider>,
      );
      fireEvent.click(screen.getByRole("button", { name: "Sign In" }));
      expect(openSignIn).toHaveBeenCalledTimes(1);
      view.unmount();
    },
  );
});
