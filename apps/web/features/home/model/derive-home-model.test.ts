import { describe, expect, it } from 'vitest';
import type { Snapshot } from '../../../components/ui/types';
import { createRuntimeConfigFixture } from '../../../test/runtime-config.fixture';
import type { SessionState } from '../../auth/controllers/session-controller';
import type { GameState } from '../../game/controllers/game-controller';
import type { MatchState } from '../../matchmaking/controllers/match-controller';
import { deriveHomeModel } from './derive-home-model';

function createAuthState(overrides: Partial<SessionState> = {}): SessionState {
  return {
    userId: 'self',
    userEmail: 'self@example.com',
    displayName: 'Self',
    userAvatar: 'self.png',
    isGuest: false,
    isAdmin: false,
    mmr: 1200,
    ratingRd: 350,
    gamesPlayed: 10,
    wins: 6,
    rankedGamesPlayed: 3,
    rankedWins: 2,
    leaderboard: null,
    accessToken: 'access-token',
    nicknameRequired: false,
    nicknameInput: 'Self',
    nicknameError: '',
    nicknameSaving: false,
    authLoading: false,
    authError: '',
    googleSignInEnabled: true,
    googleClientId: 'google-client',
    ...overrides
  };
}

function createSnapshot(overrides: Partial<Snapshot> = {}): Snapshot {
  return {
    matchId: 'match-1',
    state: 'active',
    eventSequence: 3,
    phase: 'live',
    roundPhase: 'round_live',
    phaseStartedAt: Date.now(),
    roundMsLeft: 20000,
    phaseEndsAt: Date.now() + 20000,
    currentRound: {
      roundId: 'round-1',
      roundNumber: 3,
      location: { panoId: 'pano-123' }
    },
    players: {
      self: {
        userId: 'self',
        displayName: 'Self',
        avatarUrl: 'self.png',
        hp: 5000,
        mmr: 1200,
        ratingRd: 350,
        finalized: false,
        isGuest: false,
        disconnected: false
      },
      opp: {
        userId: 'opp',
        displayName: 'Opponent',
        avatarUrl: 'opp.png',
        hp: 4500,
        mmr: 1300,
        ratingRd: 350,
        finalized: false,
        isGuest: false,
        disconnected: false
      }
    },
    ratingPreview: {
      self: { win: 80, lose: -80, draw: 10 },
      opp: { win: 80, lose: -80, draw: -10 }
    },
    ...overrides
  };
}

function createMatchState(
  snapshot: Snapshot | null,
  overrides: Partial<MatchState> = {},
): MatchState {
  return {
      matchmaking: {
        status: snapshot ? 'in_match' : 'ready',
        activeRecoverRequestID: null,
        intentVersion: 0,
        queueStartedAt: null
      },
    connected: true,
	    snapshot,
	    activeMatchId: snapshot?.matchId || '',
	    sourcePartyId: '',
    sourcePartyInviteCode: '',
    queueError: '',
    connectionIssue: '',
    onlinePlayers: 42,
    ...overrides
  };
}

function createGameState(overrides: Partial<GameState> = {}): GameState {
  return {
    persistedRoundResultCtx: null,
    guess: { lat: 5, lng: 6 },
    roundMSLeft: 20_000,
    displayRoundSeconds: 20,
    displayHP: { self: 5000, opp: 4500 },
    opponentGuessAlert: false,
    guessSubmitted: false,
    resultPhase: 'base',
    resultShownHP: { self: 5000, opp: 4500 },
    showMatchEndPage: false,
    ...overrides
  };
}

describe('deriveHomeModel', () => {
  const config = createRuntimeConfigFixture();
  const ratingDelta = (participant: { kind: string; ratingDelta?: number }) =>
    participant.kind === 'player' ? participant.ratingDelta : undefined;

  it('derives live round state for lobby and game views', () => {
    const model = deriveHomeModel({
      auth: createAuthState(),
      match: createMatchState(createSnapshot()),
      game: createGameState(),
      config,
      routeMatchId: 'match-1'
    });

    expect(model.lobby.inGame).toBe(true);
    expect(model.game.uiPhase).toBe('live_round');
    expect(model.game.sides.self.participant.name).toBe('Self');
    expect(model.game.sides.opponent.participant.name).toBe('Opponent');
    expect(model.game.sides.opponent.connection).toBe('connected');
    expect(model.game.damageMultiplier).toBe(1.5);
    expect(model.meta.appVersion).toBe('dev');
  });

  it('uses a question mark avatar fallback for users without a linked account', () => {
    const snapshot = createSnapshot({
      players: {
        ...createSnapshot().players,
        self: {
          ...createSnapshot().players.self,
          displayName: 'User 123',
          avatarUrl: '',
          isGuest: false
        }
      }
    });

    const model = deriveHomeModel({
      auth: createAuthState({
        userEmail: '',
        displayName: 'User 123',
        userAvatar: '',
        isGuest: false,
        nicknameInput: 'User 123'
      }),
      match: createMatchState(snapshot),
      game: createGameState(),
      config,
      routeMatchId: 'match-1'
    });

    expect(model.game.sides.self.participant.avatarFallback).toBe('?');
    expect(model.game.resultPlayerFallbacks.self).toBe('?');
  });

  it('shows an idle duel timer before the first finalized guess starts the clock', () => {
    const model = deriveHomeModel({
      auth: createAuthState(),
      match: createMatchState(createSnapshot({
        roundMsLeft: 0,
        phaseEndsAt: 0,
        currentRound: {
          roundId: 'round-1',
          roundNumber: 3,
          timerStarted: false,
          location: { panoId: 'pano-123' }
        }
      })),
      game: createGameState({ roundMSLeft: 0, displayRoundSeconds: 0 }),
      config,
      routeMatchId: 'match-1'
    });

    expect(model.game.isRoundTimerRunning).toBe(false);
    expect(model.game.mm).toBe('--');
    expect(model.game.ss).toBe('--');
    expect(model.game.isTimerCritical).toBe(false);
    expect(model.game.isTimerPulseActive).toBe(false);
    expect(model.game.timerProgressPct).toBe(100);
  });

  it('keeps the HUD clock hidden during the intro-to-live handoff before timerStarted is explicit', () => {
    const model = deriveHomeModel({
      auth: createAuthState(),
      match: createMatchState(createSnapshot({
        roundPhase: 'round_intro',
        roundMsLeft: 0,
        phaseEndsAt: Date.now(),
        currentRound: {
          roundId: 'round-1',
          roundNumber: 1,
          location: { panoId: 'pano-123' }
        }
      })),
      game: createGameState({ roundMSLeft: 0, displayRoundSeconds: 0 }),
      config,
      routeMatchId: 'match-1'
    });

    expect(model.game.uiPhase).toBe('live_round');
    expect(model.game.isRoundTimerRunning).toBe(false);
    expect(model.game.mm).toBe('--');
    expect(model.game.ss).toBe('--');
  });

  it('surfaces opponent disconnect state from the live snapshot', () => {
    const snapshot = createSnapshot({
      players: {
        ...createSnapshot().players,
        opp: {
          ...createSnapshot().players.opp,
          disconnected: true
        }
      }
    });

    const model = deriveHomeModel({
      auth: createAuthState(),
      match: createMatchState(snapshot),
      game: createGameState(),
      config,
      routeMatchId: 'match-1'
    });

    expect(model.game.sides.opponent.connection).toBe('disconnected');
  });

  it('prefers the round pano ID in the Street View URL', () => {
    const model = deriveHomeModel({
      auth: createAuthState(),
      match: createMatchState(createSnapshot({
        currentRound: {
          roundId: 'round-1',
          roundNumber: 3,
          location: {
            panoId: 'pano-123',
            heading: 123.5,
            pitch: -4
          }
        }
      })),
      game: createGameState(),
      config,
      routeMatchId: 'match-1'
    });

    const streetViewURL = new URL(model.game.streetViewSrc);
    expect(streetViewURL.searchParams.get('pano')).toBe('pano-123');
    expect(streetViewURL.searchParams.get('location')).toBeNull();
    expect(streetViewURL.searchParams.get('heading')).toBe('123.5');
    expect(streetViewURL.searchParams.get('pitch')).toBe('-4');
  });

  it('derives round result overlay state', () => {
    const snapshot = createSnapshot({
      phase: 'round_result',
      roundPhase: 'round_result',
      lastRoundResult: {
        roundId: 'round-1',
        roundNumber: 3,
        actualLocation: { lat: 0, lng: 0 },
        players: {
          self: {
            userId: 'self',
            lat: 0,
            lng: 0,
            score: 4200,
            distanceKm: 10,
            damageTaken: 0,
            hpAfterRound: 5000
          },
          opp: {
            userId: 'opp',
            lat: 1,
            lng: 1,
            score: 1200,
            distanceKm: 500,
            damageTaken: 700,
            hpAfterRound: 3800
          }
        }
      }
    });

    const model = deriveHomeModel({
      auth: createAuthState(),
      match: createMatchState(snapshot),
      game: createGameState({
        resultPhase: 'scores',
        resultShownHP: { self: 5000, opp: 4500 }
      }),
      config,
      routeMatchId: 'match-1'
    });

    expect(model.game.showResultStage).toBe(true);
    expect(model.game.resultOverlay?.winner).toBe('self');
    expect(model.game.resultOverlay?.damage).toBe(3000);
  });

  it('derives team duel round result overlay from team representatives', () => {
    const base = createSnapshot();
    const snapshot = createSnapshot({
      mode: 'team_duel',
      phase: 'round_result',
      roundPhase: 'round_result',
      players: {
        self: { ...base.players.self, teamId: 'a', hp: 6000 },
        mate: { ...base.players.self, userId: 'mate', displayName: 'Mate', teamId: 'a', hp: 6000 },
        opp: { ...base.players.opp, teamId: 'b', hp: 3600 },
        opp2: { ...base.players.opp, userId: 'opp2', displayName: 'Opp Two', teamId: 'b', hp: 3600 }
      },
      teams: {
        a: { teamId: 'a', name: 'Team Red', hp: 6000, players: ['self', 'mate'] },
        b: { teamId: 'b', name: 'Team Blue', hp: 3600, players: ['opp', 'opp2'] }
      },
      lastRoundResult: {
        roundId: 'round-1',
        roundNumber: 3,
        actualLocation: { lat: 0, lng: 0 },
        players: {
          self: { userId: 'self', lat: 0, lng: 0, score: 4100, distanceKm: 15, hpAfterRound: 6000 },
          mate: { userId: 'mate', lat: 1, lng: 1, score: 4500, distanceKm: 9, hpAfterRound: 6000 },
          opp: { userId: 'opp', lat: 2, lng: 2, score: 2100, distanceKm: 900, hpAfterRound: 3600 },
          opp2: { userId: 'opp2', lat: 3, lng: 3, score: 1200, distanceKm: 1500, hpAfterRound: 3600 }
        },
        teams: {
          a: { teamId: 'a', representativeUserId: 'mate', lat: 1, lng: 1, score: 4500, distanceKm: 9, hpAfterRound: 6000 },
          b: { teamId: 'b', representativeUserId: 'opp', lat: 2, lng: 2, score: 2100, distanceKm: 900, hpAfterRound: 3600 }
        }
      }
    });

    const model = deriveHomeModel({
      auth: createAuthState(),
      match: createMatchState(snapshot),
      game: createGameState({ resultPhase: 'scores', resultShownHP: { self: 6000, opp: 3600 } }),
      config,
      routeMatchId: 'match-1'
    });

    expect(model.game.sides.self.participant.name).toBe('Team Red');
    expect(model.game.sides.opponent.participant.name).toBe('Team Blue');
    expect(model.game.resultOverlay?.winner).toBe('self');
    expect(model.game.resultOverlay?.damage).toBe(2400);
    expect(model.game.resultOverlay?.sides.self.participant.avatarFallback).toBe('R');
    expect(model.game.resultOverlay?.sides.opponent.participant.avatarFallback).toBe('B');
  });

  it('uses points result UI for free for all instead of the duel overlay', () => {
    const snapshot = createSnapshot({
      mode: 'free_for_all',
      phase: 'round_result',
      roundPhase: 'round_result',
      players: {
        ...createSnapshot().players,
        self: { ...createSnapshot().players.self, totalScore: 4200 }
      },
      lastRoundResult: {
        roundId: 'round-1',
        roundNumber: 1,
        actualLocation: { lat: 0, lng: 0 },
        players: {
          self: { userId: 'self', lat: 0, lng: 0, score: 4200, distanceKm: 10, hpAfterRound: 0 },
          opp: { userId: 'opp', lat: 1, lng: 1, score: 1200, distanceKm: 500, hpAfterRound: 0 }
        }
      }
    });

    const model = deriveHomeModel({
      auth: createAuthState(),
      match: createMatchState(snapshot),
      game: createGameState({ resultPhase: 'hp_apply', resultShownHP: { self: 4200, opp: 0 } }),
      config,
      routeMatchId: 'match-1'
    });

    expect(model.game.showResultStage).toBe(true);
    expect(model.game.isPointsMode).toBe(true);
    expect(model.game.resultOverlay).toBeUndefined();
    expect(model.game.currentRoundScore).toBe(4200);
  });

  it('falls back safely when an opponent is missing', () => {
    const snapshot = createSnapshot({ players: { self: createSnapshot().players.self } });
    const model = deriveHomeModel({
      auth: createAuthState(),
      match: createMatchState(snapshot),
      game: createGameState(),
      config,
      routeMatchId: 'match-1'
    });

    expect(model.game.sides.opponent.participant.name).toBe('Opponent');
    expect(model.game.oppHP).toBe(0);
  });

  it('shows match end overlay with elo deltas for non-guests', () => {
    const snapshot = createSnapshot({
      state: 'ended',
      phase: 'round_result',
      roundPhase: 'round_result',
      lastRoundResult: {
        roundId: 'round-3',
        roundNumber: 3,
        actualLocation: { lat: 3, lng: 4 },
        players: {
          self: {
            userId: 'self',
            lat: 3.2,
            lng: 4.1,
            score: 3900,
            distanceKm: 35,
            hpAfterRound: 5000
          },
          opp: {
            userId: 'opp',
            lat: 11,
            lng: 12,
            score: 600,
            distanceKm: 1400,
            damageTaken: 3300,
            hpAfterRound: 1200
          }
        }
      }
    });
    const model = deriveHomeModel({
      auth: createAuthState(),
      match: createMatchState(snapshot),
      game: createGameState({ showMatchEndPage: true }),
      config,
      routeMatchId: 'match-1'
    });

    expect(model.overlays.endMatch.open).toBe(true);
    if (model.overlays.endMatch.open) {
      expect(model.overlays.endMatch.sides.self.participant.kind).toBe('player');
      expect(ratingDelta(model.overlays.endMatch.sides.self.participant)).not.toBe(0);
      expect(model.overlays.endMatch.roundResults).toHaveLength(1);
    }
  });

  it('hides elo deltas for unranked party matches', () => {
    const snapshot = createSnapshot({
      unranked: true,
      state: 'ended',
      phase: 'round_result',
      roundPhase: 'round_result'
    });
    const model = deriveHomeModel({
      auth: createAuthState(),
      match: createMatchState(snapshot),
      game: createGameState({ showMatchEndPage: true }),
      config,
      routeMatchId: 'match-1'
    });

    expect(
      model.game.sides.self.participant.kind === 'player'
        ? model.game.sides.self.participant.ratingPreview
        : undefined,
    ).toBeUndefined();
    expect(
      model.game.sides.opponent.participant.kind === 'player'
        ? model.game.sides.opponent.participant.ratingPreview
        : undefined,
    ).toBeUndefined();
    expect(model.overlays.endMatch.open).toBe(true);
    if (model.overlays.endMatch.open) {
      expect(model.overlays.endMatch.sides.self.participant.kind).toBe('player');
      expect(model.overlays.endMatch.sides.opponent.participant.kind).toBe('player');
      expect(ratingDelta(model.overlays.endMatch.sides.self.participant)).toBeUndefined();
      expect(ratingDelta(model.overlays.endMatch.sides.opponent.participant)).toBeUndefined();
    }
  });

  it('preserves the completed match config for replay and derives the return label from party provenance', () => {
    const matchConfig = {
      ruleset: 'nmpz' as const,
      mapId: 'map-custom',
      mapName: 'Custom World',
      roundTimerMode: 'fixed' as const,
      roundTimeLimitMs: 60_000,
      pressureTimeLimitMs: 12_000
    };
    const snapshot = createSnapshot({
      mode: 'singleplayer',
      config: matchConfig,
      state: 'ended',
      phase: 'ended',
      roundPhase: 'ended'
    });
    const model = deriveHomeModel({
      auth: createAuthState(),
      match: createMatchState(snapshot, {
        sourcePartyId: 'party-1',
        sourcePartyInviteCode: 'PARTY1'
      }),
      game: createGameState({ showMatchEndPage: true }),
      config,
      routeMatchId: 'match-1'
    });

    expect(model.game.backLabel).toBe('Back to party');
    expect(model.overlays.endMatch.open).toBe(true);
    if (model.overlays.endMatch.open) {
      expect(model.overlays.endMatch.matchConfig).toEqual(matchConfig);
      expect(model.overlays.endMatch.backLabel).toBe('Back to party');
    }
  });

  it('shows only the registered player elo delta when the opponent is a guest', () => {
    const base = createSnapshot();
    const snapshot = createSnapshot({
      state: 'ended',
      phase: 'round_result',
      roundPhase: 'round_result',
      players: {
        ...base.players,
        opp: {
          ...base.players.opp,
          isGuest: true
        }
      }
    });
    const model = deriveHomeModel({
      auth: createAuthState(),
      match: createMatchState(snapshot),
      game: createGameState({ showMatchEndPage: true }),
      config,
      routeMatchId: 'match-1'
    });

    expect(model.overlays.endMatch.open).toBe(true);
    if (model.overlays.endMatch.open) {
      expect(model.overlays.endMatch.sides.self.participant.kind).toBe('player');
      expect(model.overlays.endMatch.sides.opponent.participant.kind).toBe('player');
      expect(ratingDelta(model.overlays.endMatch.sides.self.participant)).not.toBe(0);
      expect(ratingDelta(model.overlays.endMatch.sides.opponent.participant)).toBeUndefined();
    }
  });

  it('shows only the registered opponent elo delta when self is a guest', () => {
    const base = createSnapshot();
    const snapshot = createSnapshot({
      state: 'ended',
      phase: 'round_result',
      roundPhase: 'round_result',
      players: {
        ...base.players,
        self: {
          ...base.players.self,
          isGuest: true
        }
      }
    });
    const model = deriveHomeModel({
      auth: createAuthState({ isGuest: true }),
      match: createMatchState(snapshot),
      game: createGameState({ showMatchEndPage: true }),
      config,
      routeMatchId: 'match-1'
    });

    expect(model.overlays.endMatch.open).toBe(true);
    if (model.overlays.endMatch.open) {
      expect(model.overlays.endMatch.sides.self.participant.kind).toBe('player');
      expect(model.overlays.endMatch.sides.opponent.participant.kind).toBe('player');
      expect(ratingDelta(model.overlays.endMatch.sides.self.participant)).toBeUndefined();
      expect(ratingDelta(model.overlays.endMatch.sides.opponent.participant)).not.toBe(0);
    }
  });

  it('shows a duel end overlay even when a forfeit ends the match before any round result exists', () => {
    const snapshot = createSnapshot({
      state: 'ended',
      phase: 'ended',
      roundPhase: 'ended',
      currentRound: undefined,
      lastRoundResult: undefined,
      roundResults: []
    });
    const model = deriveHomeModel({
      auth: createAuthState(),
      match: createMatchState(snapshot),
      game: createGameState({ showMatchEndPage: true }),
      config,
      routeMatchId: 'match-1'
    });

    expect(model.game.uiPhase).toBe('match_end');
    expect(model.overlays.endMatch.open).toBe(true);
    if (model.overlays.endMatch.open) {
      expect(model.overlays.endMatch.roundResults).toHaveLength(0);
      expect(model.overlays.endMatch.outcome).toBe('win');
    }
  });

  it('keeps the lobby visible when an active snapshot exists off the match route', () => {
    const model = deriveHomeModel({
      auth: createAuthState(),
      match: createMatchState(createSnapshot()),
      game: createGameState(),
      config,
      routeMatchId: null
    });

    expect(model.lobby.inGame).toBe(false);
    expect(model.meta.activeMatchId).toBe('match-1');
  });

  it('uses the same moving timer border for free-for-all rounds', () => {
    const model = deriveHomeModel({
      auth: createAuthState(),
      match: createMatchState(createSnapshot({
        mode: 'free_for_all',
        config: {
          roundTimerMode: 'fixed',
          roundTimeLimitMs: 20_000,
        },
        currentRound: {
          roundId: 'round-1',
          roundNumber: 3,
          timerStarted: true,
          location: { panoId: 'pano-123' },
        },
      })),
      game: createGameState({ roundMSLeft: 10_000, displayRoundSeconds: 10 }),
      config,
      routeMatchId: 'match-1'
    });

    expect(model.game.mode).toBe('free_for_all');
    expect(model.game.isRoundTimerRunning).toBe(true);
    expect(model.game.timerProgressPct).toBe(50);
    expect(model.game.isTimerCritical).toBe(true);
  });
});
