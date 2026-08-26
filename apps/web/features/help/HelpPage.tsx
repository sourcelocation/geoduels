import { Gamepad2, HeartPulse, MapPinned, Swords } from "lucide-react";
import Head from "next/head";
import { AppContentRail } from "../app-shell/components/AppContentRail";
import { AppShell } from "../app-shell/components/AppShell";
import { ButtonLink } from "../../components/ui/button";
import { AppPanel, SectionCard } from "../../components/ui/compositions";
import { BodyText, Eyebrow, SectionTitle } from "../../components/ui/typography";

const sections = [
  {
    icon: MapPinned,
    title: "Find the location",
    body: "Every round drops both players into the same Street View location. Explore when the rules allow it, read the landscape, and place your guess on the map.",
  },
  {
    icon: Gamepad2,
    title: "Join a match",
    body: "Choose the allowed rulesets and enter the Duel queue. Ranked matchmaking looks for an opponent near your current MMR.",
  },
  {
    icon: HeartPulse,
    title: "Protect your HP",
    body: "Both players begin with 6,000 HP. The closer guess wins the round and deals damage based on the difference between both distances.",
  },
  {
    icon: Swords,
    title: "Win the duel",
    body: "The first guess starts the round countdown. Keep winning rounds until your opponent reaches zero HP. Singleplayer uses the same geography skills without an opponent.",
  },
];

export function HelpPage() {
  return (
    <AppShell activeNavRoute={null} navigationHidden>
      <Head>
        <title>How to play | GeoDuels</title>
        <meta
          name="description"
          content="Learn the basics of GeoDuels matchmaking, guessing, scoring, and duel damage."
        />
      </Head>
      <AppContentRail
        as="main"
        size="standard"
        className="relative z-content pb-28 pt-4 sm:pb-12 sm:pt-8"
      >
        <AppPanel className="space-y-6 rounded-2xl p-5 sm:p-6">
          <div className="max-w-3xl space-y-4">
            <Eyebrow>How to play</Eyebrow>
            <SectionTitle>
              Read the world. Beat the distance.
            </SectionTitle>
            <BodyText className="sm:text-heading-sm">
              GeoDuels turns location guessing into quick head-to-head rounds.
              Here is everything you need before entering your first match.
            </BodyText>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            {sections.map(({ body, icon: Icon, title }, index) => (
              <SectionCard key={title} className="rounded-2xl p-5 sm:p-6">
                <div className="flex items-start gap-4">
                  <SectionCard
                    className="flex h-11 w-11 shrink-0 items-center justify-center text-status-success"
                  >
                    <Icon size={21} />
                  </SectionCard>
                  <div>
                    <span className="text-label font-strong text-content-secondary">
                      Step {index + 1}
                    </span>
                    <h2 className="mt-1 text-heading-sm font-strong text-content-primary">{title}</h2>
                    <p className="mt-2 text-body-sm leading-body text-content-secondary">{body}</p>
                  </div>
                </div>
              </SectionCard>
            ))}
          </div>

          <ButtonLink href="/" variant="primary" size="lg" className="rounded-xl">
            Play GeoDuels
          </ButtonLink>
        </AppPanel>
      </AppContentRail>
    </AppShell>
  );
}
