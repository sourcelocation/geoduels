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
      avatarClassName="h-32 w-32 border-[6px] border-border-strong shadow-elev-2 md:h-40 md:w-40"
      nameClassName="font-display text-heading-md font-strong leading-heading text-content-primary drop-shadow-md"
      className="gap-4"
    />
  );
}
