import { Gamepad2, HeartPulse, MapPinned, Swords } from "lucide-react";
import Head from "next/head";
import { AppContentRail } from "../app-shell/components/AppContentRail";
import { AppShell } from "../app-shell/components/AppShell";
import { LobbyActionLink, LobbyPanel } from "../lobby/components/lobby-primitives";

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
        className="relative z-10 pb-28 pt-4 sm:pb-12 sm:pt-8"
      >
        <div className="space-y-6">
          <div className="max-w-3xl space-y-4">
            <span className="text-xs font-black uppercase tracking-[0.16em] text-[#77f0be]">
              How to play
            </span>
            <h1 className="text-4xl font-black tracking-tight text-white sm:text-5xl">
              Read the world. Beat the distance.
            </h1>
            <p className="text-base leading-7 text-[#a9bfd4] sm:text-lg">
              GeoDuels turns location guessing into quick head-to-head rounds.
              Here is everything you need before entering your first match.
            </p>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            {sections.map(({ body, icon: Icon, title }, index) => (
              <LobbyPanel key={title} className="p-5 sm:p-6">
                <div className="flex items-start gap-4">
                  <LobbyPanel
                    variant="subtle"
                    density="none"
                    className="flex h-11 w-11 shrink-0 items-center justify-center text-[#77f0be]"
                  >
                    <Icon size={21} />
                  </LobbyPanel>
                  <div>
                    <span className="text-[10px] font-black uppercase tracking-[0.14em] text-[#6b8b80]">
                      Step {index + 1}
                    </span>
                    <h2 className="mt-1 text-xl font-black text-white">{title}</h2>
                    <p className="mt-2 text-sm leading-6 text-[#a9bfd4]">{body}</p>
                  </div>
                </div>
              </LobbyPanel>
            ))}
          </div>

          <LobbyActionLink href="/" size="lg" className="rounded-xl">
            Play GeoDuels
          </LobbyActionLink>
        </div>
      </AppContentRail>
    </AppShell>
  );
}
