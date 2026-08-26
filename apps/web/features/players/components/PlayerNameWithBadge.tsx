import PlayerBadge, { type PlayerBadgeInfo } from "./PlayerBadge";
import PlayerProfileLink from "./PlayerProfileLink";

type Props = {
  name: string;
  isAdmin?: boolean;
  selectedBadge?: PlayerBadgeInfo | null;
  userId?: string;
  profileDisabled?: boolean;
  nameClassName?: string;
  wrapperClassName?: string;
};

export default function PlayerNameWithBadge({
  name,
  selectedBadge,
  userId,
  profileDisabled = false,
  nameClassName = "",
  wrapperClassName = "",
}: Props) {
  const content = (
    <span
      className={`inline-flex max-w-full items-center gap-1.5 ${wrapperClassName}`.trim()}
    >
      <span className={`truncate ${nameClassName}`.trim()}>{name}</span>
      <PlayerBadge badge={selectedBadge} size="sm" />
    </span>
  );
  return (
    <PlayerProfileLink userId={userId} nickname={name} disabled={profileDisabled} className="min-w-0 hover:opacity-90">
      {content}
    </PlayerProfileLink>
  );
}
