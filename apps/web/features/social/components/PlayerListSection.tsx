import type { ReactNode } from "react";
import { AsyncState, InsetList, SectionHeader } from "../../../components/ui/patterns";
import type { CompactPlayer } from "../types";
import { CompactPlayerRow } from "./CompactPlayerRow";

export function PlayerListSection({
  title,
  players,
  loading,
  empty,
  renderActions,
}: {
  title: string;
  players: CompactPlayer[];
  loading?: boolean;
  empty?: string;
  renderActions?: (player: CompactPlayer) => ReactNode;
}) {
  return (
    <section className="mt-5">
      <SectionHeader title={title} className="mb-3" />
      <InsetList>
        {players.map((player) => (
          <CompactPlayerRow
            key={player.userId}
            player={player}
            actions={renderActions?.(player)}
          />
        ))}
        {!players.length && !loading && empty ? <AsyncState status="empty" message={empty} /> : null}
        {loading ? <AsyncState status="loading" message="Loading players" /> : null}
      </InsetList>
    </section>
  );
}
