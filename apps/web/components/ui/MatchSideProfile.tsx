import {
  ParticipantIdentityCard,
  type MatchSideView,
} from "./ParticipantIdentity";

type Props = {
  side: MatchSideView;
  opponent?: boolean;
};

export default function MatchSideProfile({
  side,
  opponent = false,
}: Props) {
  return (
    <ParticipantIdentityCard
      participant={side.participant}
      opponent={opponent}
      avatarClassName="h-32 w-32 border-[6px] shadow-[0_8px_32px_rgba(0,0,0,0.4)] md:h-40 md:w-40"
      nameClassName="text-xl font-black leading-tight text-white drop-shadow-md md:text-2xl"
      className="gap-4"
    />
  );
}
