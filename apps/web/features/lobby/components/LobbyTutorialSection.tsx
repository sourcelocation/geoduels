import type React from "react";
import { ArrowRight } from "lucide-react";
import { LobbyActionLink, LobbyPanel } from "./lobby-primitives";

export function LobbyTutorialSection() {
  return (
    <section
      aria-labelledby="geoduels-seo-heading"
      className="w-full border-t border-white/10 py-8 pointer-events-auto sm:py-10"
    >
      <div className="space-y-6 text-left">
        <div className="flex flex-col items-start justify-between gap-5 sm:flex-row sm:items-end">
          <div className="max-w-3xl space-y-3">
            <span className="inline-flex rounded-full border border-[#2ad18f]/30 bg-[#2ad18f]/10 px-3 py-1 text-[11px] font-bold uppercase tracking-[0.16em] text-[#7de3b7]">
              Tutorial
            </span>
            <h1
              id="geoduels-seo-heading"
              className="text-[30px] font-extrabold leading-tight tracking-tight text-white sm:text-[40px]"
            >
              GeoDuels
            </h1>
            <p className="text-[15px] leading-7 text-[#a9bfd4] sm:text-base">
              A free GeoGuessr-inspired Street View game. Queue for ranked
              matches against other players, with friends, or jump into
              singleplayer.
            </p>
          </div>
          <LobbyActionLink
            href="/help"
            variant="secondary"
            size="lg"
            className="shrink-0 rounded-xl"
          >
            Learn more
            <ArrowRight size={16} />
          </LobbyActionLink>
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
      </div>
    </section>
  );
}

function TutorialCard(props: { title: string; children: React.ReactNode }) {
  return (
    <LobbyPanel className="rounded-2xl p-5">
      <h2 className="text-lg font-extrabold tracking-tight text-white">
        {props.title}
      </h2>
      <p className="mt-3 text-sm leading-7 text-[#a9bfd4]">{props.children}</p>
    </LobbyPanel>
  );
}
