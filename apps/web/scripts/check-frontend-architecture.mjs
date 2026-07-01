import fs from "node:fs";
import path from "node:path";

const root = process.cwd();
const strict = process.argv.includes("--strict");
const scanRoots = ["components", "features", "pages"].map((item) => path.join(root, item));
const sourceExtensions = new Set([".ts", ".tsx"]);

const visualRecipePatterns = [
  /bg-\[#/g,
  /rounded-\[/g,
  /shadow-\[/g,
  /backdrop-blur/g,
  /\bglass-panel\b/g,
  /bg-\[linear-gradient/g,
];

const visualRecipeAllowed = [
  "components/ui/Surface.tsx",
  "components/ui/PageShell.tsx",
  "components/ui/AppModalShell.tsx",
  "components/ui/button.tsx",
  "components/ui/input.tsx",
  "components/ui/select.tsx",
  "components/ui/textarea.tsx",
  "components/ui/EndMatchOverlay.tsx",
  "components/ui/RoundResultOverlay.tsx",
  "components/ui/MinimapPanel.tsx",
  "components/ui/GameHUD.tsx",
  "components/ui/MatchSideHPCard.tsx",
  "components/ui/ParticipantIdentity.tsx",
  "components/ui/PlayerBadge.tsx",
  "components/ui/AvatarBadge.tsx",
  "components/ui/DuelOverlayBackground.tsx",
  "components/ui/OverlayDiagonalBackground.tsx",
  "components/ui/QueueCard.tsx",
  "components/ui/TopHeader.tsx",
  "components/ui/PrematchVersusOverlay.tsx",
  "components/ui/MatchSideProfile.tsx",
  "components/ui/ChatPanel.tsx",
  "components/ui/StatsPanel.tsx",
  "components/home/InGameScene.tsx",
  "components/home/RequiredNicknameModal.tsx",
  "features/home/page/HomePageGame.tsx",
  "features/home/page/HomePageOverlays.tsx",
  "features/app-shell/components/AppShell.tsx",
  "pages/match/[id].tsx",
  "features/lobby/components/lobby-primitives.tsx",
  "features/lobby/components/LobbyTutorialSection.tsx",
  "features/lobby/lib/lobby-ui.ts",
];

const visualDebtBudgets = new Map([
  ["components/ui/LobbyScreen.tsx", 0],
  ["features/lobby/components/LeaderboardPanel.tsx", 3],
  ["features/lobby/components/LobbyAuthButtons.tsx", 1],
  ["features/lobby/components/LobbyHeader.tsx", 3],
  ["features/lobby/components/LobbyShellPieces.tsx", 9],
  ["features/lobby/components/MaintenanceNotice.tsx", 2],
  ["features/lobby/components/maps/MapPanels.tsx", 26],
  ["features/lobby/components/PlayPanel.tsx", 7],
  ["features/lobby/components/PartyPanel.tsx", 18],
  ["features/lobby/components/MapUploadForm.tsx", 1],
]);

const lineBudgets = new Map([
  ["components/ui/LobbyScreen.tsx", 900],
  ["features/admin/AdminDashboard.tsx", 1500],
  ["features/home/model/useHomeModel.ts", 1300],
  ["components/ui/EndMatchOverlay.tsx", 650],
  ["components/ui/RoundResultOverlay.tsx", 550],
  ["pages/match/[id].tsx", 350],
  ["features/admin/lib/admin-client.ts", 650],
  ["features/auth/lib/auth-client.ts", 360],
  ["features/matchmaking/lib/queue-client.ts", 450],
  ["features/lobby/components/PlayPanel.tsx", 300],
  ["features/lobby/components/PartyPanel.tsx", 550],
  ["features/lobby/components/LobbyAuthButtons.tsx", 125],
  ["features/lobby/components/LobbyHeader.tsx", 175],
  ["features/lobby/components/LobbyShellPieces.tsx", 230],
  ["features/lobby/components/MaintenanceNotice.tsx", 90],
  ["features/lobby/components/LeaderboardPanel.tsx", 120],
  ["features/lobby/components/maps/MapPanels.tsx", 760],
  ["features/lobby/components/MapUploadForm.tsx", 180],
  ["features/lobby/components/lobby-primitives.tsx", 350],
]);

const defaultBudgets = [
  { pattern: /^components\/ui\/.*\.tsx$/, max: 350 },
  { pattern: /^features\/.+\/components\/.*\.tsx$/, max: 500 },
  { pattern: /^features\/.+\/hooks\/.*\.ts$/, max: 250 },
  { pattern: /^features\/.+\/lib\/.*\.ts$/, max: 300 },
  { pattern: /^pages\/.*\.tsx$/, max: 250 },
];

const findings = [];

for (const file of walkFiles(scanRoots)) {
  const rel = toRel(file);
  if (rel.includes("/__tests__/") || rel.endsWith(".test.ts") || rel.endsWith(".test.tsx")) {
    continue;
  }
  const text = fs.readFileSync(file, "utf8");
  const lines = text.split(/\r?\n/).length;
  const max = lineBudgets.get(rel) ?? defaultBudgets.find((item) => item.pattern.test(rel))?.max;

  if (max && lines > max) {
    findings.push(`${rel}: ${lines} lines exceeds budget ${max}`);
  }

  const visualDebt = countVisualDebt(text);
  const debtBudget = visualDebtBudgets.get(rel);
  if (typeof debtBudget === "number") {
    if (visualDebt > debtBudget) {
      findings.push(`${rel}: visual recipe debt ${visualDebt} exceeds budget ${debtBudget}`);
    }
  } else if (!visualRecipeAllowed.includes(rel)) {
    if (visualDebt > 0) {
      for (const pattern of visualRecipePatterns) {
        if (pattern.test(text)) {
          findings.push(`${rel}: contains banned visual recipe ${pattern}`);
        }
        pattern.lastIndex = 0;
      }
    }
  }

  if (/features\/.+\/components\/.*\.tsx$/.test(rel) && text.includes('from "../../../components/ui/LobbyScreen"')) {
    findings.push(`${rel}: feature components must not import LobbyScreen`);
  }
}

if (findings.length > 0) {
  console.log(`Frontend architecture report: ${findings.length} finding${findings.length === 1 ? "" : "s"}.`);
  for (const finding of findings) console.log(`- ${finding}`);
  if (strict) process.exit(1);
} else {
  console.log("Frontend architecture report: no findings.");
}

function* walkFiles(roots) {
  for (const current of roots) {
    if (!fs.existsSync(current)) continue;
    const stat = fs.statSync(current);
    if (stat.isDirectory()) {
      for (const entry of fs.readdirSync(current)) {
        yield* walkFiles([path.join(current, entry)]);
      }
    } else if (sourceExtensions.has(path.extname(current))) {
      yield current;
    }
  }
}

function toRel(file) {
  return path.relative(root, file).split(path.sep).join("/");
}

function countVisualDebt(text) {
  let count = 0;
  for (const pattern of visualRecipePatterns) {
    count += (text.match(pattern) || []).length;
    pattern.lastIndex = 0;
  }
  return count;
}
