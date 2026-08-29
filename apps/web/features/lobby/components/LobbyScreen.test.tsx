import React from "react";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import LobbyScreen from "./LobbyScreen";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  AuthActionsProvider,
  AuthStateProvider,
  deriveAuthState,
  type AuthActions,
} from "../../auth/components/AuthProvider";

function resetStoredQueueRulesets() {
  if (
    typeof window === "undefined" ||
    typeof window.localStorage?.removeItem !== "function"
  ) {
    return;
  }
  window.localStorage.removeItem("geoduels.queueRulesets");
  window.localStorage.removeItem("geoduels.play.duels");
  window.localStorage.removeItem("geoduels.play.singleplayer");
}

function reportExtensionAvailable(extensionVersion = "0.1.3") {
  window.dispatchEvent(
    new MessageEvent("message", {
      source: window,
      origin: window.location.origin,
      data: {
        source: "geoduels-extension",
        version: 1,
        extensionVersion,
        type: "extension_ready",
      },
    }),
  );
}

function renderLobbyScreen(
  overrides?: Partial<React.ComponentProps<typeof LobbyScreen>>,
  authState = deriveAuthState(null, true),
) {
  const props: React.ComponentProps<typeof LobbyScreen> = {
    userId: "self",
    accessToken: "token",
    userEmail: "self@example.com",
    displayName: "Self",
    userAvatar: "",
    isGuest: false,
    connected: true,
    mmr: 1200,
    leaderboard: null,
    leaderboardLoading: false,
    status: "ready",
    queueStartedAt: null,
    joinQueue: vi.fn(),
    startSingleplayer: vi.fn(async () => ""),
    clearSingleplayerError: vi.fn(),
    cancelQueue: vi.fn(),
    queueError: "",
    singleplayerError: "",
    onlinePlayers: 42,
    maintenance: null,
    appVersion: "dev",
    isAdmin: false,
    changelogEyebrow: "News",
    changelogTitle: "Latest",
    changelogMarkdown: "",
    changelogSlug: "",
    changelogUpdatedAt: "",
    onBrowseLeaderboard: vi.fn(),
    authLoading: false,
    authError: "",
    nicknameSaving: false,
    ...overrides,
  };
  const openSignIn = vi.fn();
  const authActions: AuthActions = {
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

  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return {
    ...render(
      <QueryClientProvider client={queryClient}>
        <AuthStateProvider value={authState}>
          <AuthActionsProvider value={authActions}>
            <LobbyScreen {...props} />
          </AuthActionsProvider>
        </AuthStateProvider>
      </QueryClientProvider>,
    ),
    props,
    openSignIn,
  };
}

beforeEach(() => {
  const values = new Map<string, string>();
  Object.defineProperty(window, "localStorage", {
    configurable: true,
    value: {
      clear: () => values.clear(),
      getItem: (key: string) => values.get(key) ?? null,
      removeItem: (key: string) => values.delete(key),
      setItem: (key: string, value: string) => values.set(key, value),
    },
  });
  resetStoredQueueRulesets();
});

afterEach(() => {
  cleanup();
  resetStoredQueueRulesets();
  vi.unstubAllGlobals();
});

describe("LobbyScreen", () => {
  it("replaces only lobby content with a route loading indicator", () => {
    renderLobbyScreen({ contentRoute: "maps", routeLoading: true });

    expect(screen.getByRole("status", { name: "Loading page" })).toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: "Primary navigation" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "GeoDuels home" })).toBeInTheDocument();
    expect(screen.queryByText("Browse and manage maps")).not.toBeInTheDocument();
  });

  it("loads the leaderboard only on the leaderboard route", () => {
    const onBrowseLeaderboard = vi.fn();
    renderLobbyScreen({ onBrowseLeaderboard });

    expect(onBrowseLeaderboard).not.toHaveBeenCalled();

    cleanup();
    renderLobbyScreen({ contentRoute: "top", onBrowseLeaderboard });

    expect(onBrowseLeaderboard).toHaveBeenCalledTimes(1);
  });

  it("requests trending maps while rendering the play route", async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL) => new Response(JSON.stringify([]), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    renderLobbyScreen({ contentRoute: "play" });
    await new Promise((resolve) => window.setTimeout(resolve, 0));

    const requestedURLs = fetchMock.mock.calls.map((call) => String(call[0]));
    expect(requestedURLs.some((url) => url.includes("/v1/maps?scope=official"))).toBe(true);
    expect(screen.getByRole("heading", { name: "Quiz" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Very Soon" })).toBeDisabled();
    expect(screen.getByRole("link", { name: "View all" })).toHaveAttribute("href", "/maps");
    expect(screen.getByRole("link", { name: "How to Play" })).toHaveAttribute("href", "/help");
  });

  it("keeps play and party actions disabled only while auth is bootstrapping", () => {
    renderLobbyScreen({}, deriveAuthState(undefined, false));
    const disabledPlay = screen.getAllByRole("button", { name: "Play" });
    expect(disabledPlay.length).toBeGreaterThan(0);
    expect(disabledPlay.every((button) => (button as HTMLButtonElement).disabled)).toBe(true);

    cleanup();
    renderLobbyScreen(
      {
        userId: "guest-1",
        isGuest: true,
        authLoading: false,
      },
      deriveAuthState(
        { accessToken: "guest-token", user: { id: "guest-1", isGuest: true } },
        true,
      ),
    );
    const enabledPlay = screen.getAllByRole("button", { name: "Play" });
    expect(enabledPlay.every((button) => !(button as HTMLButtonElement).disabled)).toBe(true);
  });

  it("shows the maintenance warning banner and pauses duel queueing", () => {
    const { openSignIn } = renderLobbyScreen({
      maintenance: {
        phase: "warning",
        startsAt: new Date(Date.now() + 5 * 60_000).toISOString(),
        endsAt: "",
        queuePaused: true,
        playPaused: false,
        message: "Deploy window opens shortly.",
      },
    });

    expect(screen.getByText(/Maintenance/i)).toBeInTheDocument();
    expect(
      screen.getByText("Deploy window opens shortly."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Paused" })).toBeDisabled();
  });

  it("opens the duel chooser and requires at least one mode", () => {
    renderLobbyScreen();

    fireEvent.click(screen.getAllByRole("button", { name: "Play" })[0]);

    expect(
      screen.getByRole("dialog", { name: "Find a Duel" }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Moving" }));

    expect(screen.getByRole("button", { name: "Moving" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
    expect(screen.getByRole("button", { name: "Start" })).toBeDisabled();
  });

  it("requests the centralized sign-in surface instead of queueing when signed out players press ranked play", () => {
    const joinQueue = vi.fn();
    const { openSignIn } = renderLobbyScreen({
      userId: "",
      userEmail: "",
      displayName: "",
      joinQueue,
    });

    fireEvent.click(screen.getAllByRole("button", { name: "Play" })[0]);

    expect(openSignIn).toHaveBeenCalledTimes(1);
    expect(joinQueue).not.toHaveBeenCalled();
  });

  it("requests centralized sign-in instead of queueing when guest players press ranked play", () => {
    const joinQueue = vi.fn();
    const { openSignIn } = renderLobbyScreen({
      isGuest: true,
      joinQueue,
    });

    fireEvent.click(screen.getAllByRole("button", { name: "Play" })[0]);

    expect(openSignIn).toHaveBeenCalledTimes(1);
    expect(joinQueue).not.toHaveBeenCalled();
  });

  it("reveals extension setup only after selecting an extension-only ranked mode", () => {
    renderLobbyScreen();

    fireEvent.click(screen.getAllByRole("button", { name: "Play" })[0]);

    expect(screen.getByRole("button", { name: "No Move" })).toBeEnabled();
    expect(
      screen.queryByRole("button", { name: "NMPZ" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: /Chrome setup/i }),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "No Move" }));
    expect(
      screen.getByRole("link", { name: /Chrome setup/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Firefox setup/i }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Start" })).toBeDisabled();
  });

  it("requires users with an outdated extension to update before extension-only modes", () => {
    renderLobbyScreen();
    reportExtensionAvailable("0.1.2");

    fireEvent.click(screen.getAllByRole("button", { name: "Play" })[0]);

    expect(
      screen.queryByText(/update the official GeoDuels browser extension/i),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "No Move" }));
    expect(
      screen.getByText(/update the official GeoDuels browser extension/i),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Chrome update/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Firefox update/i }),
    ).toBeInTheDocument();
  });

  it("migrates legacy duel selections to the supported ranked mode", () => {
    window.localStorage.setItem(
      "geoduels.queueRulesets",
      JSON.stringify(["moving", "nmpz"]),
    );
    renderLobbyScreen();
    reportExtensionAvailable();

    fireEvent.click(screen.getAllByRole("button", { name: "Play" })[0]);

    expect(screen.getByRole("button", { name: "Moving" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(
      screen.queryByRole("button", { name: "NMPZ" }),
    ).not.toBeInTheDocument();
  });

  it("queues registered players for the two supported ranked variants", async () => {
    const joinQueue = vi.fn();
    renderLobbyScreen({ joinQueue });
    reportExtensionAvailable();

    fireEvent.click(screen.getAllByRole("button", { name: "Play" })[0]);
    expect(
      await screen.findByRole("dialog", { name: "Find a Duel" }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "No Move" }));
    fireEvent.click(screen.getByRole("button", { name: "Start" }));

    expect(joinQueue).toHaveBeenCalledWith(["moving", "no_move_hidden"]);
  });

  it("uses radio modes and no Any visibility for singleplayer", async () => {
    const startSingleplayer = vi.fn(async () => "");
    renderLobbyScreen({ startSingleplayer });
    reportExtensionAvailable();

    fireEvent.click(screen.getAllByRole("button", { name: "Play" })[1]);
    expect(
      await screen.findByRole("dialog", { name: "Start Singleplayer" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("switch", { name: "Hide street names" }),
    ).not.toBeChecked();
    fireEvent.click(screen.getByRole("button", { name: "No Move" }));
    expect(screen.getByRole("button", { name: "Moving" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
    fireEvent.click(screen.getByRole("switch", { name: "Hide street names" }));
    fireEvent.click(screen.getByRole("button", { name: "Start" }));

    expect(startSingleplayer).toHaveBeenCalledWith(
      {
        ruleset: "no_move",
        streetNames: "hidden",
      },
      { kind: "home" },
    );
  });

  it("keeps a singleplayer launch failure in its modal and out of the Duel card", async () => {
    renderLobbyScreen({ singleplayerError: "Unable to create session" });

    expect(screen.queryByText("Unable to create session")).not.toBeInTheDocument();
    fireEvent.click(screen.getAllByRole("button", { name: "Play" })[1]);

    expect(await screen.findByRole("alert")).toHaveTextContent("Unable to create session");
  });

  it("keeps the singleplayer modal open when starting does not create a match", async () => {
    const startSingleplayer = vi.fn(async () => "");
    renderLobbyScreen({ startSingleplayer });

    fireEvent.click(screen.getAllByRole("button", { name: "Play" })[1]);
    await screen.findByRole("dialog", { name: "Start Singleplayer" });
    fireEvent.click(screen.getByRole("button", { name: "Start" }));

    await waitFor(() => expect(startSingleplayer).toHaveBeenCalledTimes(1));
    expect(screen.getByRole("dialog", { name: "Start Singleplayer" })).toBeInTheDocument();
  });

  it("restores the last selected singleplayer options", async () => {
    renderLobbyScreen();
    reportExtensionAvailable();

    fireEvent.click(screen.getAllByRole("button", { name: "Play" })[1]);
    await screen.findByRole("dialog", { name: "Start Singleplayer" });
    fireEvent.click(screen.getByRole("button", { name: "NMPZ" }));
    fireEvent.click(screen.getByRole("switch", { name: "Hide street names" }));

    cleanup();
    renderLobbyScreen();

    fireEvent.click(screen.getAllByRole("button", { name: "Play" })[1]);
    await screen.findByRole("dialog", { name: "Start Singleplayer" });
    expect(screen.getByRole("button", { name: "NMPZ" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(
      screen.getByRole("switch", { name: "Hide street names" }),
    ).toBeChecked();
  });

  it("lets singleplayer select the fake toggle and then reveals extension setup", async () => {
    renderLobbyScreen();

    fireEvent.click(screen.getAllByRole("button", { name: "Play" })[1]);
    expect(
      await screen.findByRole("dialog", { name: "Start Singleplayer" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "NMPZ" })).toBeInTheDocument();
    const streetNames = screen.getByRole("switch", {
      name: "Hide street names",
    });
    expect(streetNames).toBeEnabled();
    expect(
      screen.queryByRole("link", { name: /Chrome setup/i }),
    ).not.toBeInTheDocument();
    fireEvent.click(streetNames);
    expect(streetNames).toBeChecked();
    expect(
      screen.getByRole("link", { name: /Chrome setup/i }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Start" })).toBeDisabled();
  });

  it("shows singleplayer as loading while a start is connecting", () => {
    renderLobbyScreen({ status: "matched_connecting" });

    const loadingButton = screen.getByRole("button", { name: /Loading\.\.\.$/ });

    expect(loadingButton).toBeDisabled();
    expect(loadingButton.querySelector(".animate-spin")).toBeInTheDocument();
  });

  it("keeps queueing navigation available while global activity owns cancellation", () => {
    renderLobbyScreen({ status: "queueing", queueStartedAt: Date.now() });
    expect(screen.getByRole("link", { name: "Maps" })).toHaveAttribute("href", "/maps");
  });

  it("keeps an active party in the background when the route is home", () => {
    renderLobbyScreen({
      contentRoute: "play",
      party: {
        status: "ready",
        snapshot: {
          id: "party-1",
          inviteCode: "ABCD12",
          ownerUserId: "self",
          state: "open",
          mode: "duel",
          mapScope: "world",
          members: [{ userId: "self", displayName: "Self", role: "owner", connected: true }],
        },
        inviteCode: "ABCD12",
        isOwner: true,
        busy: false,
        error: "",
      },
    });

    expect(screen.getByRole("heading", { level: 1, name: "Duel" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Invite friends" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Copy ABCD12" })).not.toBeInTheDocument();
  });

  it("replaces the tabbed lobby content when an invite lobby is active", async () => {
    renderLobbyScreen({
      contentRoute: "party",
      party: {
        status: "ready",
        snapshot: {
          id: "party-1",
          inviteCode: "ABCD12",
          ownerUserId: "self",
          state: "open",
          mode: "duel",
          mapScope: "world",
          members: [
            {
              userId: "self",
              displayName: "Self",
              role: "owner",
              connected: true,
            },
          ],
        },
        inviteCode: "ABCD12",
        isOwner: true,
        busy: false,
        error: "",
      },
    });

    expect(screen.getByRole("button", { name: "Invite friends" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Copy ABCD12" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Leave" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Private Party" })).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "FRIENDS" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "PLAY" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "TOP" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Tutorial")).not.toBeInTheDocument();

    const fetchMock = vi.fn(async () =>
      new Response(
        JSON.stringify({
          friends: [
            {
              userId: "friend-1",
              displayName: "Ada",
              relationship: "friends",
              presenceStatus: "online",
            },
          ],
          requests: { incoming: [], outgoing: [] },
          recentPlayers: [],
        }),
        { status: 200 },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);
    fireEvent.click(screen.getByRole("button", { name: "Invite friends" }));
    expect(await screen.findByRole("heading", { name: "Invite friends" })).toBeInTheDocument();
    expect(await screen.findByText("Ada")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Invite Ada to party" })).toBeInTheDocument();
  });

  it("keeps mobile party teams and controls in one scrollable flow", () => {
    renderLobbyScreen({
      contentRoute: "party",
      party: {
        status: "ready",
        snapshot: {
          id: "party-1",
          inviteCode: "ABCD12",
          ownerUserId: "self",
          state: "open",
          mode: "team_duel",
          mapScope: "world",
          members: [
            { userId: "self", displayName: "Self", role: "owner", connected: true, teamId: "a" },
            { userId: "blue", displayName: "Blue Player", role: "member", connected: true, teamId: "b" },
          ],
        },
        inviteCode: "ABCD12",
        isOwner: true,
        busy: false,
        error: "",
      },
    });

    const main = screen.getByRole("main");
    expect(main).toHaveClass("overflow-y-auto", "md:overflow-hidden");

    const mainText = main.textContent ?? "";
    expect(mainText.indexOf("Team Blue")).toBeLessThan(mainText.indexOf("Switch teams"));
    expect(mainText.indexOf("Switch teams")).toBeLessThan(mainText.indexOf("Team Red"));
    expect(mainText.indexOf("Team Red")).toBeLessThan(mainText.indexOf("Multipliers"));
    expect(mainText.indexOf("Multipliers")).toBeLessThan(mainText.indexOf("Start Team Duel"));

    const bluePanel = screen.getByText("Team Blue").closest("section");
    expect(bluePanel?.querySelector(".overflow-visible")).toHaveClass(
      "md:overflow-y-auto",
    );
  });

  it("keeps party admission on the friends surface before a snapshot arrives", () => {
    renderLobbyScreen({
      contentRoute: "party",
      party: {
        status: "admitting",
        snapshot: null,
        inviteCode: "ABCD12",
        isOwner: false,
        busy: true,
        error: "",
      },
    });

    expect(screen.getByText("Create a party or join your friend")).toBeInTheDocument();
    expect(screen.queryByText("Connecting to party")).not.toBeInTheDocument();
  });

  it("does not render a party screen for a snapshot that is still admitting", () => {
    renderLobbyScreen({
      contentRoute: "friends",
      party: {
        status: "admitting",
        snapshot: {
          id: "party-1",
          inviteCode: "ABCD12",
          ownerUserId: "self",
          state: "open",
          mode: "duel",
          mapScope: "world",
          members: [
            {
              userId: "self",
              displayName: "Self",
              role: "owner",
              connected: true,
            },
          ],
        },
        inviteCode: "ABCD12",
        isOwner: false,
        busy: true,
        error: "",
      },
    });

    expect(
      screen.queryByText("Share the invite, wait for one opponent, then the leader starts the match."),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Join Party" }),
    ).not.toBeInTheDocument();
  });

  it("disables party start and marks players outside the party", () => {
    renderLobbyScreen({
      contentRoute: "party",
      party: {
        status: "ready",
        snapshot: {
          id: "party-1",
          inviteCode: "ABCD12",
          ownerUserId: "self",
          state: "open",
          mode: "duel",
          mapScope: "world",
          members: [
            {
              userId: "self",
              displayName: "Self",
              role: "owner",
              connected: true,
            },
            {
              userId: "opponent",
              displayName: "Opponent",
              role: "member",
              connected: false,
            },
          ],
        },
        inviteCode: "ABCD12",
        isOwner: true,
        busy: false,
        error: "",
      },
    });

    expect(screen.getByText("You · Online")).toBeInTheDocument();
    expect(screen.getByText("Offline")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Start Duel" })).toBeDisabled();
  });

  it("offers reconnect only to players in the active match roster", () => {
    renderLobbyScreen({
      contentRoute: "party",
      party: {
        status: "ready",
        snapshot: {
          id: "party-1",
          inviteCode: "ABCD12",
          ownerUserId: "opponent",
          state: "in_match",
          mode: "duel",
          mapScope: "world",
          activeMatchId: "match-1",
          members: [
            {
              userId: "self",
              displayName: "Self",
              role: "member",
              inActiveMatch: true,
            },
            {
              userId: "opponent",
              displayName: "Opponent",
              role: "owner",
              inActiveMatch: true,
            },
          ],
        },
        inviteCode: "ABCD12",
        isOwner: false,
        busy: false,
        error: "",
      },
    });

    expect(
      screen.getByText(
        "You are part of this game and can reconnect whenever you are ready.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Reconnect to Game" }),
    ).toHaveAttribute("href", "/match/match-1");
  });

  it("keeps late lobby members out of the active game", () => {
    renderLobbyScreen({
      contentRoute: "party",
      party: {
        status: "ready",
        snapshot: {
          id: "party-1",
          inviteCode: "ABCD12",
          ownerUserId: "opponent",
          state: "in_match",
          mode: "duel",
          mapScope: "world",
          activeMatchId: "match-1",
          members: [
            { userId: "self", displayName: "Self", role: "member" },
            {
              userId: "opponent",
              displayName: "Opponent",
              role: "owner",
              inActiveMatch: true,
            },
          ],
        },
        inviteCode: "ABCD12",
        isOwner: false,
        busy: false,
        error: "",
      },
    });

    expect(
      screen.getByText(
        "You joined after this game started and will be able to play in the next one.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Reconnect to Game" }),
    ).not.toBeInTheDocument();
  });

  it("opens invite lobby choices and joins with a typed code", () => {
    const joinParty = vi.fn(async () => true);
    renderLobbyScreen({ joinParty });

    expect(
      screen.queryByRole("button", { name: /Private Party/i }),
    ).not.toBeInTheDocument();

    cleanup();
    renderLobbyScreen({ contentRoute: "friends", joinParty });

    expect(screen.getByText("Custom")).toBeInTheDocument();
    expect(
      screen.getByText("Create a party or join your friend"),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Private Party/i }));

    expect(
      screen.getByRole("dialog", { name: "Private Party" }),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Join With Code"), {
      target: { value: "abcd12" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Join" }));

    expect(joinParty).toHaveBeenCalledWith("ABCD12");
  });

  it("keeps party admission errors on the friends surface and in its modal", () => {
    renderLobbyScreen({
      contentRoute: "friends",
      party: {
        status: "error",
        snapshot: null,
        inviteCode: "BAD123",
        isOwner: false,
        busy: false,
        error: "Party not found",
      },
    });

    expect(screen.getByRole("alert")).toHaveTextContent("Party not found");
    expect(
      screen.queryByText("Share the invite, wait for one opponent, then the leader starts the match."),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText("Create a party or join your friend"),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Private Party/i }));

    expect(screen.getByRole("dialog", { name: "Private Party" })).toHaveTextContent(
      "Party not found",
    );
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
