import { MoreHorizontal } from "lucide-react";
import AvatarBadge from "../../players/components/AvatarBadge";
import { DropdownMenu } from "../../../components/ui/DropdownMenu";
import { IconButton } from "../../../components/ui/button";
import { EntityRow } from "../../../components/ui/patterns";
import PlayerNameWithBadge from "../../players/components/PlayerNameWithBadge";
import { RelativeTime } from "../../../components/ui/RelativeTime";
import type { CompactPlayer } from "../types";

export function PlayerPresence({ player }: { player: CompactPlayer }) {
  if (player.presenceStatus === "online") {
    return <span className="text-body-sm font-semibold text-status-success">{player.activity === "in_match" ? "In a duel" : "Online"}</span>;
  }
  if (player.presenceStatus === "away") {
    return <span className="text-body-sm font-semibold text-status-warning">Away</span>;
  }
  if (player.lastSeenAt) {
    return <span className="text-body-sm text-content-secondary">Last seen <RelativeTime value={player.lastSeenAt} /></span>;
  }
  return <span className="text-body-sm text-content-secondary">Offline</span>;
}

export function CompactPlayerRow({
  player,
  meta,
  actions,
  overflow,
}: {
  player: CompactPlayer;
  meta?: React.ReactNode;
  actions?: React.ReactNode;
  overflow?: () => void;
}) {
  return (
    <EntityRow
      leading={<AvatarBadge
        avatarUrl={player.avatarUrl}
        fallback={(player.displayName || "?").slice(0, 1).toUpperCase()}
        alt=""
        size="sm"
      />}
      title={<PlayerNameWithBadge
          userId={player.userId}
          name={player.displayName}
          selectedBadge={player.selectedBadge}
          nameClassName="truncate text-body-sm font-strong text-content-primary"
        />}
      description={meta || <PlayerPresence player={player} />}
      actions={<>{actions}{overflow ? <DropdownMenu trigger={<IconButton aria-label={`More actions for ${player.displayName}`}><MoreHorizontal size={17} /></IconButton>} items={[{ label: "More actions", onSelect: overflow }]} /> : null}</>}
    />
  );
}

/** Preferred domain name; CompactPlayerRow remains a source-compatible alias. */
export const PlayerRow = CompactPlayerRow;
