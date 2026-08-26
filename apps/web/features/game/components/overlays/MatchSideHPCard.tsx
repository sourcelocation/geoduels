import { WifiOff } from "lucide-react";
import {
  ParticipantAvatar,
  ParticipantName,
  type MatchSideView,
} from "./ParticipantIdentity";
import { MultiplierBadge } from "../MultiplierBadge";
import { getHealthTone } from "../health-bar";
import styles from "./GameHud.module.css";

type Props = {
  position: "left" | "right";
  side: MatchSideView;
  hpPct: string;
  opponent?: boolean;
  damageMultiplier?: number;
};

export default function MatchSideHPCard({
  position,
  side,
  hpPct,
  opponent,
  damageMultiplier,
}: Props) {
  const numericPct = parseFloat(hpPct) || 0;

  const isLeft = position === "left";
  const showDisconnectBadge =
    opponent && side.connection !== "connected";
  const connectionLabel =
    side.connection === "degraded"
      ? "Opponent team connection degraded"
      : "Opponent disconnected";
  const hp = side.hp ?? 0;

  const healthTone = getHealthTone(numericPct);

  return (
    <div
      className={`pointer-events-none absolute top-4 z-game-controls flex w-[min(380px,calc(50vw-1.25rem))] flex-col md:w-[min(380px,calc(50vw-8.5rem))] ${isLeft ? "left-2 md:left-4" : "right-2 md:right-4"}`}
    >
      <div
        className={`flex items-center ${isLeft ? "flex-row" : "flex-row-reverse"}`}
      >
        {/* Avatar Profile Picture */}
        <div className="relative z-content w-[54px] h-[54px] shrink-0 drop-shadow-md">
          <ParticipantAvatar
            participant={side.participant}
            size="lg"
            opponent={opponent}
            className="h-full w-full border-0 shadow-elev-3"
          />
        </div>

        {/* HP Bar Container */}
        <div
          className={`relative ${styles.healthBar} ${isLeft ? styles.healthBarLeft : styles.healthBarRight}`}
          data-health-tone={healthTone}
        >
          {showDisconnectBadge ? (
            <div
              aria-label={connectionLabel}
              data-testid="disconnect-badge"
              title={connectionLabel}
              className={`absolute -top-3 z-sticky flex h-7 w-7 items-center justify-center rounded-full border border-status-danger/45 bg-status-danger text-content-on-danger shadow-elev-1 ${isLeft ? styles.healthTextLeft : styles.healthTextRight} ${isLeft ? "-right-2" : "-left-2"}`}
            >
              <WifiOff aria-hidden="true" size={15} strokeWidth={2.6} />
            </div>
          ) : null}

          {/* Inner dark background */}
          <div className={styles.healthBarInner}>
            {/* The colored fill */}
            <div
              className={`${styles.healthFill} ${isLeft ? "left-0" : "right-0"}`}
              style={{ width: hpPct }}
            />

            {/* HP Text */}
            <div
              className={`font-hud absolute inset-0 flex items-center justify-center text-heading-sm font-strong leading-collapsed text-content-on-action drop-shadow-sm ${isLeft ? styles.healthTextLeft : styles.healthTextRight}`}
            >
              {hp}
            </div>
          </div>
        </div>

        {damageMultiplier !== undefined ? (
          <div className={`relative z-content shrink-0 ${isLeft ? "ml-2" : "mr-2"}`}>
            <MultiplierBadge
              value={damageMultiplier}
              showBaseline
              mirrored={!isLeft}
              testId={opponent ? "opponent-multiplier" : "self-multiplier"}
            />
          </div>
        ) : null}
      </div>

      {/* Player Name */}
      <div
        className={`-mt-2 flex items-center ${isLeft ? "justify-start pl-[50px]" : "justify-end pr-[50px]"}`}
        data-testid="player-name-row"
      >
        <span className="block max-w-full truncate px-2 text-body-sm font-strong text-content-primary drop-shadow-sm">
          <ParticipantName
            participant={side.participant}
            nameClassName="font-strong text-content-primary"
          />{" "}
          {side.participant.kind === "player" &&
            side.participant.rating !== undefined && (
            <span className="inline-flex items-center gap-1 text-status-success/80">
              ({side.participant.rating})
            </span>
          )}
        </span>
      </div>
    </div>
  );
}
