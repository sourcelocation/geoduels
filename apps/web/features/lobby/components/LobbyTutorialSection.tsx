import type React from "react";
import { ArrowRight } from "lucide-react";
import { ButtonLink } from "../../../components/ui/button";
import { Badge } from "../../../components/ui/Badge";
import { AppPanel, SectionCard } from "../../../components/ui/compositions";
import { BodyText, CardTitle, Eyebrow, MutedText, SectionTitle } from "../../../components/ui/typography";

export function LobbyTutorialSection() {
  return (
    <section
      aria-labelledby="geoduels-seo-heading"
      className="w-full pointer-events-auto"
    >
      <AppPanel className="space-y-6 rounded-2xl p-5 text-left sm:p-6">
        <div className="flex flex-col items-start justify-between gap-5 sm:flex-row sm:items-end">
          <div className="max-w-3xl space-y-3">
            <Badge tone="success">Tutorial</Badge>
            <SectionTitle
              id="geoduels-seo-heading"
            >
              GeoDuels
            </SectionTitle>
            <BodyText>
              A free GeoGuessr-inspired Street View game. Queue for ranked
              matches against other players, with friends, or jump into
              singleplayer.
            </BodyText>
          </div>
          <ButtonLink
            href="/help"
            variant="secondary"
            size="lg"
            className="shrink-0 rounded-xl"
          >
            Learn more
            <ArrowRight size={16} />
          </ButtonLink>
        </div>

        <div className="grid gap-5 lg:grid-cols-3">
          <TutorialCard title="How to Play?">
            Find the location, place your guess. The closer you are, the more
            points you get.
          </TutorialCard>
          <TutorialCard title="100% Free (seriously)">
            No subscriptions to play, no pay-to-win. Considered one of the best
            GeoGuessr free alternatives.
          </TutorialCard>
          <TutorialCard title="Ranked & Casual">
            Climb the ladder or practice in casual mode, which not many
            GeoGuessr alternatives offer.
          </TutorialCard>
        </div>
      </AppPanel>
    </section>
  );
}

function TutorialCard(props: { title: string; children: React.ReactNode }) {
  return (
    <SectionCard className="rounded-2xl p-5">
      <CardTitle>
        {props.title}
      </CardTitle>
      <MutedText className="mt-3 leading-prose-lg">{props.children}</MutedText>
    </SectionCard>
  );
}
