import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import Head from "next/head";
import { useRouter } from "next/router";
import { useState } from "react";
import { AppShell } from "../../app-shell/components/AppShell";
import { AppContentRail } from "../../app-shell/components/AppContentRail";
import { Surface } from "../../../components/ui/Surface";
import { getSiteURL } from "../../../lib/site";
import { useProfileEditor } from "../hooks/use-profile-editor";
import { useOptionalViewer, usePlayerProfile } from "../hooks/use-player-profile";
import type { PublicPlayerProfile } from "../types";
import { AccountSettingsModal } from "./AccountSettingsModal";
import {
  ProfileBadges,
  ProfileHistory,
  ProfileOverview,
} from "./PlayerProfileSections";

export function PlayerProfilePage({
  playerId,
  initialProfile,
}: {
  playerId: string;
  initialProfile?: PublicPlayerProfile;
}) {
  const [historyFilter, setHistoryFilter] = useState<"all" | "ranked">("all");
  const { profileQuery, matchesQuery } = usePlayerProfile(
    playerId,
    initialProfile,
    historyFilter,
  );
  const viewer = useOptionalViewer().data;
  const router = useRouter();
  const profile = profileQuery.data;
  const owner = !!profile && viewer?.userId === profile.userId;
  const editor = useProfileEditor(
    profile,
    owner ? viewer?.accessToken || "" : "",
    (nickname) => void router.replace(`/players/${encodeURIComponent(nickname)}`),
  );
  const matches = matchesQuery.data?.pages.flatMap((page) => page.matches) || [];
  const profilePath = `/players/${encodeURIComponent(profile?.displayName || playerId)}`;
  const settingsOpen =
    owner && router.isReady && router.query.settings === "account";
  const shellViewer = viewer
    ? {
        userId: viewer.userId,
        displayName: viewer.displayName,
        avatarUrl: viewer.avatarUrl,
        avatarFallback: (viewer.displayName || "?").slice(0, 1).toUpperCase(),
        mmr: viewer.mmr,
        selectedBadge: viewer.selectedBadge,
      }
    : null;
  const setSettings = (open: boolean) => {
    const query = { ...router.query };
    if (open) query.settings = "account";
    else delete query.settings;
    void router.replace({ pathname: router.pathname, query }, undefined, {
      shallow: true,
    });
  };

  if (!playerId || profileQuery.isLoading) {
    return (
      <AppShell activeNavRoute={null} viewer={shellViewer} isAdmin={!!viewer?.isAdmin} isModerator={!!viewer?.isModerator}>
        <ProfileMain>
          <div className="h-[520px] animate-pulse rounded-3xl bg-white/[0.06]" />
        </ProfileMain>
      </AppShell>
    );
  }
  if (profileQuery.isError || !profile) {
    return (
      <AppShell activeNavRoute={null} viewer={shellViewer} isAdmin={!!viewer?.isAdmin} isModerator={!!viewer?.isModerator}>
        <Head>
          <title>Player not found | GeoDuels</title>
          <meta name="robots" content="noindex" />
        </Head>
        <ProfileMain>
          <Surface variant="gameGlass" className="rounded-3xl p-10 text-center">
            <h1 className="text-3xl font-black">Player not found</h1>
            <p className="mt-3 text-[#a9bfd4]">
              This profile does not exist or is no longer available.
            </p>
          </Surface>
        </ProfileMain>
      </AppShell>
    );
  }

  return (
    <>
      <AppShell
        activeNavRoute={null}
        viewer={shellViewer}
        isAdmin={!!viewer?.isAdmin}
        isModerator={!!viewer?.isModerator}
      >
        <ProfileMetadata profile={profile} path={profilePath} />
        <ProfileMain>
          <ProfileContent>
            <ProfileOverview
              profile={profile}
              editor={editor}
              owner={owner}
              onSettings={() => setSettings(true)}
            />
            <ProfileBadges profile={profile} editor={editor} owner={owner} />
            <ProfileHistory
              matches={matches}
              query={matchesQuery}
              filter={historyFilter}
              onFilterChange={setHistoryFilter}
            />
          </ProfileContent>
        </ProfileMain>
      </AppShell>
      <AnimatePresence>
        {settingsOpen ? (
          <AccountSettingsModal
            onClose={() => setSettings(false)}
            profilePath={profilePath}
          />
        ) : null}
      </AnimatePresence>
    </>
  );
}

function ProfileMetadata({
  profile,
  path,
}: {
  profile: PublicPlayerProfile;
  path: string;
}) {
  const siteURL = getSiteURL();
  const winRate = profile.gamesPlayed
    ? Math.round((profile.wins / profile.gamesPlayed) * 100)
    : 0;
  const description = `${profile.mmr} MMR · ${profile.gamesPlayed} duels · ${winRate}% duel win rate`;
  return (
    <Head>
      <title>{profile.displayName} | GeoDuels</title>
      <meta name="description" content={`${profile.displayName} has ${description}.`} />
      <meta name="robots" content="index,follow" />
      <link rel="canonical" href={`${siteURL}${path}`} />
      <meta property="og:title" content={`${profile.displayName} | GeoDuels`} />
      <meta property="og:description" content={description} />
      <meta property="og:url" content={`${siteURL}${path}`} />
      <meta
        property="og:image"
        content={profile.avatarUrl || `${siteURL}/logo.v2.png`}
      />
    </Head>
  );
}

function ProfileMain({ children }: { children: React.ReactNode }) {
  return (
    <AppContentRail
      as="main"
      size="standard"
      className="relative z-10 pb-28 pt-4 sm:pb-12 sm:pt-8"
    >
      {children}
    </AppContentRail>
  );
}

function ProfileContent({ children }: { children: React.ReactNode }) {
  const reduceMotion = useReducedMotion();
  return (
    <motion.div
      initial={reduceMotion ? false : { opacity: 0, y: 14, scale: 0.985 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      transition={{ duration: reduceMotion ? 0 : 0.24 }}
      className="space-y-5"
    >
      {children}
    </motion.div>
  );
}
