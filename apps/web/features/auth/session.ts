export type AuthSessionSnapshot = {
  userId: string;
  accessToken: string;
  nicknameRequired: boolean;
  authMigrationRequired?: boolean;
  recoveryAvailable?: boolean;
  linkedProviders?: string[];
  canPlay?: boolean;
  nicknameInput: string;
  expiresAt?: number;
};

export function hasPlayableSession(session: AuthSessionSnapshot | null): boolean {
  return !!session && !!session.userId && !!session.accessToken && !session.nicknameRequired && !session.authMigrationRequired && session.canPlay !== false;
}

export function emptyAuthSession(): AuthSessionSnapshot {
  return {
    userId: '',
    accessToken: '',
    nicknameRequired: false,
    authMigrationRequired: false,
    recoveryAvailable: false,
    linkedProviders: [],
    canPlay: false,
    nicknameInput: '',
    expiresAt: 0
  };
}
