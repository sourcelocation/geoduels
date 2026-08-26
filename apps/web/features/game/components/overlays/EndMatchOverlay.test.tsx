import type { ComponentProps } from "react";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import EndMatchOverlay from "./EndMatchOverlay";

vi.mock("next/dynamic", () => ({
  default: () => () => <div data-testid="guess-map" />,
}));

type Props = ComponentProps<typeof EndMatchOverlay>;

function createProps(overrides: Partial<Props> = {}): Props {
  return {
    onLeaveGame: vi.fn(),
    mode: "duel",
    outcome: "win",
    selfUserId: "self",
    sides: {
      self: {
        id: "self",
        participant: {
          kind: "player",
          id: "self",
          name: "Self",
          avatarFallback: "S",
        },
        hp: 5000,
        connection: "connected",
      },
      opponent: {
        id: "opponent",
        participant: {
          kind: "player",
          id: "opponent",
          name: "Opponent",
          avatarFallback: "O",
        },
        hp: 0,
        connection: "connected",
      },
    },
    totalScore: 9000,
    maxHP: 6000,
    roundResults: [
      {
        roundId: "round-1",
        roundNumber: 1,
        actualLocation: { lat: 0, lng: 0 },
        players: {
          self: {
            userId: "self",
            lat: 1,
            lng: 1,
            distanceKm: 10,
            score: 4500,
            hpAfterRound: 5000,
            guessMs: 2500,
          },
          opponent: {
            userId: "opponent",
            lat: 2,
            lng: 2,
            distanceKm: 500,
            score: 1000,
            hpAfterRound: 0,
            guessMs: 7000,
          },
        },
      },
    ],
    resultPlayerNames: { self: "Self", opponent: "Opponent" },
    resultPlayerAvatars: {},
    resultPlayerFallbacks: { self: "S", opponent: "O" },
    ...overrides,
  };
}

function openBreakdown() {
  fireEvent.click(screen.getByRole("button", { name: "Breakdown" }));
}

describe("EndMatchOverlay breakdown", () => {
  afterEach(cleanup);

  it("shows each player's post-round health for a duel, including zero health", () => {
    render(<EndMatchOverlay {...createProps()} />);
    openBreakdown();

    expect(screen.getByRole("button", { name: "health" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "points" })).toHaveAttribute("aria-pressed", "false");
    expect(screen.getByText("5,000 HP")).toBeInTheDocument();
    expect(screen.getByText("0 HP")).toBeInTheDocument();
    expect(screen.getByRole("progressbar", { name: "Self health" }).firstChild).toHaveStyle({
      width: "83.33333333333334%",
      backgroundImage: "linear-gradient(90deg, rgb(var(--gd-status-success) / 1) 0%, rgb(var(--gd-status-success) / 0.72) 100%)",
    });
    expect(screen.getByRole("progressbar", { name: "Opponent health" }).firstChild).toHaveStyle({
      width: "0%",
      backgroundImage: "linear-gradient(180deg, rgb(var(--gd-status-danger) / 1) 0%, rgb(var(--gd-status-danger) / 0.72) 100%)",
    });
    expect(screen.getByText("2.5s")).toHaveClass("text-content-secondary");
    expect(screen.getByText("7.0s")).toHaveClass("text-content-secondary");
    expect(screen.queryByText("4,500")).not.toBeInTheDocument();
    expect(screen.queryByText("Total")).not.toBeInTheDocument();
  });

  it("shows final rating deltas without outcome forecasts or brackets", () => {
    render(<EndMatchOverlay {...createProps({
      sides: {
        self: {
          ...createProps().sides.self,
          participant: { kind: "player", id: "self", name: "Self", avatarFallback: "S", rating: 1200, ratingDelta: 40, ratingPreview: { win: 40, lose: 0, draw: 0 } },
        },
        opponent: {
          ...createProps().sides.opponent,
          participant: { kind: "player", id: "opponent", name: "Opponent", avatarFallback: "O", rating: 1200, ratingDelta: 0, ratingPreview: { win: 40, lose: 0, draw: 0 } },
        },
      },
    })} />);

    expect(screen.getByText("Win")).toHaveClass("text-display-md", "sm:text-display-lg");
    expect(screen.getByText("+40")).toBeInTheDocument();
    expect(screen.queryByText(/W \+40/)).not.toBeInTheDocument();
    expect(screen.queryByText(/L 0/)).not.toBeInTheDocument();
    expect(screen.queryByText("(+40)")).not.toBeInTheDocument();
  });

  it("switches a duel breakdown between health and points", () => {
    render(<EndMatchOverlay {...createProps()} />);
    openBreakdown();

    fireEvent.click(screen.getByRole("button", { name: "points" }));

    expect(screen.getByRole("button", { name: "points" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByText("4,500 pts")).toBeInTheDocument();
    expect(screen.getByText("1,000 pts")).toBeInTheDocument();
    expect(screen.queryByText("5,000 HP")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "health" }));

    expect(screen.getByText("5,000 HP")).toBeInTheDocument();
    expect(screen.queryByText("4,500 pts")).not.toBeInTheDocument();
  });

  it("shows team health rather than individual player health for a team duel", () => {
    const props = createProps();
    render(
      <EndMatchOverlay
        {...props}
        mode="team_duel"
        sides={{
          self: {
            id: "a",
            participant: {
              kind: "team",
              id: "a",
              name: "Team Red",
              avatarFallback: "R",
              avatarColor: "#dc2626",
              members: [],
            },
            hp: 6000,
            connection: "connected",
          },
          opponent: {
            id: "b",
            participant: {
              kind: "team",
              id: "b",
              name: "Team Blue",
              avatarFallback: "B",
              avatarColor: "#2563eb",
              members: [],
            },
            hp: 3200,
            connection: "connected",
          },
        }}
        roundResults={[
          {
            ...props.roundResults[0],
            players: {
              self: { ...props.roundResults[0].players.self, hpAfterRound: 1111 },
              opponent: {
                ...props.roundResults[0].players.opponent,
                hpAfterRound: 2222,
              },
            },
            teams: {
              a: {
                teamId: "a",
                representativeUserId: "self",
                lat: 1,
                lng: 1,
                distanceKm: 10,
                score: 4500,
                hpAfterRound: 6000,
              },
              b: {
                teamId: "b",
                representativeUserId: "opponent",
                lat: 2,
                lng: 2,
                distanceKm: 500,
                score: 1000,
                hpAfterRound: 3200,
              },
            },
          },
        ]}
      />,
    );
    openBreakdown();

    expect(screen.getByText("6,000 HP")).toBeInTheDocument();
    expect(screen.getByText("3,200 HP")).toBeInTheDocument();
    expect(screen.getByText("2.5s")).toHaveClass("text-content-secondary");
    expect(screen.getByText("7.0s")).toHaveClass("text-content-secondary");
    expect(screen.queryByText("1,111 HP")).not.toBeInTheDocument();
    expect(screen.queryByText("2,222 HP")).not.toBeInTheDocument();
  });

  it("shows team points rather than representative player points in points mode", () => {
    const props = createProps();
    render(
      <EndMatchOverlay
        {...props}
        mode="team_duel"
        sides={{
          self: {
            id: "a",
            participant: {
              kind: "team",
              id: "a",
              name: "Team Red",
              avatarFallback: "R",
              avatarColor: "#dc2626",
              members: [],
            },
            hp: 6000,
            connection: "connected",
          },
          opponent: {
            id: "b",
            participant: {
              kind: "team",
              id: "b",
              name: "Team Blue",
              avatarFallback: "B",
              avatarColor: "#2563eb",
              members: [],
            },
            hp: 3200,
            connection: "connected",
          },
        }}
        roundResults={[
          {
            ...props.roundResults[0],
            teams: {
              a: {
                teamId: "a",
                representativeUserId: "self",
                lat: 1,
                lng: 1,
                distanceKm: 10,
                score: 4800,
                hpAfterRound: 6000,
              },
              b: {
                teamId: "b",
                representativeUserId: "opponent",
                lat: 2,
                lng: 2,
                distanceKm: 500,
                score: 900,
                hpAfterRound: 3200,
              },
            },
          },
        ]}
      />,
    );
    openBreakdown();
    fireEvent.click(screen.getByRole("button", { name: "points" }));

    expect(screen.getByText("4,800 pts")).toBeInTheDocument();
    expect(screen.getByText("900 pts")).toBeInTheDocument();
    expect(screen.queryByText("4,500 pts")).not.toBeInTheDocument();
    expect(screen.queryByText("1,000 pts")).not.toBeInTheDocument();
  });

  it("shows a placeholder when an older round result has no health value", () => {
    const props = createProps();
    render(
      <EndMatchOverlay
        {...props}
        roundResults={[
          {
            ...props.roundResults[0],
            players: {
              self: { ...props.roundResults[0].players.self, hpAfterRound: undefined },
              opponent: {
                ...props.roundResults[0].players.opponent,
                hpAfterRound: undefined,
              },
            },
          },
        ]}
      />,
    );
    openBreakdown();

    expect(screen.getAllByText("-")).toHaveLength(2);
  });

  it("keeps score breakdowns for points-based modes", () => {
    render(
      <EndMatchOverlay
        {...createProps({
          mode: "singleplayer",
        })}
      />,
    );
    openBreakdown();

    expect(screen.getAllByText("4,500").length).toBeGreaterThan(0);
    expect(screen.getByText("2.5s")).toHaveClass("text-content-secondary");
    expect(screen.getByText("Total")).toBeInTheDocument();
    expect(screen.queryByText("5,000 HP")).not.toBeInTheDocument();
  });

  it("renders the explicit return destination instead of inferring it from mode", () => {
    render(
      <EndMatchOverlay
        {...createProps({
          mode: "singleplayer",
          backLabel: "Back to party",
        })}
      />,
    );

    expect(
      screen.getByRole("button", { name: "Back to party" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Back To Home")).not.toBeInTheDocument();
  });
});
