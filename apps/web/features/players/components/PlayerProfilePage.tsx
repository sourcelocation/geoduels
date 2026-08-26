import { motion, useReducedMotion } from "framer-motion";
import Head from "next/head";
import { useRouter } from "next/router";
import { useState } from "react";
import { AppShell } from "../../app-shell/components/AppShell";
import { AppContentRail } from "../../app-shell/components/AppContentRail";
import { AppPanel } from "../../../components/ui/compositions";
import { CenteredSpinner } from "../../../components/ui/Spinner";
import { getSiteURL } from "../../../lib/site";
import { useProfileEditor } from "../hooks/use-profile-editor";
import { usePlayerProfile } from "../hooks/use-player-profile";
import { useAuthState } from "../../auth/components/AuthProvider";
import type { PublicPlayerProfile } from "../types";
import {
  ProfileBadges,
  ProfileHistory,
  ProfileOverview,
} from "./PlayerProfileSections";
import { ProfileSocialActions } from "../../social/components/ProfileSocialActions";

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
  const auth = useAuthState();
  const router = useRouter();
  const profile = profileQuery.data;
  const owner = !!profile && auth.isRegistered && auth.userId === profile.userId;
  const editor = useProfileEditor(
    profile,
    owner ? auth.accessToken : "",
    (nickname) => void router.replace(`/players/${encodeURIComponent(nickname)}`),
  );
  const matches = matchesQuery.data?.pages.flatMap((page) => page.matches) || [];
  const profilePath = `/players/${encodeURIComponent(profile?.displayName || playerId)}`;
  if (!playerId || profileQuery.isLoading) {
    return (
      <AppShell activeNavRoute={null}>
        <ProfileMain>
          <CenteredSpinner label="Loading player profile" className="min-h-[520px]" />
        </ProfileMain>
      </AppShell>
    );
  }
  if (profileQuery.isError || !profile) {
    return (
      <AppShell activeNavRoute={null}>
        <Head>
          <title>Player not found | GeoDuels</title>
          <meta name="robots" content="noindex" />
        </Head>
        <ProfileMain>
          <AppPanel className="rounded-2xl p-10 text-center">
            <h1 className="text-heading-lg font-strong text-content-primary">Player not found</h1>
            <p className="mt-3 text-content-secondary">
              This profile does not exist or is no longer available.
            </p>
          </AppPanel>
        </ProfileMain>
      </AppShell>
    );
  }

  return (
    <>
      <AppShell
        activeNavRoute={null}
      >
        <ProfileMetadata profile={profile} path={profilePath} />
        <ProfileMain>
          <ProfileContent>
            <ProfileOverview
              profile={profile}
              editor={editor}
              owner={owner}
              socialActions={<ProfileSocialActions profile={profile} />}
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
      className="relative z-content pb-28 pt-4 sm:pb-12 sm:pt-8"
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
