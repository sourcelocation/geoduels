import AvatarBadge from './AvatarBadge';
import PlayerProfileLink from './PlayerProfileLink';

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

      <div className="min-w-[220px] rounded-panel border border-emerald-300/60 bg-emerald-300/15 p-3 text-emerald-50 shadow-[0_0_28px_rgba(52,211,153,0.4)] backdrop-blur-sm md:min-w-[280px]">
        <div className="mb-2 flex items-center justify-between">
          <span className="text-[11px] font-bold uppercase tracking-[0.16em]">{connected ? 'Online' : 'Offline'}</span>
          <span className="text-[11px] font-bold uppercase tracking-[0.14em] text-emerald-100/90">Competitive</span>
        </div>
        <PlayerProfileLink userId={userId} nickname={displayName} disabled={!userId} className="flex items-center gap-3">
          <AvatarBadge avatarUrl={userAvatar} fallback={fallback} alt={profileTitle} size="sm" />
          <div>
            <p className="text-sm font-semibold leading-tight">{profileTitle}</p>
            {/* <p className="text-xs text-emerald-100/85">{profileSubtitle}</p> */}
          </div>
        </PlayerProfileLink>
      </div>
    </header>
  );
}
