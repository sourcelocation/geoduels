import { LegalDocumentPage } from "../../features/legal/LegalDocumentPage";

const sections = [
  {
    title: "1. Overview",
    body: [
      "GeoDuels Enhancer is the official browser extension for GeoDuels. It is used only to enable gameplay enhancements inside GeoDuels and Google Maps Embed Street View frames opened by GeoDuels.",
      "The extension helps GeoDuels support features such as hidden street names, No Move rules, compass heading, and match-rule syncing inside Street View.",
    ],
  },
  {
    title: "2. Information the Extension Collects",
    body: [
      "GeoDuels Enhancer does not collect, sell, share, or transmit personal information.",
      "The extension does not collect browsing history, account credentials, personal communications, payment information, health information, or other personal data.",
      "The extension does not use analytics, advertising, tracking, remote code, or a background worker.",
    ],
  },
  {
    title: "3. How the Extension Works",
    body: [
      "The extension may run on GeoDuels pages so the website can detect that the official extension is installed.",
      "The extension may also run inside Google Maps Embed Street View frames opened by GeoDuels so it can apply match rules, hide street names when selected, enforce No Move gameplay rules, and provide compass heading information during gameplay.",
      "All extension behavior is limited to supporting GeoDuels gameplay.",
    ],
  },
  {
    title: "4. Data Sharing",
    body: [
      "GeoDuels Enhancer does not sell or transfer extension-collected user data to third parties.",
      "GeoDuels Enhancer does not use or transfer user data for purposes unrelated to its single purpose, and does not use user data to determine creditworthiness or for lending purposes.",
    ],
  },
  {
    title: "5. Third-Party Services",
    body: [
      "GeoDuels uses Google Maps and Street View for gameplay. Google services may process information according to Google's own terms and privacy policies.",
      "GeoDuels Enhancer does not control Google's services and does not add analytics, advertising, or third-party tracking to Street View.",
    ],
  },
  {
    title: "6. Changes",
    body: [
      "This Extension Privacy Policy may be updated from time to time. The updated version will be posted on this page with a new “Last updated” date.",
    ],
  },
  {
    title: "7. Contact",
    body: ["Privacy questions can be sent to me@sourceloc.net."],
  },
];

export default function ExtensionPrivacyPage() {
  return (
    <LegalDocumentPage
      title="GeoDuels Enhancer Privacy Policy"
      updatedAt="June 24, 2026"
      description="This policy describes how the official GeoDuels Enhancer browser extension handles information."
      sections={sections}
    />
  );
}
