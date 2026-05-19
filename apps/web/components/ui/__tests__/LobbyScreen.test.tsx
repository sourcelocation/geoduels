import React from 'react';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import LobbyScreen from '../LobbyScreen';

function renderLobbyScreen(overrides?: Partial<React.ComponentProps<typeof LobbyScreen>>) {
  const props: React.ComponentProps<typeof LobbyScreen> = {
    userId: 'self',
    userEmail: 'self@example.com',
    displayName: 'Self',
    userAvatar: '',
    isGuest: false,
    connected: true,
    mmr: 1200,
    gamesPlayed: 10,
    winsPct: 60,
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
    linkedProviders: [],
    changelogEyebrow: 'News',
    changelogTitle: 'Latest',
    changelogMarkdown: '',
    devLogin: vi.fn(),
    onGoogleSignIn: vi.fn(),
    onBrowseLeaderboard: vi.fn(),
    authLoading: false,
    authError: '',
    nicknameInput: 'Self',
    nicknameError: '',
    nicknameSaving: false,
    onChangeNickname: vi.fn(),
    onSaveNickname: vi.fn(async () => true),
    onLogout: vi.fn(),
    ...overrides
  };

  return {
    ...render(<LobbyScreen {...props} />),
    props
  };
}

afterEach(() => {
  cleanup();
});

describe('LobbyScreen', () => {
  it('loads the leaderboard only after the leaderboard tab is opened', () => {
    const onBrowseLeaderboard = vi.fn();
    renderLobbyScreen({ onBrowseLeaderboard });

    expect(onBrowseLeaderboard).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'TOP' }));

    expect(onBrowseLeaderboard).toHaveBeenCalledTimes(1);
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

  it('allows all duel modes to be unselected and disables play', () => {
    renderLobbyScreen();

    const playButton = screen.getAllByRole('button', { name: 'Play' })[0];
    expect(playButton).not.toBeDisabled();

    const duelMovingButton = screen.getAllByRole('button', { name: 'Moving' })[0];
    fireEvent.click(duelMovingButton);

    expect(duelMovingButton).toHaveAttribute('aria-pressed', 'false');
    expect(playButton).toBeDisabled();
  });

  it('starts the selected singleplayer mode', () => {
    const startSingleplayer = vi.fn();
    renderLobbyScreen({ startSingleplayer });

    const singleplayerMovingButton = screen.getAllByRole('button', { name: 'Moving' })[1];
    const noMoveButton = screen.getByRole('button', { name: 'No Move' });
    const nmpzButton = screen.getAllByRole('button', { name: 'NMPZ' })[1];

    expect(singleplayerMovingButton).toHaveAttribute('aria-pressed', 'true');
    fireEvent.click(noMoveButton);

    expect(singleplayerMovingButton).toHaveAttribute('aria-pressed', 'false');
    expect(noMoveButton).toHaveAttribute('aria-pressed', 'true');
    expect(nmpzButton).toHaveAttribute('aria-pressed', 'false');

    fireEvent.click(screen.getAllByRole('button', { name: 'Play' })[1]);

    expect(startSingleplayer).toHaveBeenCalledWith('no_move');
  });

  it('shows singleplayer as loading while a start is connecting', () => {
    renderLobbyScreen({ status: 'matched_connecting' });

    const loadingButton = screen.getByRole('button', { name: 'Loading...' });

    expect(loadingButton).toBeDisabled();
    expect(loadingButton.querySelector('.animate-spin')).toBeInTheDocument();
  });

  it('replaces the tabbed lobby content when an invite lobby is active', () => {
    renderLobbyScreen({
      privateLobby: {
        status: 'ready',
        snapshot: {
          id: 'lobby-1',
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

    expect(screen.getByRole('heading', { level: 2, name: 'Private Lobby' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'FRIENDS' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'PLAY' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'TOP' })).not.toBeInTheDocument();
    expect(screen.queryByText('Tutorial')).not.toBeInTheDocument();
  });

  it('keeps lobby route intent on a lobby loading surface before the snapshot arrives', () => {
    renderLobbyScreen({
      privateLobby: {
        status: 'connecting',
        snapshot: null,
        inviteCode: 'ABCD12',
        isMember: false,
        isOwner: false,
        busy: true,
        error: ''
      }
    });

    expect(screen.getByRole('heading', { level: 2, name: 'Private Lobby' })).toBeInTheDocument();
    expect(screen.getByText('Connecting to lobby')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'FRIENDS' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'PLAY' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'TOP' })).not.toBeInTheDocument();
  });

  it('disables private lobby start and marks players outside the lobby', () => {
    renderLobbyScreen({
      privateLobby: {
        status: 'ready',
        snapshot: {
          id: 'lobby-1',
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

  it('opens invite lobby choices and joins with a typed code', () => {
    const joinInviteLobby = vi.fn(async () => true);
    renderLobbyScreen({ joinInviteLobby });

    expect(screen.queryByRole('button', { name: /Private Lobby/i })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'FRIENDS' }));

    expect(screen.getByText('CUSTOM')).toBeInTheDocument();
    expect(screen.getByText('Create a lobby or join your friend')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Private Lobby/i }));

    expect(screen.getByRole('dialog', { name: 'Private Lobby' })).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Join With Code'), {
      target: { value: 'abcd12' }
    });
    fireEvent.click(screen.getByRole('button', { name: 'Join' }));

    expect(joinInviteLobby).toHaveBeenCalledWith('ABCD12');
  });

  it('shows private lobby lookup errors outside the active lobby panel', () => {
    renderLobbyScreen({
      privateLobby: {
        status: 'idle',
        snapshot: null,
        inviteCode: 'BAD123',
        isMember: false,
        isOwner: false,
        busy: false,
        error: 'Lobby not found'
      }
    });

    expect(screen.getByRole('alert')).toHaveTextContent('Lobby not found');
    expect(screen.queryByRole('heading', { level: 2, name: 'Private Lobby' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Create Lobby' })).not.toBeInTheDocument();
  });
});
