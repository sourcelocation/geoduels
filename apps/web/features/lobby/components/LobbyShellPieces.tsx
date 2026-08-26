import type React from "react";
import Link from "next/link";
import { ArrowUpRight, Github, Heart, Shield, Twitter, UserPlus, Youtube } from "lucide-react";
import MarkdownContent from "../../../components/ui/MarkdownContent";
import { CardTitle, Eyebrow, HelperText, MutedText } from "../../../components/ui/typography";
import { formatChangelogDate } from "../lib/lobby-ui";
import {
  LobbyNotice,
} from "./lobby-primitives";
import { AppCardButton, AppChromeIconLink, AppPanel } from "../../../components/ui/compositions";

export function NewsPanel({
  changelogEyebrow,
  changelogMarkdown,
  changelogSlug,
  changelogTitle,
  changelogUpdatedAt,
}: {
  changelogEyebrow: string;
  changelogMarkdown: string;
  changelogSlug: string;
  changelogTitle: string;
  changelogUpdatedAt: string;
}) {
  const changelogBody = changelogMarkdown.trim() || "No changelog content yet.";
  return (
    <AppPanel className="flex h-full min-w-0 flex-col rounded-2xl p-5 sm:p-6" style={{ animationDelay: "-1s" }}>
      <div className="flex flex-1 flex-col">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <Eyebrow className="mb-1">
              {changelogEyebrow}
            </Eyebrow>
            <CardTitle className="drop-shadow-md">
              {changelogTitle}
            </CardTitle>
            {changelogUpdatedAt ? (
              <time dateTime={changelogUpdatedAt} className="mt-2 block text-caption font-medium text-content-secondary">
                Updated {formatChangelogDate(changelogUpdatedAt)}
              </time>
            ) : null}
          </div>
        </div>
        {/* Artwork mask: the exact fade is part of this lobby-specific visual contract. */}
        <div className="mt-4 max-h-[8rem] flex-1 overflow-hidden [mask-image:linear-gradient(180deg,black_48%,rgba(0,0,0,0.76)_70%,transparent_100%)] [-webkit-mask-image:linear-gradient(180deg,black_48%,rgba(0,0,0,0.76)_70%,transparent_100%)]">
          <MarkdownContent
            markdown={changelogBody}
            compact
            className="text-body-sm text-content-secondary"
          />
        </div>
        <div className="mt-4 flex justify-end">
          <Link
            href={changelogSlug ? `/changelog/${encodeURIComponent(changelogSlug)}` : "/changelog"}
            className="inline-flex shrink-0 items-center gap-1 rounded-full border border-border-default bg-surface-fill px-3 py-2 text-label font-strong text-status-success transition hover:border-border-strong hover:bg-surface-grouped hover:text-content-primary"
          >
            Read
            <ArrowUpRight size={13} />
          </Link>
        </div>
      </div>
    </AppPanel>
  );
}

export function DonateCard({ onSupportDonation }: { onSupportDonation: () => Promise<void> }) {
  return (
    <AppCardButton onClick={() => void onSupportDonation()} className="group flex h-full min-h-44 w-full flex-1 items-center gap-4 p-5" style={{ animationDelay: "-0.75s" }}>
      <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-brand-pink/15 text-brand-pink-soft">
        <Heart size={22} />
      </div>
      <div className="min-w-0 flex-1">
        <Eyebrow className="mb-1 text-brand-pink-soft">Donate</Eyebrow>
        <CardTitle>Support GeoDuels</CardTitle>
        <MutedText className="mt-1 leading-heading">Help GeoDuels stay ad-free!</MutedText>
      </div>
      <ArrowUpRight size={17} className="shrink-0 text-content-secondary transition-colors group-hover:text-content-primary" />
    </AppCardButton>
  );
}

export function SocialLinksCard() {
  const links = [
    {
      href: "https://discord.gg/xxz8V9UU7Z",
      label: "Discord",
      icon: <svg viewBox="0 0 127.14 96.36" className="h-5 w-5" aria-hidden="true"><path fill="currentColor" d="M107.7 8.07A105.15 105.15 0 0 0 81.47 0a72.06 72.06 0 0 0-3.36 6.83 97.68 97.68 0 0 0-29.11 0A72.37 72.37 0 0 0 45.64 0 105.89 105.89 0 0 0 19.39 8.09C2.79 32.65-1.71 56.6.54 80.21a105.73 105.73 0 0 0 32.17 16.15 77.7 77.7 0 0 0 6.89-11.11 68.42 68.42 0 0 1-10.85-5.18c.91-.66 1.8-1.34 2.66-2.04a75.57 75.57 0 0 0 64.32 0c.87.71 1.76 1.39 2.66 2.04a68.68 68.68 0 0 1-10.87 5.19 77 77 0 0 0 6.89 11.1 105.25 105.25 0 0 0 32.19-16.14c2.64-27.38-4.52-51.11-18.9-72.15ZM42.45 65.69c-6.27 0-11.43-5.73-11.43-12.78s5.05-12.79 11.43-12.79 11.54 5.78 11.43 12.79-5.06 12.78-11.43 12.78Zm42.24 0c-6.27 0-11.43-5.73-11.43-12.78s5.05-12.79 11.43-12.79 11.54 5.78 11.43 12.79-5.05 12.78-11.43 12.78Z" /></svg>,
    },
    { href: "https://github.com/sourcelocation/geoduels", label: "GitHub", icon: <Github size={20} /> },
    { href: "http://twitter.com/sourceloc", label: "Twitter", icon: <Twitter size={20} /> },
    { href: "https://youtube.com/@sourcelocation", label: "YouTube", icon: <Youtube size={20} /> },
  ];
  return (
    <AppPanel className="flex h-full min-h-44 w-full flex-1 flex-col justify-center gap-4 rounded-2xl p-5" style={{ animationDelay: "-1s" }}>
      <Eyebrow className="text-content-secondary">Community</Eyebrow>
      <div className="flex flex-wrap gap-3">
        {links.map((social) => (
          <AppChromeIconLink key={social.label} href={social.href} target="_blank" rel="noreferrer" aria-label={social.label}>
            {social.icon}
          </AppChromeIconLink>
        ))}
      </div>
    </AppPanel>
  );
}

export function LobbyUpdatesPanel({
  newsPanel,
  donateCard,
  socialLinksCard,
}: {
  newsPanel: React.ReactNode;
  donateCard: React.ReactNode;
  socialLinksCard: React.ReactNode;
}) {
  return (
    <div className="grid w-full items-stretch gap-5 sm:gap-6 lg:grid-cols-[minmax(0,1fr)_300px]">
      <div className="min-w-0">{newsPanel}</div>
      <div className="flex min-w-0 flex-col gap-5 sm:gap-6">
        {socialLinksCard}
        {donateCard}
      </div>
    </div>
  );
}

export function LegalFooter({ appVersion }: { appVersion: string }) {
  return (
    <div className="pointer-events-auto flex w-full items-center justify-center px-1 py-1">
      <div className="flex items-center gap-6">
        {[
          { href: "/help", label: "How to Play" },
          { href: "/changelog", label: "Changelog" },
          { href: "/privacy", label: "Privacy Policy" },
          { href: "/terms", label: "Terms of Service" },
        ].map((item) => (
          <FooterLink key={item.href} href={item.href}>
            {item.label}
          </FooterLink>
        ))}
        <FooterDot />
        <span className="text-caption font-semibold text-content-secondary">{appVersion}</span>
      </div>
    </div>
  );
}

function FooterLink({ href, children }: { href: string; children: string }) {
  return (
    <>
    <Link href={href} className="text-label font-semibold text-content-secondary transition-colors hover:text-content-primary">
        {children}
      </Link>
      <FooterDot />
    </>
  );
}

function FooterDot() {
  return <div className="h-1 w-1 rounded-full bg-content-secondary/40" />;
}

export function InvitePartyCard({
  disabled,
  onClick,
}: {
  disabled: boolean;
  onClick: () => void;
}) {
  return (
    <AppCardButton
      onClick={onClick}
      disabled={disabled}
      className="group flex w-full items-center gap-4 p-5"
    >
      <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-status-success/10 text-status-success">
        <UserPlus size={22} />
      </div>
      <div className="min-w-0 flex-1">
        <Eyebrow className="mb-1 text-content-secondary">Custom</Eyebrow>
        <CardTitle>Private Party</CardTitle>
        <HelperText className="mt-1 leading-prose">Create a party or join your friend</HelperText>
      </div>
      <ArrowUpRight size={18} className="shrink-0 text-content-secondary transition-colors group-hover:text-content-primary" />
    </AppCardButton>
  );
}

export function PartyErrorNotice({ message }: { message: string }) {
  if (!message) return null;
  return (
    <div className="mb-4 w-full max-w-[1160px] pointer-events-auto">
      <LobbyNotice title="Party Error" tone="danger">
        <span className="flex items-start gap-3 text-left text-body-sm font-semibold leading-body">
          <Shield className="mt-0.5 shrink-0 text-status-danger" size={18} />
          <span>{message}</span>
        </span>
      </LobbyNotice>
    </div>
  );
}
