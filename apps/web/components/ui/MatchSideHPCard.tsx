import { useMemo } from "react";
import { WifiOff } from "lucide-react";
import {
  ParticipantAvatar,
  ParticipantName,
  type MatchSideView,
} from "./ParticipantIdentity";

type Props = {
  position: "left" | "right";
  side: MatchSideView;
  hpPct: string;
  opponent?: boolean;
};

export default function MatchSideHPCard({
  position,
  side,
  hpPct,
  opponent,
}: Props) {
  const numericPct = parseFloat(hpPct) || 0;

  const isLeft = position === "left";
  const skewClass = isLeft ? "-skew-x-[25deg]" : "skew-x-[25deg]";
  const reverseSkewClass = isLeft ? "skew-x-[25deg]" : "-skew-x-[25deg]";
  const fillGradientAngle = isLeft ? "90deg" : "270deg";
  const showDisconnectBadge =
    opponent && side.connection !== "connected";
  const connectionLabel =
    side.connection === "degraded"
      ? "Opponent team connection degraded"
      : "Opponent disconnected";
  const hp = side.hp ?? 0;

  const hpFill = useMemo(
    () =>
      numericPct > 50
        ? {
          backgroundImage: `linear-gradient(${fillGradientAngle}, #09C967 0%, #52B891 100%)`,
        }
        : numericPct > 25
          ? {
            backgroundImage: "linear-gradient(180deg, #F7D046 0%, #FACC15 100%)",
          }
          : {
            backgroundImage: "linear-gradient(180deg, #F87171 0%, #EF4444 100%)",
          },
    [fillGradientAngle, numericPct],
  );

  return (
    <div
      className={`pointer-events-none absolute top-4 z-40 flex w-[min(380px,calc(50vw-1.25rem))] flex-col md:w-[min(380px,calc(50vw-8.5rem))] ${isLeft ? "left-2 md:left-4" : "right-2 md:right-4"}`}
    >
      <div
        className={`flex items-center ${isLeft ? "flex-row" : "flex-row-reverse"}`}
      >
        {/* Avatar Profile Picture */}
        <div className="relative z-10 w-[54px] h-[54px] shrink-0 drop-shadow-[0_4px_6px_rgba(0,0,0,0.6)]">
          <ParticipantAvatar
            participant={side.participant}
            size="lg"
            opponent={opponent}
            className="h-full w-full border-0 shadow-lg"
          />
        </div>

        {/* HP Bar Container */}
        <div
          className={`relative h-[28px] flex-1 rounded-[4px] bg-[linear-gradient(180deg,#595B69_0%,#3C3E4F_100%)] p-[1px] drop-shadow-[0_4px_4px_rgba(0,0,0,0.6)] ${skewClass}`}
          style={{
            marginLeft: isLeft ? "-12px" : "0",
            marginRight: !isLeft ? "-12px" : "0",
          }}
        >
          {showDisconnectBadge ? (
            <div
              aria-label={connectionLabel}
              data-testid="disconnect-badge"
              title={connectionLabel}
              className={`absolute -top-3 z-20 flex h-7 w-7 items-center justify-center rounded-full border border-red-200/45 bg-red-500 text-white shadow-[0_4px_12px_rgba(0,0,0,0.45)] ${reverseSkewClass} ${isLeft ? "-right-2" : "-left-2"}`}
            >
              <WifiOff aria-hidden="true" size={15} strokeWidth={2.6} />
            </div>
          ) : null}

          {/* Inner dark background */}
          <div className="relative h-full w-full overflow-hidden rounded-[3px]">
            {/* The colored fill */}
            <div
              className={`absolute top-0 bottom-0 ${isLeft ? "left-0" : "right-0"} transition-[width] duration-700 ease-[cubic-bezier(0.16,1,0.3,1)]`}
              style={{ width: hpPct, ...hpFill }}
            />

            {/* HP Text */}
            <div
              className={`font-hud absolute inset-0 flex items-center justify-center text-md text-white drop-shadow-[0_2px_2px_rgba(0,0,0,0.8)] ${reverseSkewClass}`}
            >
              {hp}
            </div>
          </div>
        </div>
      </div>

      {/* Player Name */}
      <div
        className={`-mt-2 flex items-center ${isLeft ? "justify-start pl-[50px]" : "justify-end pr-[50px]"}`}
        data-testid="player-name-row"
      >
        <span className="block max-w-full truncate px-2 text-[15px] font-bold text-white drop-shadow-[0_2px_2px_rgba(0,0,0,0.7)]">
          <ParticipantName
            participant={side.participant}
            nameClassName="font-bold text-white"
          />{" "}
          {side.participant.kind === "player" &&
            side.participant.rating !== undefined && (
            <span className="inline-flex items-center gap-1 text-[#9fd6bf]">
              ({side.participant.rating})
            </span>
          )}
        </span>
      </div>
    </div>
  );
}
