import { LegalDocumentPage } from "../features/legal/LegalDocumentPage";

const sections = [
  {
    title: "1. Who We Are",
    body: [
      "GeoDuels is an open-source online geography duel game operated by Matthew Anisovich, also known as sourcelocation. This Privacy Policy explains how GeoDuels collects, uses, stores, and shares information when you use the website, game, accounts, matchmaking, rankings, moderation tools, and related services.",
      "For privacy questions or account deletion requests, contact me@sourceloc.net.",
    ],
  },
  {
    title: "2. Information We Collect",
    body: [
      "Account information. If you sign in with Discord, we receive information needed to create and maintain your account, such as your Discord account identifier, email address when available, display name, and avatar URL. During account migration, Google OAuth may be used only to prove ownership of an older GeoDuels account. GeoDuels may also create guest accounts with generated identifiers and display names.",
      "Profile and ranking information. We store your display name, avatar, MMR, rating data, games played, wins, ranked statistics, account status, moderator/admin flags, bans, and related account history.",
      "Authentication and session information. We store session records, secure refresh-session information, IP addresses, user agents, timestamps, and related security data used to keep accounts signed in and protect the service.",
      "Gameplay information. We store match IDs, players in a match, guesses, coordinates, round timing, scores, HP, match snapshots, match results, reconnect/session state, party data, invite codes, and match history.",
      "Moderation and safety information. We store reports, report categories and reasons, reporter and reported player IDs, moderation cases, evidence, moderator notes, actions, bans, report reputation signals, and related audit history.",
      "Technical information. We may collect logs, diagnostic data, request metadata, error information, service health data, and timing information needed to operate, secure, debug, and improve GeoDuels.",
      "Analytics information. GeoDuels may process basic analytics about matches, rankings, service usage, and timing to understand gameplay and operate the service.",
    ],
  },
  {
    title: "3. How We Use Information",
    body: [
      "We use information to provide accounts, authentication, matchmaking, lobbies, real-time gameplay, reconnects, rankings, leaderboards, match history, moderation, abuse prevention, service reliability, debugging, and analytics.",
      "We also use information to enforce the Terms of Service, investigate cheating or abuse, protect ranked integrity, issue rating adjustments or refunds, respond to requests, and maintain the security of GeoDuels.",
    ],
  },
  {
    title: "4. Cookies and Similar Technologies",
    body: [
      "GeoDuels uses cookies and similar browser technologies for authentication, session management, security, and service operation. The main session cookie is designed to be protected from app JavaScript where supported by the browser.",
      "Third-party services may use cookies or similar technologies according to their own policies.",
    ],
  },
  {
    title: "5. Third-Party Services",
    body: [
      "GeoDuels uses Discord for sign-in and Google Maps/Street View-related gameplay. During migration, Google OAuth may be used to connect older accounts.",
      "GeoDuels may use infrastructure providers for hosting, databases, caching, networking, logs, and service operations. Moderation alerts may be sent to configured Discord webhooks or similar notification tools.",
      "Third-party services process information under their own terms and privacy policies. GeoDuels is not responsible for third-party privacy practices.",
    ],
  },
  {
    title: "6. Sharing Information",
    body: [
      "We do not sell your personal information. We may share information with service providers that help operate GeoDuels, with third-party services you use through GeoDuels, when needed for security or moderation, when required by law, or to protect GeoDuels, players, or the public.",
      "Some gameplay and profile information, such as display names, avatars, rankings, match outcomes, and leaderboard information, may be visible to other players.",
    ],
  },
  {
    title: "7. Data Retention",
    body: [
      "We retain account, gameplay, ranking, moderation, and technical information for as long as needed to operate GeoDuels, preserve ranked integrity, investigate abuse, comply with legal obligations, resolve disputes, and maintain security.",
      "Some data, such as match history, moderation history, bans, and ranking records, may be retained even if an account is no longer active where retention is needed for integrity, safety, audit, or operational reasons.",
    ],
  },
  {
    title: "8. Security",
    body: [
      "GeoDuels uses technical and organizational measures intended to protect information, including secure session design, limited access to administrative tools, and operational safeguards. No online service can guarantee perfect security.",
      "You are responsible for keeping access to your Discord account, any migration Google account, and devices secure.",
    ],
  },
  {
    title: "9. Your Choices and Rights",
    body: [
      "You may request deletion of your GeoDuels account by emailing me@sourceloc.net from an address that can reasonably identify your account. During Discord migration, GeoDuels may offer an in-app option to delete the current Discord-only account data before migrating an older Google-backed account.",
      "Depending on where you live, you may have rights to access, delete, correct, or object to certain processing of your personal information. Contact me@sourceloc.net to make a request. GeoDuels may need to retain certain information where required or reasonably necessary for safety, fraud prevention, moderation, legal, or operational reasons.",
    ],
  },
  {
    title: "10. Children",
    body: [
      "GeoDuels is intended for users who are at least 13 years old. If you are under the age required to use online services in your location, you may use GeoDuels only with permission from a parent or guardian.",
      "If you believe a child has provided personal information in violation of this policy, contact me@sourceloc.net.",
    ],
  },
  {
    title: "11. Changes",
    body: [
      "This Privacy Policy may be updated from time to time. The updated version will be posted on this page with a new “Last updated” date. Continued use of GeoDuels after an update means the updated policy applies.",
    ],
  },
  {
    title: "12. Contact",
    body: ["Privacy requests and questions can be sent to me@sourceloc.net."],
  },
];

export default function PrivacyPage() {
  return (
    <LegalDocumentPage
      title="Privacy Policy"
      updatedAt="May 10, 2026"
      description="This policy describes how GeoDuels handles information when you use the game and related services."
      sections={sections}
    />
  );
}
