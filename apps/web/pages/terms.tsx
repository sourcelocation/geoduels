import { LegalDocumentPage } from "../features/legal/LegalDocumentPage";

const sections = [
  {
    title: "1. Introduction",
    body: [
      "These Terms of Service govern your use of GeoDuels, an open-source online geography duel game operated by Matthew Anisovich, also known as sourcelocation. By using GeoDuels, you agree to these Terms.",
      "If you do not agree to these Terms, do not use GeoDuels.",
    ],
  },
  {
    title: "2. Eligibility",
    body: [
      "You must be at least 13 years old to use GeoDuels. If you are under the age required to use online services in your location, you may use GeoDuels only with permission from a parent or guardian.",
      "You are responsible for making sure your use of GeoDuels is lawful where you live.",
    ],
  },
  {
    title: "3. Accounts and Security",
    body: [
      "GeoDuels may support Discord sign-in, guest access, and migration-only Google account recovery. You are responsible for activity under your account and for keeping access to your Discord account, any migration Google account, devices, and sessions secure.",
      "You may not share, sell, transfer, or misuse accounts. GeoDuels may restrict, suspend, or terminate accounts used for abuse, cheating, fraud, security risks, or violations of these Terms.",
      "You may request deletion of your account by contacting me@sourceloc.net. During Discord migration, in-app deletion may be available for replacing a current Discord-only account with an older migrated account.",
    ],
  },
  {
    title: "4. License to Use GeoDuels",
    body: [
      "GeoDuels gives you a limited, personal, non-exclusive, non-transferable, revocable permission to access and use the service for lawful gameplay and community participation.",
      "GeoDuels, its code, design, systems, branding, and related materials remain owned by their respective owners. Open-source components are governed by their applicable licenses.",
    ],
  },
  {
    title: "5. User Content",
    body: [
      "User content includes display names, avatars, party information, reports, report reasons, and any other information you submit or make available through GeoDuels.",
      "You are responsible for your user content. You may not submit content that is illegal, hateful, harassing, threatening, sexually explicit, infringing, impersonating, spammy, or otherwise abusive.",
      "GeoDuels may remove, restrict, or moderate user content. Nickname checks and other automated or manual moderation tools may be used to block bad names or abusive content.",
      "By submitting user content, you give GeoDuels permission to host, display, process, reproduce, and use it as needed to operate, moderate, secure, and improve the service.",
    ],
  },
  {
    title: "6. Fair Play and Competitive Integrity",
    body: [
      "Competitive integrity is central to GeoDuels. You may not obtain or attempt to obtain an unfair advantage over other players in any context where your conduct affects other players' experience, ratings, standings, or outcomes. This rule applies regardless of the method used — including software, hardware, information sources, coordination with other users, or exploitation of bugs, design quirks, or system behavior.",
      "This rule does not apply to a friendly match — a match, lobby, or party in which every participant has knowingly and mutually agreed to the rules of play, and which remains confined to those participants. For example, two friends in a private lobby who agree in advance that one of them may use a location-revealing tool while playing against the other are not gaining an unfair advantage, because the disadvantaged participant consented. Friendly matches may not feed into ranked systems, public leaderboards, or any other setting that affects players outside the agreed participants.",
      "Conduct that typically violates the rule above includes — without limitation — bots, scripts, macros, overlays, location hint tools, external databases used during a match, unauthorized browser extensions, automated guessing, account sharing, boosting, smurf abuse, coordinated win-trading, intentionally throwing matches, MMR manipulation, and exploiting bugs or system behavior. The official GeoDuels browser extension is authorized only for the gameplay features presented by GeoDuels.",
      "Separately, you may not tamper with clients, network traffic, tokens, matchmaking, game state APIs, maps, location data, or any infrastructure used by GeoDuels. This rule protects the service itself and applies regardless of consent, because it is not about competitive fairness between specific players.",
    ],
  },
  {
    title: "7. Prohibited Conduct",
    body: [
      "You may not harass, threaten, impersonate, spam, abuse reports, evade bans, interfere with other players, disrupt matchmaking or realtime services, scrape or reverse engineer the service except as allowed by applicable open-source licenses, attack infrastructure, or use GeoDuels for unlawful activity.",
      "False, malicious, or abusive reports may lead to report mutes, account restrictions, or bans.",
    ],
  },
  {
    title: "8. Moderation and Enforcement",
    body: [
      "GeoDuels may review reports, gameplay signals, match history, account history, technical logs, and moderation evidence to protect players and ranked integrity.",
      "Enforcement may include warnings, content removal, report mutes, queue restrictions, rating adjustments, MMR resets, rating refunds, temporary suspensions, permanent bans, IP or signup restrictions, or other reasonable measures.",
      "GeoDuels does not currently offer a formal appeal process. You may contact me@sourceloc.net about moderation questions, but moderation decisions are not guaranteed to be reconsidered.",
    ],
  },
  {
    title: "9. Rankings, Matches, and Progress",
    body: [
      "Rankings, MMR, match results, statistics, leaderboards, and other progress systems are part of the service and have no monetary value. GeoDuels may adjust, reset, remove, or recalculate them to fix bugs, address cheating, preserve ranked integrity, or improve the service.",
      "GeoDuels does not guarantee that match results, rankings, lobbies, reconnects, or gameplay history will always be accurate, available, or preserved.",
    ],
  },
  {
    title: "10. Third-Party Services",
    body: [
      "GeoDuels may use third-party services including Discord sign-in, migration-only Google OAuth, Google Maps or Street View-related services, hosting providers, infrastructure providers, and notification tools such as Discord webhooks.",
      "Third-party services may have their own terms and privacy policies. GeoDuels is not responsible for third-party services or their practices.",
    ],
  },
  {
    title: "11. Payments and Paid Features",
    body: [
      "GeoDuels may add paid features, subscriptions, cosmetics, donations, or other monetization in the future. Any paid features may be subject to additional terms shown at the time of purchase or use.",
      "Unless required by law or stated otherwise in additional terms, purchases may be non-refundable.",
    ],
  },
  {
    title: "12. Service Availability and Changes",
    body: [
      "GeoDuels may change, suspend, limit, or discontinue any part of the service at any time. The service may be unavailable because of maintenance, incidents, upgrades, network problems, third-party services, or other reasons.",
      "Features, maps, matchmaking rules, ranked rules, moderation systems, and balancing may change over time.",
    ],
  },
  {
    title: "13. Termination",
    body: [
      "You may stop using GeoDuels at any time. GeoDuels may suspend or terminate your access if you violate these Terms, create risk for the service or players, or use the service in a way that is harmful or unlawful.",
      "Sections that by their nature should survive termination, including moderation history, disclaimers, limitations of liability, and account integrity records, may continue to apply.",
    ],
  },
  {
    title: "14. Disclaimers",
    body: [
      "GeoDuels is provided “as is” and “as available.” To the maximum extent allowed by law, GeoDuels makes no warranties about availability, accuracy, reliability, security, matchmaking quality, ranking accuracy, third-party services, or fitness for a particular purpose.",
      "Use GeoDuels at your own risk.",
    ],
  },
  {
    title: "15. Limitation of Liability",
    body: [
      "To the maximum extent allowed by law, GeoDuels and its operator will not be liable for indirect, incidental, special, consequential, exemplary, or punitive damages, or for lost data, lost rankings, lost progress, service interruptions, account restrictions, or third-party service issues arising from your use of GeoDuels.",
      "Nothing in these Terms limits liability where it cannot legally be limited.",
    ],
  },
  {
    title: "16. Changes to These Terms",
    body: [
      "GeoDuels may update these Terms from time to time. The updated version will be posted on this page with a new “Last updated” date. Continued use of GeoDuels after an update means you accept the updated Terms.",
    ],
  },
  {
    title: "17. Contact",
    body: ["Questions about these Terms can be sent to me@sourceloc.net."],
  },
];

export default function TermsPage() {
  return (
    <LegalDocumentPage
      title="Terms of Service"
      updatedAt="June 25, 2026"
      description="These Terms explain the rules for using GeoDuels and help protect the game, players, and ranked integrity."
      sections={sections}
    />
  );
}
