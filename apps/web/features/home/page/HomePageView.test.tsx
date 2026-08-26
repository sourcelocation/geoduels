import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { HomeModel } from "../model/types";
import HomePageView from "./HomePageView";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

vi.mock("next/dynamic", () => ({
  default: () => () => null,
}));

function createModel(overrides?: Partial<HomeModel["view"]>): HomeModel {
  return {
    view: {
      auth: {
        userId: "self",
        accessToken: "access-token",
        userEmail: "self@example.com",
        displayName: "Self",
        userAvatar: "",
        nicknameRequired: true,
        isAdmin: false,
        isGuest: false,
        nicknameInput: "Self",
        nicknameError: "",
        nicknameSaving: false,
        authLoading: false,
        authError: "",
      },
      lobby: {
        inGame: false,
        connected: true,
        mmr: 1200,
        gamesPlayed: 10,
        winsPct: 60,
        leaderboard: null,
        leaderboardLoading: false,
        status: "ready",
        queueStartedAt: null,
        queueError: "",
        singleplayerError: "",
        onlinePlayers: 42,
        canStartSingleplayer: true,
        maintenance: null,
        changelogEyebrow: "News",
        changelogTitle: "Latest",
        changelogMarkdown: "",
        changelogSlug: "",
        changelogUpdatedAt: "",
        party: {
          status: "idle",
          snapshot: null,
          inviteCode: "",
          isOwner: false,
          busy: false,
          error: "",
        },
      },
      game: {
        inGame: true,
        mode: "duel",
        isSingleplayer: false,
        isPointsMode: false,
        uiPhase: "match_end",
        showResultStage: false,
        showMatchEndPage: true,
        streetViewSrc: "",
        ruleset: "moving",
        streetNames: "shown",
        roundResult: undefined,
        roundResults: [],
        resultOverlay: undefined,
        resultPlayerNames: { self: "Self", opp: "Opponent" },
        resultPlayerAvatars: {},
        resultPlayerFallbacks: {},
        resultPlayerBorderColors: {},
        teammateGuesses: {},
        teamPings: [],
        participantsById: {
          self: { kind: "player", id: "self", name: "Self", avatarFallback: "S" },
          opp: { kind: "player", id: "opp", name: "Opponent", avatarFallback: "O" },
        },
        sides: {
          self: {
            id: "self",
            participant: { kind: "player", id: "self", name: "Self", avatarFallback: "S", rating: 1200 },
            hp: 5000,
            connection: "connected",
          },
          opponent: {
            id: "opp",
            participant: { kind: "player", id: "opp", name: "Opponent", avatarFallback: "O", rating: 1100 },
            hp: 0,
            connection: "connected",
          },
        },
        mm: "00",
        ss: "00",
        isRoundTimerRunning: false,
        timerProgressPct: 0,
        isTimerCritical: false,
        isTimerPulseActive: false,
        resultMode: true,
        selfHP: 5000,
        oppHP: 0,
        totalScore: 0,
        currentRoundScore: 0,
        currentRoundDistanceKm: 0,
        canFinalizeGuess: false,
        canAdvanceRound: false,
        guess: undefined,
        currentRoundId: "round-1",
        currentRoundNumber: 1,
        userAvatar: "",
        damageMultiplier: 1,
        guessSubmitted: false,
        opponentGuessAlert: false,
        connectionIssue: "",
        modeName: "Moving",
        mapName: "A Source World",
        streetViewInteractive: true,
        selfUserId: "self",
      },
      chat: {
        conversationId: "",
        messages: [],
        selfUserId: "self",
        error: "",
      },
      overlays: {
        nicknameRequiredOpen: true,
        notifications: [],
        guestVerification: {
          open: false,
          siteKey: "",
          status: "checking",
          error: "",
          resetKey: 0,
        },
        endMatch: {
          open: true,
          mode: "duel",
          outcome: "win",
          selfUserId: "self",
          sides: {
            self: {
              id: "self",
              participant: { kind: "player", id: "self", name: "Self", avatarFallback: "S", rating: 1200, ratingDelta: 15 },
              hp: 5000,
              connection: "connected",
            },
            opponent: {
              id: "opp",
              participant: { kind: "player", id: "opp", name: "Opponent", avatarFallback: "O", rating: 1100, ratingDelta: -15 },
              hp: 0,
              connection: "connected",
            },
          },
          totalScore: 0,
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
                  score: 4000,
                  distanceKm: 20,
                },
                opp: {
                  userId: "opp",
                  lat: 5,
                  lng: 5,
                  score: 1000,
                  distanceKm: 500,
                },
              },
            },
          ],
          resultPlayerNames: { self: "Self", opp: "Opponent" },
          resultPlayerAvatars: { self: "", opp: "" },
          resultPlayerFallbacks: { self: "S", opp: "O" },
          resultPlayerBorderColors: {},
          participantsById: {
            self: { kind: "player", id: "self", name: "Self", avatarFallback: "S" },
            opp: { kind: "player", id: "opp", name: "Opponent", avatarFallback: "O" },
          },
        },
      },
      meta: {
        activeMatchId: "match-1",
        sourcePartyInviteCode: "",
        appVersion: "dev",
        maxHP: 6000,
      },
      ...overrides,
    },
    actions: {
      joinQueue: vi.fn(),
      startSingleplayer: vi.fn(),
      clearSingleplayerError: vi.fn(),
      startSupportDonation: vi.fn(),
      cancelQueue: vi.fn(),
      placeGuess: vi.fn(),
      finalizeGuess: vi.fn(),
      advanceRound: vi.fn(() => true),
      forfeitMatch: vi.fn(() => true),
      leaveGame: vi.fn(),
      pingTeam: vi.fn(),
      sendChatMessage: vi.fn(() => true),
      sendChatEmote: vi.fn(() => true),
      reportPlayer: vi.fn(async () => {}),
      createParty: vi.fn(async () => true),
      joinParty: vi.fn(async () => true),
      leaveParty: vi.fn(async () => {}),
      kickPartyMember: vi.fn(async () => {}),
      transferPartyOwner: vi.fn(async () => {}),
      startParty: vi.fn(async () => {}),
      updatePartySettings: vi.fn(async () => {}),
      switchPartyTeam: vi.fn(async () => {}),
      loadLeaderboard: vi.fn(),
      clearAuthSession: vi.fn(),
      deleteAccount: vi.fn(async () => {}),
      submitRequiredNickname: vi.fn(async () => {}),
      submitProfileNickname: vi.fn(async () => true),
      selectBadge: vi.fn(async () => {}),
      setNicknameInput: vi.fn(),
      dismissNotification: vi.fn(async () => {}),
      submitGuestVerificationToken: vi.fn(),
      markGuestVerificationExpired: vi.fn(),
      cancelGuestVerification: vi.fn(),
    },
  };
}

describe("HomePageView", () => {
  it("renders required nickname and end match overlays while hiding the game scene", () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><HomePageView model={createModel()} /></QueryClientProvider>);

    expect(screen.getByText("Choose Your Nickname")).toBeInTheDocument();
    expect(screen.getByText("Match Complete")).toBeInTheDocument();
    expect(screen.queryByTitle("Street View")).not.toBeInTheDocument();
  });

  it("keeps chat on the top app layer over the match end page", () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}><HomePageView
        model={createModel({
          chat: {
            conversationId: "match:match-1",
            messages: [],
            selfUserId: "self",
            error: "",
          },
        })}
      /></QueryClientProvider>,
    );

    expect(screen.getByLabelText("Open chat").parentElement).toHaveClass(
      "app-layer-chat",
    );
  });
});
