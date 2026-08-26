import { useEffect, useState } from 'react';

type Props = {
  avatarUrl?: string;
  fallback: string;
  alt: string;
  opponent?: boolean;
  size?: 'sm' | 'md' | 'lg' | 'xl';
  className?: string;
  avatarColor?: string;
};

const sizeClass: Record<NonNullable<Props['size']>, string> = {
  sm: 'h-9 w-9 text-body-sm',
  md: 'h-11 w-11 text-body',
  lg: 'h-14 w-14 text-heading-sm',
  xl: 'h-20 w-20 text-heading-md'
};

export default function AvatarBadge({
  avatarUrl,
  fallback,
  alt,
  opponent = false,
  size = 'md',
  className = '',
  avatarColor
}: Props) {
  const [imgFailed, setImgFailed] = useState(false);

  useEffect(() => {
    setImgFailed(false);
  }, [avatarUrl]);

  const base = avatarColor
    ? ''
    : opponent
      ? 'bg-gradient-to-br from-brand-orange via-brand-orange to-status-danger'
      : 'bg-gradient-to-br from-action-primary via-action-primary to-status-success';

  return (
    <div
      className={`relative grid place-items-center overflow-hidden rounded-full border border-border-strong ${base} ${sizeClass[size]} ${className}`}
      style={avatarColor ? { backgroundColor: avatarColor } : undefined}
    >
      {avatarUrl && !imgFailed ? (
        // Using img keeps this simple for remote avatar URLs.
        <img
          src={avatarUrl}
          alt={alt}
          className="h-full w-full object-cover"
          onError={() => setImgFailed(true)}
        />
      ) : (
        <span className={`font-strong ${avatarColor ? 'text-content-on-action font-hud' : 'text-content-inverse'}`}>{fallback.slice(0, 1).toUpperCase()}</span>
      )}
    </div>
  );
}
