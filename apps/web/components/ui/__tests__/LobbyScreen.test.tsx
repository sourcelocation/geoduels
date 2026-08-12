import React from 'react';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import LobbyScreen from '../LobbyScreen';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

function resetStoredQueueRulesets() {
  if (
    typeof window === 'undefined' ||
    typeof window.localStorage?.removeItem !== 'function'
  ) {
    return;
  }
  window.localStorage.removeItem('geoduels.queueRulesets');
  window.localStorage.removeItem('geoduels.play.duels');
  window.localStorage.removeItem('geoduels.play.singleplayer');
}

function reportExtensionAvailable(extensionVersion = '0.1.3') {
  window.dispatchEvent(
    new MessageEvent('message', {
      source: window,
      origin: window.location.origin,
      data: {
        source: 'geoduels-extension',
        version: 1,
        extensionVersion,
        type: 'extension_ready',
      },
    }),
  );
}

function renderLobbyScreen(overrides?: Partial<React.ComponentProps<typeof LobbyScreen>>) {
  const props: React.ComponentProps<typeof LobbyScreen> = {
    userId: 'self',
    userEmail: 'self@example.com',
    displayName: 'Self',
    userAvatar: '',
    isGuest: false,
    connected: true,
    mmr: 1200,
    leaderboard: null,
    leaderboardLoading: false,
    status: 'ready',
    queueStartedAt: null,
    joinQueue: vi.fn(),
    startSingleplayer: vi.fn(),
    cancelQueue: vi.fn(),
    queueError: '',
    onlinePlayers: 42,
    maintenance: null,
    googleClientId: '',
    appVersion: 'dev',
    isAdmin: false,
    changelogEyebrow: 'News',
    changelogTitle: 'Latest',
    changelogMarkdown: '',
    changelogSlug: '',
    changelogUpdatedAt: '',
    devLogin: vi.fn(),
    onGoogleSignIn: vi.fn(),
    onBrowseLeaderboard: vi.fn(),
    authLoading: false,
    authError: '',
    nicknameSaving: false,
    ...overrides
  };

  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return {
    ...render(<QueryClientProvider client={queryClient}><LobbyScreen {...props} /></QueryClientProvider>),
    props
  };
}

beforeEach(() => {
  const values = new Map<string, string>();
  Object.defineProperty(window, 'localStorage', {
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

describe('LobbyScreen', () => {
  it('loads the leaderboard only on the leaderboard route', () => {
    const onBrowseLeaderboard = vi.fn();
    renderLobbyScreen({ onBrowseLeaderboard });

    expect(onBrowseLeaderboard).not.toHaveBeenCalled();

    cleanup();
    renderLobbyScreen({ contentRoute: 'top', onBrowseLeaderboard });

    expect(onBrowseLeaderboard).toHaveBeenCalledTimes(1);
  });

  it('does not request maps while rendering the play route', async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);

    renderLobbyScreen({ contentRoute: 'play' });
    await new Promise((resolve) => window.setTimeout(resolve, 0));

    const requestedURLs = fetchMock.mock.calls.map((call) => String(call[0]));
    expect(requestedURLs.some((url) => url.includes('/v1/maps'))).toBe(false);
  });

  it('shows the maintenance warning banner and pauses duel queueing', () => {
    renderLobbyScreen({
      maintenance: {
        phase: 'warning',
        startsAt: new Date(Date.now() + 5 * 60_000).toISOString(),
        endsAt: '',
        queuePaused: true,
        playPaused: false,
        message: 'Deploy window opens shortly.'
      }
    });

    expect(screen.getByText(/Maintenance/i)).toBeInTheDocument();
    expect(screen.getByText('Deploy window opens shortly.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Paused' })).toBeDisabled();
  });

  it('opens the duel chooser and requires at least one mode', () => {
    renderLobbyScreen();

    fireEvent.click(screen.getAllByRole('button', { name: 'Play' })[0]);

    expect(screen.getByRole('dialog', { name: 'Find a Duel' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Moving' }));

    expect(screen.getByRole('button', { name: 'Moving' })).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByRole('button', { name: 'Start' })).toBeDisabled();
  });

  it('opens centered sign-in instead of queueing when signed out players press ranked play', () => {
    const joinQueue = vi.fn();
    renderLobbyScreen({
      userId: '',
      userEmail: '',
      displayName: '',
      googleClientId: 'google-client',
      joinQueue
    });

    fireEvent.click(screen.getAllByRole('button', { name: 'Play' })[0]);

    const dialog = screen.getByRole('dialog', { name: 'Sign In' });
    expect(dialog).toBeInTheDocument();
    expect(dialog.parentElement).toHaveClass('items-center');
    expect(dialog.parentElement).not.toHaveClass('items-end');
    expect(joinQueue).not.toHaveBeenCalled();
  });

  it('opens sign-in instead of queueing when guest players press ranked play', () => {
    const joinQueue = vi.fn();
    renderLobbyScreen({
      isGuest: true,
      googleClientId: 'google-client',
      joinQueue
    });

    fireEvent.click(screen.getAllByRole('button', { name: 'Play' })[0]);

    expect(screen.getByRole('dialog', { name: 'Sign In' })).toBeInTheDocument();
    expect(joinQueue).not.toHaveBeenCalled();
  });

  it('locks extension-only modes and visibility without the extension', () => {
    renderLobbyScreen();

    fireEvent.click(screen.getAllByRole('button', { name: 'Play' })[0]);

    expect(screen.getByRole('button', { name: 'No Move' })).toBeDisabled();
    expect(screen.queryByRole('button', { name: 'Hidden' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Any' })).not.toBeInTheDocument();
    expect(
      screen.getByRole('button', {
        name: /hide street names requires the GeoDuels extension/i,
      }),
    ).toHaveAttribute('aria-disabled', 'true');
    expect(screen.getByRole('link', { name: /Chrome setup/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Firefox setup/i })).toBeInTheDocument();
  });

  it('requires users with an outdated extension to update before extension-only modes', () => {
    renderLobbyScreen();
    reportExtensionAvailable('0.1.2');

    fireEvent.click(screen.getAllByRole('button', { name: 'Play' })[0]);

    expect(screen.getByRole('button', { name: 'No Move' })).toBeDisabled();
    expect(screen.getByText(/update the official GeoDuels browser extension/i)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Chrome update/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Firefox update/i })).toBeInTheDocument();
  });

  it('migrates legacy duel mode selections and defaults street names to shown', () => {
    window.localStorage.setItem(
      'geoduels.queueRulesets',
      JSON.stringify(['moving', 'nmpz']),
    );
    renderLobbyScreen();
    reportExtensionAvailable();

    fireEvent.click(screen.getAllByRole('button', { name: 'Play' })[0]);

    expect(screen.getByRole('button', { name: 'Moving' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByRole('button', { name: 'NMPZ' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByRole('button', { name: 'Shown' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
  });

  it('queues registered players for every selected mode and visibility', async () => {
    const joinQueue = vi.fn();
    renderLobbyScreen({ joinQueue });
    reportExtensionAvailable();

    fireEvent.click(screen.getAllByRole('button', { name: 'Play' })[0]);
    expect(await screen.findByRole('dialog', { name: 'Find a Duel' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'No Move' }));
    fireEvent.click(screen.getByRole('button', { name: 'NMPZ' }));
    fireEvent.click(screen.getByRole('button', { name: 'Any' }));
    fireEvent.click(screen.getByRole('button', { name: 'Start' }));

    expect(joinQueue).toHaveBeenCalledWith([
      'moving_hidden',
      'moving',
      'no_move_hidden',
      'no_move',
      'nmpz_hidden',
      'nmpz',
    ]);
  });

  it('uses radio modes and no Any visibility for singleplayer', async () => {
    const startSingleplayer = vi.fn();
    renderLobbyScreen({ startSingleplayer });
    reportExtensionAvailable();

    fireEvent.click(screen.getAllByRole('button', { name: 'Play' })[1]);
    expect(
      await screen.findByRole('dialog', { name: 'Start Singleplayer' }),
    ).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Any' })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'No Move' }));
    expect(screen.getByRole('button', { name: 'Moving' })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
    fireEvent.click(screen.getByRole('button', { name: 'Hidden' }));
    fireEvent.click(screen.getByRole('button', { name: 'Start' }));

    expect(startSingleplayer).toHaveBeenCalledWith({
      ruleset: 'no_move',
      streetNames: 'hidden',
    });
  });

  it('shows singleplayer as loading while a start is connecting', () => {
    renderLobbyScreen({ status: 'matched_connecting' });

    const loadingButton = screen.getByRole('button', { name: 'Loading...' });

    expect(loadingButton).toBeDisabled();
    expect(loadingButton.querySelector('.animate-spin')).toBeInTheDocument();
  });

  it('replaces the tabbed lobby content when an invite lobby is active', () => {
    renderLobbyScreen({
      party: {
        status: 'ready',
        snapshot: {
          id: 'party-1',
          inviteCode: 'ABCD12',
          ownerUserId: 'self',
          state: 'open',
          mode: 'duel',
          mapScope: 'world',
          members: [
            {
              userId: 'self',
              displayName: 'Self',
              role: 'owner',
              connected: true
            }
          ]
        },
        inviteCode: 'ABCD12',
        isMember: true,
        isOwner: true,
        busy: false,
        error: ''
      }
    });

    expect(screen.getByRole('heading', { level: 2, name: 'Private Party' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'FRIENDS' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'PLAY' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'TOP' })).not.toBeInTheDocument();
    expect(screen.queryByText('Tutorial')).not.toBeInTheDocument();
  });

  it('keeps lobby route intent on a lobby loading surface before the snapshot arrives', () => {
    renderLobbyScreen({
      party: {
        status: 'connecting',
        snapshot: null,
        inviteCode: 'ABCD12',
        isMember: false,
        isOwner: false,
        busy: true,
        error: ''
      }
    });

    expect(screen.getByRole('heading', { level: 2, name: 'Private Party' })).toBeInTheDocument();
    expect(screen.getByText('Connecting to party')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'FRIENDS' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'PLAY' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'TOP' })).not.toBeInTheDocument();
  });

  it('disables party start and marks players outside the party', () => {
    renderLobbyScreen({
      party: {
        status: 'ready',
        snapshot: {
          id: 'party-1',
          inviteCode: 'ABCD12',
          ownerUserId: 'self',
          state: 'open',
          mode: 'duel',
          mapScope: 'world',
          members: [
            {
              userId: 'self',
              displayName: 'Self',
              role: 'owner',
              connected: true
            },
            {
              userId: 'opponent',
              displayName: 'Opponent',
              role: 'member',
              connected: false
            }
          ]
        },
        inviteCode: 'ABCD12',
        isMember: true,
        isOwner: true,
        busy: false,
        error: ''
      }
    });

    expect(screen.getByText('You · Online')).toBeInTheDocument();
    expect(screen.getByText('Offline')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Start Duel' })).toBeDisabled();
  });

  it('offers reconnect only to players in the active match roster', () => {
    renderLobbyScreen({
      party: {
        status: 'ready',
        snapshot: {
          id: 'party-1',
          inviteCode: 'ABCD12',
          ownerUserId: 'opponent',
          state: 'in_match',
          mode: 'duel',
          mapScope: 'world',
          activeMatchId: 'match-1',
          members: [
            { userId: 'self', displayName: 'Self', role: 'member', inActiveMatch: true },
            { userId: 'opponent', displayName: 'Opponent', role: 'owner', inActiveMatch: true }
          ]
        },
        inviteCode: 'ABCD12',
        isMember: true,
        isOwner: false,
        busy: false,
        error: ''
      }
    });

    expect(screen.getByText('You are part of this game and can reconnect whenever you are ready.')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Reconnect to Game' })).toHaveAttribute('href', '/match/match-1');
  });

  it('keeps late lobby members out of the active game', () => {
    renderLobbyScreen({
      party: {
        status: 'ready',
        snapshot: {
          id: 'party-1',
          inviteCode: 'ABCD12',
          ownerUserId: 'opponent',
          state: 'in_match',
          mode: 'duel',
          mapScope: 'world',
          activeMatchId: 'match-1',
          members: [
            { userId: 'self', displayName: 'Self', role: 'member' },
            { userId: 'opponent', displayName: 'Opponent', role: 'owner', inActiveMatch: true }
          ]
        },
        inviteCode: 'ABCD12',
        isMember: true,
        isOwner: false,
        busy: false,
        error: ''
      }
    });

    expect(screen.getByText('You joined after this game started and will be able to play in the next one.')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Reconnect to Game' })).not.toBeInTheDocument();
  });

  it('opens invite lobby choices and joins with a typed code', () => {
    const joinParty = vi.fn(async () => true);
    renderLobbyScreen({ joinParty });

    expect(screen.queryByRole('button', { name: /Create Party & Invite/i })).not.toBeInTheDocument();

    cleanup();
    renderLobbyScreen({ contentRoute: 'friends', joinParty });

    expect(screen.getByText('Play GeoDuels with Friends')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Create Party & Invite/i })).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText(/ENTER CODE/i), {
      target: { value: 'abcd12' }
    });
    fireEvent.click(screen.getByRole('button', { name: 'Join Party' }));

    expect(joinParty).toHaveBeenCalledWith('ABCD12');
  });

  it('shows party lookup errors outside the active party panel', () => {
    renderLobbyScreen({
      party: {
        status: 'idle',
        snapshot: null,
        inviteCode: 'BAD123',
        isMember: false,
        isOwner: false,
        busy: false,
        error: 'Party not found'
      }
    });

    expect(screen.getByRole('alert')).toHaveTextContent('Party not found');
    expect(screen.queryByRole('heading', { level: 2, name: 'Private Party' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Create Party' })).not.toBeInTheDocument();
  });
});
