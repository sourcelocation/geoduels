import type React from "react";
import Link from "next/link";
import { ArrowUpRight, Github, Heart, Shield, Twitter, UserPlus, Youtube } from "lucide-react";
import MarkdownContent from "../../../components/ui/MarkdownContent";
import { formatChangelogDate } from "../lib/lobby-ui";
import {
  LobbyCardButton,
  LobbyNotice,
  LobbyPanel,
} from "./lobby-primitives";

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
    <LobbyPanel className="flex h-full min-w-0 flex-col p-5 sm:p-6" style={{ animationDelay: "-1s" }}>
      <div className="flex flex-1 flex-col">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <span className="mb-1 block text-[12px] font-bold uppercase tracking-[0.16em] text-[#2ad18f] drop-shadow-sm">
              {changelogEyebrow}
            </span>
            <h2 className="text-[18px] font-extrabold leading-tight tracking-tight text-white drop-shadow-md">
              {changelogTitle}
            </h2>
            {changelogUpdatedAt ? (
              <time dateTime={changelogUpdatedAt} className="mt-2 block text-[12px] font-semibold text-[#a9bfd4]/70">
                Updated {formatChangelogDate(changelogUpdatedAt)}
              </time>
            ) : null}
          </div>
        </div>
        <div className="mt-4 max-h-[8rem] flex-1 overflow-hidden [mask-image:linear-gradient(180deg,black_48%,rgba(0,0,0,0.76)_70%,transparent_100%)] [-webkit-mask-image:linear-gradient(180deg,black_48%,rgba(0,0,0,0.76)_70%,transparent_100%)]">
          <MarkdownContent
            markdown={changelogBody}
            compact
            className="text-[13px] font-semibold leading-6 text-[#a9bfd4]"
          />
        </div>
        <div className="mt-4 flex justify-end">
          <Link
            href={changelogSlug ? `/changelog/${encodeURIComponent(changelogSlug)}` : "/changelog"}
            className="inline-flex shrink-0 items-center gap-1 rounded-full border border-white/10 bg-white/[0.06] px-3 py-2 text-[11px] font-extrabold uppercase tracking-[0.1em] text-[#77f0be] transition hover:bg-white/[0.1] hover:text-white"
          >
            Read
            <ArrowUpRight size={13} />
          </Link>
        </div>
      </div>
    </LobbyPanel>
  );
}

export function DonateCard({ onSupportDonation }: { onSupportDonation: () => Promise<void> }) {
  return (
    <LobbyCardButton onClick={() => void onSupportDonation()} className="group flex min-h-[112px] w-full flex-1 items-center gap-4 p-5" style={{ animationDelay: "-0.75s" }}>
      <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-[#ef476f]/14 text-[#f7a1b5]">
        <Heart size={22} />
      </div>
      <div className="min-w-0 flex-1">
        <span className="mb-1 block text-[12px] font-bold uppercase tracking-[0.16em] text-[#ee7f98]">Donate</span>
        <h3 className="text-[17px] font-extrabold tracking-tight text-white">Support GeoDuels</h3>
        <p className="mt-1 text-[13px] font-semibold leading-snug text-[#a9bfd4]">Help GeoDuels stay ad-free and in active development.</p>
      </div>
      <ArrowUpRight size={17} className="shrink-0 text-white/50 transition-colors group-hover:text-white" />
    </LobbyCardButton>
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
    <LobbyPanel className="flex w-full flex-1 flex-col justify-center gap-4 p-5" style={{ animationDelay: "-1s" }}>
      <span className="block text-[12px] font-bold uppercase tracking-[0.16em] text-[#6b8b80]">Community</span>
      <div className="flex flex-wrap gap-3">
        {links.map((social) => (
          <a key={social.label} href={social.href} target="_blank" rel="noreferrer" aria-label={social.label} className="glass-panel glass-panel-interactive flex h-12 w-12 items-center justify-center rounded-full text-white">
            {social.icon}
          </a>
        ))}
      </div>
    </LobbyPanel>
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
          { href: "/changelog", label: "Changelog" },
          { href: "/privacy", label: "Privacy Policy" },
          { href: "/terms", label: "Terms of Service" },
        ].map((item) => (
          <FooterLink key={item.href} href={item.href}>
            {item.label}
          </FooterLink>
        ))}
        <FooterDot />
        <span className="text-[12px] font-semibold text-[#6b8b80]">{appVersion}</span>
      </div>
    </div>
  );
}

function FooterLink({ href, children }: { href: string; children: string }) {
  return (
    <>
      <Link href={href} className="text-[12px] font-semibold text-[#6b8b80] transition-colors hover:text-white">
        {children}
      </Link>
      <FooterDot />
    </>
  );
}

function FooterDot() {
  return <div className="h-1 w-1 rounded-full bg-[#6b8b80]/40" />;
}

export function InvitePartyCard({
  disabled,
  onClick,
}: {
  disabled: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className="glass-panel glass-panel-interactive lobby-feature-card group flex w-full items-center gap-4 rounded-[20px] p-5 text-left transition disabled:cursor-not-allowed disabled:opacity-60"
    >
      <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-[#2ad18f]/14 text-[#77f0be]">
        <UserPlus size={22} />
      </div>
      <div className="min-w-0 flex-1">
        <span className="mb-1 block text-[12px] font-bold uppercase tracking-[0.16em] text-[#6b8b80]">CUSTOM</span>
        <h3 className="text-[18px] font-extrabold tracking-tight text-white">Private Party</h3>
        <p className="mt-1 text-[13px] leading-relaxed text-[#a9bfd4]">Create a party or join your friend</p>
      </div>
      <ArrowUpRight size={18} className="shrink-0 text-white/50 transition-colors group-hover:text-white" />
    </button>
  );
}

export function PartyErrorNotice({ message }: { message: string }) {
  if (!message) return null;
  return (
    <div role="alert" className="mb-4 w-full max-w-[1160px] pointer-events-auto">
      <LobbyNotice title="Party Error" tone="danger">
        <span className="flex items-start gap-3 text-left text-sm font-semibold leading-6">
          <Shield className="mt-0.5 shrink-0 text-red-200" size={18} />
          <span>{message}</span>
        </span>
      </LobbyNotice>
    </div>
  );
}
