import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { TooltipProvider } from "../../../components/ui/Tooltip";
import type { PublicPlayerProfile } from "../types";
import { PlayerProfilePage } from "./PlayerProfilePage";

const hookMocks = vi.hoisted(() => ({
  usePlayerProfile: vi.fn(),
  useOptionalViewer: vi.fn(),
  useProfileOwnerActions: vi.fn(),
  router: {
    isReady: true,
    pathname: "/players/[id]",
    query: { id: "player-1" } as Record<string, string>,
    replace: vi.fn(),
    push: vi.fn(),
  },
}));

vi.mock("../hooks/use-player-profile", () => hookMocks);
vi.mock("../hooks/use-profile-mutations", () => ({
  useProfileOwnerActions: hookMocks.useProfileOwnerActions,
}));
vi.mock("next/router", () => ({
  useRouter: () => hookMocks.router,
}));
vi.mock("./AccountSettingsModal", () => ({
  AccountSettingsModal: () => <div role="dialog" aria-label="Account settings" />,
}));

const profile: PublicPlayerProfile = {
  userId: "player-1",
  displayName: "Atlas",
  avatarUrl: "/atlas.png",
  mmr: 1432,
  seasonId: "s3",
  gamesPlayed: 20,
  wins: 12,
  rankedGamesPlayed: 14,
  rankedWins: 8,
  selectedBadge: {
    id: "badge-1",
    kind: "achievement",
    label: "World Walker",
    description: "Visited every continent.",
    imageUrl: "/badge.png",
  },
  badges: [
    {
      id: "badge-1",
      kind: "achievement",
      label: "World Walker",
      description: "Visited every continent.",
      imageUrl: "/badge.png",
    },
    {
      id: "badge-2",
      kind: "achievement",
      label: "Speedrunner",
      description: "Guessed quickly.",
      imageUrl: "/speed.png",
    },
  ],
};

const match = {
  matchId: "match-1",
  mode: "duel",
  endedAt: "2026-06-20T12:30:00.000Z",
  outcome: "win" as const,
  ranked: true,
  ratingDelta: 14,
  totalScore: 19420,
  opponentUserId: "player-2",
  opponentDisplayName: "Rival",
};
const singleplayerMatch = {
  matchId: "match-2",
  mode: "singleplayer",
  endedAt: "2026-06-23T11:30:00.000Z",
  outcome: "completed" as const,
  ranked: false,
  ratingDelta: 50,
  totalScore: 10250,
};

function arrange({
  viewerId = "player-1",
  matches = [match, singleplayerMatch],
  hasNextPage = true,
}: {
  viewerId?: string;
  matches?: Array<typeof match | typeof singleplayerMatch>;
  hasNextPage?: boolean;
} = {}) {
  const fetchNextPage = vi.fn();
  hookMocks.usePlayerProfile.mockReturnValue({
    profileQuery: {
      data: profile,
      isLoading: false,
      isError: false,
    },
    matchesQuery: {
      data: { pages: [{ matches }] },
      isLoading: false,
      isError: false,
      hasNextPage,
      isFetchingNextPage: false,
      fetchNextPage,
    },
  });
  hookMocks.useOptionalViewer.mockReturnValue({
    data: {
      userId: viewerId,
      accessToken: "token",
      isGuest: false,
      displayName: viewerId === "player-1" ? "Atlas" : "Viewer",
      mmr: 1432,
      selectedBadge: profile.selectedBadge,
    },
  });
  hookMocks.useProfileOwnerActions.mockReturnValue({
    nicknameMutation: {
      mutate: vi.fn(),
      isPending: false,
      isError: false,
      error: null,
    },
    badgeMutation: {
      mutate: vi.fn(),
      isPending: false,
      isError: false,
      error: null,
    },
  });

  render(
    <TooltipProvider>
      <PlayerProfilePage playerId="player-1" initialProfile={profile} />
    </TooltipProvider>,
  );
  return { fetchNextPage };
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-06-23T12:30:00.000Z"));
  hookMocks.router.query = { id: "player-1" };
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  window.sessionStorage.clear();
  vi.useRealTimers();
});

describe("PlayerProfilePage", () => {
  it("renders compact stats, badges, match details, and owner settings", async () => {
    const { fetchNextPage } = arrange();

    expect(screen.getByRole("heading", { name: "Atlas" })).toBeInTheDocument();
    expect(screen.getAllByLabelText("1432 MMR")).toHaveLength(2);
    expect(screen.getByText("Duels played")).toBeInTheDocument();
    expect(screen.getByText("Duel wins")).toBeInTheDocument();
    expect(screen.getByText("Duel win rate")).toBeInTheDocument();
    expect(screen.getByText("Ranked duels")).toBeInTheDocument();
    expect(screen.getByText("20")).toBeInTheDocument();
    expect(screen.getByText("12")).toBeInTheDocument();
    expect(screen.getByText("60%")).toBeInTheDocument();
    expect(screen.getByText("14")).toBeInTheDocument();
    expect(screen.getByText("8")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Account settings" })).toBeInTheDocument();

    const badge = screen
      .getAllByLabelText("World Walker - Visited every continent.")
      .find((element) => element.getAttribute("tabindex") === "0");
    expect(badge).toBeTruthy();
    fireEvent.focus(badge!);
    expect(screen.getByRole("tooltip")).toHaveTextContent(
      "World WalkerVisited every continent.",
    );

    expect(screen.getByText("win")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "All" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Ranked" })).toBeInTheDocument();
    expect(screen.getByText("vs Rival")).toBeInTheDocument();
    expect(screen.getByText("Singleplayer")).toBeInTheDocument();
    expect(screen.getAllByText("Ranked")).toHaveLength(2);
    expect(screen.queryByText("19,420")).not.toBeInTheDocument();
    expect(screen.getByText("+14")).toBeInTheDocument();
    expect(screen.getByText("3d ago")).toBeInTheDocument();
    expect(screen.getByText("10,250")).toBeInTheDocument();
    expect(screen.queryByText("+50")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Load more" }));
    expect(fetchNextPage).toHaveBeenCalledTimes(1);
  });

  it("requests ranked match history from the Ranked tab", () => {
    arrange();

    fireEvent.click(screen.getByRole("button", { name: "Ranked" }));

    expect(hookMocks.usePlayerProfile).toHaveBeenLastCalledWith(
      "player-1",
      profile,
      "ranked",
    );
  });

  it("hides owner controls and keeps the empty history state", () => {
    arrange({ viewerId: "viewer-2", matches: [], hasNextPage: false });

    expect(screen.queryByRole("button", { name: "Account settings" })).not.toBeInTheDocument();
    expect(screen.getByText("No match history yet.")).toBeInTheDocument();
    expect(
      screen
        .getAllByRole("link")
        .filter((link) => link.hasAttribute("aria-current")),
    ).toHaveLength(0);
  });

  it("keeps medals presentational until the owner opens badge selection", () => {
    const badgeMutate = vi.fn();
    arrange();
    hookMocks.useProfileOwnerActions.mock.results[0].value.badgeMutation.mutate =
      badgeMutate;

    expect(
      screen.queryByRole("button", { name: "Display World Walker" }),
    ).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Choose displayed badge" }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Display Speedrunner" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(badgeMutate).toHaveBeenCalledWith(
      "badge-2",
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    );
  });

  it("allows the owner to clear the displayed badge", () => {
    const badgeMutate = vi.fn();
    arrange();
    hookMocks.useProfileOwnerActions.mock.results[0].value.badgeMutation.mutate =
      badgeMutate;

    fireEvent.click(
      screen.getByRole("button", { name: "Choose displayed badge" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "None" }));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(badgeMutate).toHaveBeenCalledWith(
      "",
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    );
  });

  it("edits the display name separately from confidential settings", () => {
    const nicknameMutate = vi.fn();
    arrange();
    hookMocks.useProfileOwnerActions.mock.results[0].value.nicknameMutation.mutate =
      nicknameMutate;

    fireEvent.click(screen.getByRole("button", { name: "Edit display name" }));
    fireEvent.change(screen.getByDisplayValue("Atlas"), {
      target: { value: "AtlasTwo" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save display name" }));

    expect(nicknameMutate).toHaveBeenCalledWith(
      "AtlasTwo",
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    );
  });

  it("opens confidential settings from the owner profile query flag", () => {
    hookMocks.router.query = { id: "player-1", settings: "account" };
    arrange();

    expect(
      screen.getByRole("dialog", { name: "Account settings" }),
    ).toBeInTheDocument();
  });

  it("renders the profile loading state inside the shared shell", () => {
    hookMocks.usePlayerProfile.mockReturnValue({
      profileQuery: { data: undefined, isLoading: true, isError: false },
      matchesQuery: { data: undefined },
    });
    hookMocks.useOptionalViewer.mockReturnValue({ data: null });

    const { container } = render(
      <TooltipProvider>
        <PlayerProfilePage playerId="player-1" />
      </TooltipProvider>,
    );

    expect(container.querySelector(".animate-pulse")).toBeInTheDocument();
    expect(screen.getAllByRole("navigation", { name: "Primary navigation" })).toHaveLength(1);
  });

  it("renders the unavailable profile state", () => {
    hookMocks.usePlayerProfile.mockReturnValue({
      profileQuery: { data: undefined, isLoading: false, isError: true },
      matchesQuery: { data: undefined },
    });
    hookMocks.useOptionalViewer.mockReturnValue({ data: null });

    render(
      <TooltipProvider>
        <PlayerProfilePage playerId="missing-player" />
      </TooltipProvider>,
    );

    expect(
      screen.getByRole("heading", { name: "Player not found" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("This profile does not exist or is no longer available."),
    ).toBeInTheDocument();
  });
});
