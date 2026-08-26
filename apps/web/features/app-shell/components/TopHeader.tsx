import AvatarBadge from '../../players/components/AvatarBadge';
import PlayerProfileLink from '../../players/components/PlayerProfileLink';

type Props = {
  userId: string;
  displayName: string;
  userEmail: string;
  userAvatar?: string;
  connected: boolean;
};

export default function TopHeader({ userId, displayName, userEmail, userAvatar, connected }: Props) {
  const fallback = (displayName || userEmail || 'G').slice(0, 1).toUpperCase();
  const profileTitle = userId ? displayName || userEmail : 'Guest';
  // const profileSubtitle = userId ? 'Ready for ranked map duels' : 'Sign in to play ranked';

  return (
    <header className="flex items-start justify-between gap-4">
      <img src="/logo.v2.png" alt="GeoDuels" className="h-11 w-auto md:h-14" />

      <div className="min-w-[220px] rounded-xl border border-status-success/40 bg-status-success/10 p-3 text-content-primary shadow-elev-2 md:min-w-[280px]">
        <div className="mb-2 flex items-center justify-between">
          <span className="text-label font-strong uppercase tracking-eyebrow-wide">{connected ? 'Online' : 'Offline'}</span>
          <span className="text-label font-strong uppercase tracking-control text-content-secondary">Competitive</span>
        </div>
        <PlayerProfileLink userId={userId} nickname={displayName} disabled={!userId} className="flex items-center gap-3">
          <AvatarBadge avatarUrl={userAvatar} fallback={fallback} alt={profileTitle} size="sm" />
          <div>
            <p className="text-body-sm font-semibold leading-heading">{profileTitle}</p>
          </div>
        </PlayerProfileLink>
      </div>
    </header>
  );
}
