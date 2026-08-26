import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";

const scriptRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const root = fs.existsSync(path.join(process.cwd(), "components")) ? process.cwd() : scriptRoot;
const strict = process.argv.includes("--strict");
const scanRoots = ["components", "features", "pages"].map((item) => path.join(root, item));
const sourceExtensions = new Set([".ts", ".tsx"]);
const require = createRequire(import.meta.url);
const theme = require(path.join(root, "tailwind.config.js")).theme;

// These are deliberately semantic names. Adding a name is a design-system
// change, not a local styling decision.
const approved = {
  colors: new Set(["transparent", "current", "inherit", "surface-page", "surface-panel", "surface-raised", "surface-grouped", "surface-fill", "surface-inset", "surface-overlay", "content-primary", "content-secondary", "content-inverse", "content-on-action", "content-on-danger", "border-default", "border-strong", "border-focus", "action-primary", "action-primary-hover", "action-secondary", "action-danger", "status-success", "status-warning", "status-danger", "status-info", "scrim", "hud-surface", "hud-border", "brand-blue", "brand-blue-hover", "brand-pink", "brand-pink-soft", "brand-orange"]),
  fontSize: new Set(["body", "body-sm", "label", "caption", "heading-sm", "heading-md", "heading-lg", "display-md", "display-lg", "hud-label", "hud-value", "hud-countdown", "hud-countdown-lg"]),
  fontFamily: new Set(["body", "display", "hud", "mono"]),
  fontWeight: new Set(Object.keys(theme.fontWeight)),
  radius: new Set(Object.keys(theme.borderRadius)),
  shadow: new Set(Object.keys(theme.boxShadow)),
  dropShadow: new Set(Object.keys(theme.dropShadow)),
  opacity: new Set(Object.keys(theme.opacity)),
  z: new Set(Object.keys(theme.zIndex)),
  tracking: new Set(Object.keys(theme.letterSpacing)),
  leading: new Set(Object.keys(theme.lineHeight)),
  duration: new Set(Object.keys(theme.transitionDuration)),
  ease: new Set(Object.keys(theme.transitionTimingFunction)),
};
approved.colors = new Set(Object.keys(theme.colors));
approved.fontSize = new Set(Object.keys(theme.fontSize));
approved.fontFamily = new Set(Object.keys(theme.fontFamily));

const rawPalette = /^(?:bg|text|border|from|via|to|ring|outline|decoration|accent|caret|fill|stroke|placeholder|divide)-(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-(?:\d+|\d+\/\d+)$/;
const rawBlackWhite = /^(?:bg|text|border|from|via|to|ring|outline|decoration|accent|caret|fill|stroke|placeholder|divide)-(?:black|white)(?:\/\d+)?$/;
const forbiddenLegacy = /^(?:bg|text|border|from|via|to|ring|outline|decoration|accent|caret|fill|stroke|placeholder|divide)-(?:canvas|panel|raised|grouped|fill|inset|overlay|content|muted|line|lineStrong|positive|warning|negative|info|accentPrimary|accentDanger|accentBlue|accentPink|accentOrange|surface|surfaceElevated|ink|inkMuted|hudBg|hudBorder)(?:\/\d+)?$/;
const forbiddenTypography = /^(?:text)-(?:xs|sm|md|lg|xl|2xl|3xl|4xl|5xl|6xl|7xl|8xl|9xl)$/;
const forbiddenWeight = /^font-(?:thin|extralight|light|normal|black|bold)$/;
const forbiddenRadius = /^rounded-(?:none|3xl|radius(?:-.+)?)$/;
const forbiddenShadow = /^shadow-(?:sm|md|lg|xl|2xl)$/;
const forbiddenArbitraryDesign = /^(?:bg|text|border|from|via|to|ring|outline|decoration|accent|caret|fill|stroke|placeholder|divide|rounded|shadow|opacity|tracking|leading|duration|ease|z)-\[/;
const arbitraryUtility = /^-?[a-z][a-z0-9-]*-\[|^\[(?:mask-image|-webkit-mask-image):/;
const colorNamespaces = ["placeholder", "decoration", "background", "border", "outline", "divide", "accent", "caret", "from", "via", "to", "ring", "fill", "stroke", "text", "bg"];

// Exact file+token exceptions for layout geometry and vendor/artwork. A new
// arbitrary value in an existing file fails until this table is reviewed.
const geometryExceptions = new Map(Object.entries({
  "components/home/InGameScene.tsx": ["top-[-75px]", "h-[calc(100%+75px)]", "w-[min(calc(100vw-1.5rem),19rem)]", "w-[19rem]", "w-[min(calc(100vw-2rem),24rem)]", "max-h-[35vh]", "scale-[1.01]", "scale-[0.98]"],
  "components/ui/AppModalShell.tsx": ["max-h-[85vh]"],
  "components/ui/Popover.tsx": ["max-w-[min(24rem,calc(100vw-1rem))]"],
  "components/ui/PageShell.tsx": ["w-[110px]"],
  "components/ui/DiscreteSlider.tsx": ["mt-[5px]", "-mt-[5px]"],
  "components/ui/compositions.tsx": ["scale-[0.995]", "scale-[0.98]"],
  "components/ui/button.tsx": ["scale-[0.98]"],
  "features/admin/AdminDashboard.tsx": ["min-w-[840px]", "min-w-[760px]", "min-w-[900px]", "grid-cols-[280px_minmax(0,1fr)]", "grid-cols-[180px_1fr]", "grid-cols-[minmax(0,1fr)_minmax(0,0.8fr)]", "grid-cols-[1fr_1fr_auto]", "grid-cols-[1fr_180px_1fr_auto]"],
  "features/admin/components/SignalsRoute.tsx": ["min-w-[960px]"],
  "features/app-shell/components/AppContentRail.tsx": ["max-w-[520px]", "max-w-[1120px]", "max-w-[1220px]"],
  "features/app-shell/components/AppActivityBanner.tsx": ["max-w-[1220px]"],
  "features/app-shell/components/AppShell.tsx": ["h-[68px]", "h-[82px]", "h-[90px]", "w-[112px]", "w-[140px]", "max-w-[7.5rem]", "h-[42px]", "w-[42px]", "bottom-[max(0.75rem,env(safe-area-inset-bottom))]", "max-w-[430px]", "max-w-[520px]", "max-w-[620px]", "min-h-[52px]", "grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)]"],
  "features/app-shell/components/AppNavTasks.tsx": ["min-h-[52px]"],
  "features/app-shell/components/TopHeader.tsx": ["min-w-[220px]", "min-w-[280px]"],
  "features/browser-extension/components/StreetViewEnhancements.tsx": ["w-[260px]"],
  "features/chat/components/ChatPanel.tsx": ["w-[min(calc(100vw-1.5rem),21rem)]", "w-[5.5rem]", "max-w-[min(calc(100vw-1.5rem),19rem)]"],
  "features/game/components/MultiplierBadge.tsx": ["h-[54px]", "w-[58px]", "h-[60px]", "w-[66px]", "h-[20px]", "w-[40px]"],
  "features/game/components/overlays/EndMatchOverlay.tsx": ["min-w-[210px]", "min-w-[104px]", "max-h-[50vh]", "max-w-[320px]", "transition-[width]"],
  "features/game/components/overlays/GameHUD.tsx": ["top-[91px]"],
  "features/game/components/overlays/IntroCountdownText.tsx": ["w-[150px]", "w-[200px]", "h-[150px]", "h-[200px]"],
  "features/game/components/overlays/MatchSideHPCard.tsx": ["-skew-x-[25deg]", "skew-x-[25deg]", "w-[min(380px,calc(50vw-1.25rem))]", "w-[min(380px,calc(50vw-8.5rem))]", "w-[54px]", "h-[54px]", "h-[28px]", "p-[1px]", "pl-[50px]", "pr-[50px]", "transition-[width]"],
  "features/game/components/overlays/MatchSideProfile.tsx": ["border-[6px]"],
  "features/game/components/overlays/MinimapPanel.tsx": ["transition-[width,height]", "transition-[height,opacity,box-shadow]", "w-[min(34vw,460px)]", "h-[min(33vh,360px)]", "w-[min(90vw,800px)]", "h-[min(52vh,560px)]", "h-[50vh]", "min-h-[280px]", "h-[55vh]", "min-h-[320px]", "h-[22vh]", "min-h-[150px]", "h-[27vh]", "min-h-[190px]"],
  "features/game/components/overlays/ResultDistanceBar.tsx": ["w-[28px]", "h-[28px]", "w-[40px]", "h-[40px]", "w-[48px]", "h-[48px]", "h-[56px]", "w-[140px]", "w-[170px]", "w-[280px]", "w-[340px]", "border-[4px]", "inset-[3px]", "border-[2.5px]", "border-[3px]", "w-[14px]", "pr-[30px]", "pl-[30px]", "mt-[3px]", "mt-[2px]", "-mt-[3px]", "-mt-[2px]"],
  "features/game/components/overlays/RoundResultOverlay.tsx": ["top-[148px]", "top-[176px]", "border-b-[4px]"],
  "features/home/page/HomePageChatDock.tsx": ["w-[min(calc(100vw-1.5rem),21rem)]"],
  "features/home/page/GuestVerificationOverlay.tsx": ["min-h-[70px]"],
  "features/lobby/components/LeaderboardPanel.tsx": ["max-w-[980px]", "min-w-[240px]", "min-w-[48px]", "grid-cols-[72px_minmax(0,1fr)_90px]", "grid-cols-[72px_minmax(0,1fr)_110px_110px]"],
  "features/lobby/components/LobbyScreenView.tsx": ["min-h-[calc(100svh-21.25rem)]", "min-h-[18rem]"],
  "features/lobby/components/LobbyShellPieces.tsx": ["max-h-[8rem]", "max-w-[1160px]", "min-h-[112px]", "grid-cols-[minmax(0,1fr)_300px]", "[mask-image:linear-gradient(180deg,black_48%,rgba(0,0,0,0.76)_70%,transparent_100%)]", "[-webkit-mask-image:linear-gradient(180deg,black_48%,rgba(0,0,0,0.76)_70%,transparent_100%)]"],
  "features/lobby/components/MaintenanceNotice.tsx": ["max-w-[560px]", "max-w-[42ch]"],
  "features/lobby/components/MapThumbnailPickerModal.tsx": ["max-w-[980px]", "grid-cols-[minmax(0,1fr)_260px]", "max-h-[58vh]", "aspect-[16/9]"],
  "features/lobby/components/MapMetadataFields.tsx": ["grid-cols-[minmax(0,1fr)_300px]", "aspect-[16/9]"],
  "features/lobby/components/MapUploadLimitsModal.tsx": ["grid-cols-[120px_1fr]"],
  "features/lobby/components/maps/MapPanels.tsx": ["w-[260px]", "aspect-[16/9]", "grid-cols-[minmax(0,1fr)_auto]", "min-h-[min(640px,calc(100dvh-11rem))]", "grid-cols-[220px_minmax(0,1fr)]", "grid-cols-[minmax(0,1.25fr)_minmax(320px,0.75fr)]", "min-h-[280px]", "max-w-[720px]", "grid-cols-[64px_minmax(0,1fr)]", "min-h-[46px]"],
  "features/lobby/components/maps/MapRouteSurfaces.tsx": ["max-w-[1120px]"],
  "features/lobby/components/PartyPanel.tsx": ["max-w-[1180px]", "max-w-[900px]", "max-w-[1040px]", "min-h-[76px]", "grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)]", "max-w-[600px]"],
  "features/lobby/components/PlayLaunchModal.tsx": ["scale-[0.99]"],
  "features/lobby/components/PlayPanel.tsx": ["min-h-[54px]", "scale-[1.01]", "scale-[0.98]", "max-w-[860px]", "min-h-[165px]", "min-h-[180px]"],
  "features/lobby/components/maps/MapEditMetadataModal.tsx": ["max-w-[900px]"],
  "features/lobby/components/maps/MapPickerModal.tsx": ["max-w-[1040px]", "grid-cols-[190px_minmax(0,1fr)]", "max-h-[56vh]"],
  "features/lobby/components/modals/InviteModal.tsx": ["min-h-[46px]"],
  "features/matchmaking/components/QueueCard.tsx": ["max-w-[540px]", "bg-[url('/mountains.v1.svg')]"],
  "features/notifications/components/NotificationCenter.tsx": ["w-[min(92vw,25rem)]", "max-h-[70vh]"],
  "features/players/components/PlayerProfilePage.tsx": ["min-h-[520px]"],
  "features/players/components/PlayerProfilePrimitives.tsx": ["grid-cols-[minmax(0,1fr)_auto]", "grid-cols-[minmax(180px,0.85fr)_minmax(160px,1fr)_120px_24px]", "min-w-[90px]"],
  "features/players/components/IconMetric.tsx": ["grid-cols-[42px_minmax(0,1fr)]", "grid-cols-[48px_minmax(0,1fr)]"],
  "pages/changelog/index.tsx": ["[mask-image:linear-gradient(180deg,black_62%,transparent_100%)]", "[-webkit-mask-image:linear-gradient(180deg,black_62%,transparent_100%)]"],
  "features/social/components/FriendsDashboard.tsx": ["max-w-[720px]", "max-w-[820px]"]
}));

// Native controls belong only in the exact generic primitive that owns their
// semantic and visual contract. Product and page code have no exceptions.
const nativePrimitiveKinds = new Map(Object.entries({
  "components/ui/DiscreteSlider.tsx": new Set(["button", "input"]),
  "components/ui/DropdownMenu.tsx": new Set(["button"]),
  "components/ui/FileInputTrigger.tsx": new Set(["input"]),
  "components/ui/Switch.tsx": new Set(["button", "input"]),
  "components/ui/Tabs.tsx": new Set(["button"]),
  "components/ui/button.tsx": new Set(["button"]),
  "components/ui/compositions.tsx": new Set(["button"]),
  "components/ui/input.tsx": new Set(["input"]),
  "components/ui/select.tsx": new Set(["select"]),
  "components/ui/textarea.tsx": new Set(["textarea"]),
}));
// Interaction exceptions are element-scoped.  A generic primitive may own a
// small amount of event plumbing, but adding a second ad-hoc interaction to
// that file must still fail the architecture check.
const genericInteractionExceptions = new Map(Object.entries({
  "components/ui/AppModalShell.tsx": new Set(["modal-backdrop", "modal-panel"]),
}));
// These structural primitives intentionally forward HTML attributes. This is
// not permission to add direct click/keyboard handling to the implementation.
const genericInteractionSpreadFiles = new Set([
  "components/ui/Badge.tsx",
  "components/ui/ScrollArea.tsx",
  "components/ui/Separator.tsx",
  "components/ui/Spinner.tsx",
  "components/ui/Table.tsx",
  "components/ui/Toolbar.tsx",
  "components/ui/compositions.tsx",
  "components/ui/patterns.tsx",
]);
const interactionAttribute = /\b(?:onClick|onDoubleClick|onKeyDown|onKeyUp|onKeyPress|onMouseDown|onMouseUp|onTouchStart|onTouchEnd|onContextMenu|onPointer[A-Z][A-Za-z]*)\s*[:=]/i;
const safeInteractionSpreads = new Set(["getFloatingProps", "panelMotion", "tabPanelMotion", "sharedStyle"]);

const lineBudgets = new Map([
  ["components/GuessMap.tsx", 700], ["components/home/InGameScene.tsx", 700], ["features/admin/AdminDashboard.tsx", 1300], ["features/admin/lib/admin-client.ts", 700], ["features/auth/controllers/session-controller.ts", 800], ["features/game/controllers/game-controller.ts", 800], ["features/home/model/derive-home-model.ts", 700], ["features/home/model/useHomeModel.ts", 1300], ["features/lobby/controllers/party-controller.ts", 700], ["features/lobby/components/maps/MapPanels.tsx", 760], ["features/game/components/overlays/EndMatchOverlay.tsx", 650], ["features/game/components/overlays/RoundResultOverlay.tsx", 550]
]);

if (process.argv.includes("--self-test")) {
  runSelfTest();
  process.exit(0);
}

const findings = [];
for (const file of walkFiles(scanRoots)) {
  const rel = toRel(file);
  if (rel.includes("/__tests__/") || rel.endsWith(".test.ts") || rel.endsWith(".test.tsx")) continue;
  const text = fs.readFileSync(file, "utf8");
  const lines = text.split(/\r?\n/).length;
  const budget = lineBudgets.get(rel) ?? (rel.startsWith("components/ui/") ? 500 : rel.includes("/components/") || rel.startsWith("pages/") ? 700 : undefined);
  if (budget && lines > budget) findings.push(`${rel}: ${lines} lines exceeds budget ${budget}`);

  for (const match of text.matchAll(/\b(?:[a-z]+:)*!?-?[a-z][a-z0-9]*(?:-[a-z0-9]+)*(?:\/\d+)?(?:-\[[^\]]+\])?/gi)) {
    const token = match[0].replace(/^!/, "");
    const base = token.split(":").at(-1);
    const namespace = base.split("-")[0];
    // Match only utility tokens. Plain prose/identifiers such as "opacity"
    // and "duration" are not Tailwind classes.
    if (!base.includes("-")) continue;
    findings.push(...validateToken(rel, base, token));
  }

  if (/#(?:[0-9a-f]{3,8})\b/i.test(text)) findings.push(`${rel}: color literal; use a named token`);
  if (/\b(?:rgb|rgba|hsl|hsla)\((?!var\(--gd-)/i.test(text) && !rel.endsWith("health-bar.ts") && !rel.endsWith("LobbyShellPieces.tsx")) findings.push(`${rel}: raw color function; use a named token`);
  if (/\b(?:material|level)=/.test(text) && !rel.startsWith("components/ui/")) findings.push(`${rel}: feature code selects material/elevation directly`);
  if (!rel.startsWith("components/ui/") && /\b(?:buttonClassName|surfaceClassName)\s*\(|<Surface\b|import\s*\{[^}]*\b(?:buttonClassName|surfaceClassName|Surface)\b[^}]*\}\s*from/m.test(text)) {
    findings.push(`${rel}: feature/page code may not recreate shared control/surface primitives with low-level helpers`);
  }
  if (!rel.startsWith("components/ui/") && /\btranslucent-surface(?:-interactive)?\b/.test(text)) {
    findings.push(`${rel}: feature/page code may not apply the low-level translucent surface recipe directly`);
  }
  if (/\b(?:LobbyInset|LobbyMutedBox)\b/.test(text)) findings.push(`${rel}: obsolete ambiguous component name`);
  if (rel.startsWith("components/ui/") && /from ["'`][^"'`]*features\//.test(text)) findings.push(`${rel}: generic UI may not import feature code`);
  if (rel.startsWith("components/ui/") && /from ["'`][^"'`]*components\/(?:home|game|lobby|players|chat|admin)\//.test(text)) findings.push(`${rel}: generic UI may not import product components`);
  findings.push(...validateNativeControls(rel, text));
}

if (findings.length) {
  console.log(`Frontend architecture report: ${findings.length} finding${findings.length === 1 ? "" : "s"}.`);
  for (const finding of findings) console.log(`- ${finding}`);
  if (strict) process.exit(1);
} else console.log("Frontend architecture report: no findings.");

function isAllowedGeometry(rel, token) {
  const base = token.replace(/^!?/, "");
  return geometryExceptions.get(rel)?.includes(base) ?? false;
}

function validateToken(rel, base, token) {
  if (!base.includes("-")) return [];
  const errors = [];
  const namespace = base.split("-")[0];
  if (isColorUtility(base)) {
    const color = colorName(base);
    if (rawPalette.test(base) || rawBlackWhite.test(base) || forbiddenLegacy.test(base) || !approved.colors.has(color)) errors.push("unapproved color utility");
  }
  if (forbiddenTypography.test(base) || forbiddenWeight.test(base) || forbiddenRadius.test(base) || forbiddenShadow.test(base)) errors.push("legacy design utility");
  if (forbiddenArbitraryDesign.test(base) && !isAllowedGeometry(rel, base)) errors.push("arbitrary design value");
  if (arbitraryUtility.test(base) && !isAllowedGeometry(rel, base)) errors.push("arbitrary value outside approved geometry exception");
  if (namespace === "rounded" && !approved.radius.has(radiusName(base))) errors.push("unknown radius token");
  if (namespace === "shadow" && !approved.shadow.has(base.slice(7))) errors.push("unknown shadow token");
  if (base.startsWith("drop-shadow-") && !approved.dropShadow.has(base.slice("drop-shadow-".length))) errors.push("unknown drop-shadow token");
  if (namespace === "opacity" && !approved.opacity.has(base.slice(8))) errors.push("unknown opacity token");
  if (namespace === "z" && !approved.z.has(base.slice(2))) errors.push("unknown z-layer token");
  for (const family of ["tracking", "leading", "duration", "ease"]) if (namespace === family && !approved[family].has(base.slice(family.length + 1))) errors.push(`unknown ${family} token`);
  if (namespace === "font" && !approved.fontFamily.has(base.slice(5)) && !approved.fontWeight.has(base.slice(5))) errors.push("unknown font token");
  return errors.map((error) => `${rel}: ${error} ${token}`);
}

function validateNativeControls(rel, text) {
  const errors = [];
  const allowedNativeKinds = nativePrimitiveKinds.get(rel) ?? new Set();
  const interactionExceptions = genericInteractionExceptions.get(rel) ?? new Set();
  const exceptionUseCount = new Map();
  const aliases = findIntrinsicAliases(text);
  const openingTags = findOpeningTags(text);

  const getMarker = (attrs) => {
    const matches = [...attrs.matchAll(/\bdata-architecture-exception\s*=\s*(["'])(.*?)\1/gi)];
    return { value: matches[0]?.[2], count: matches.length };
  };
  const hasType = (attrs) => {
    const value = getAttribute(attrs, "type");
    return (value !== null && !isEmptyAttributeValue(value)) || /(?:^|[,\s])type\s*:/i.test(attrs);
  };
  const markerFor = (tag) => getMarker(tag.attrs);

  // Every marker is checked independently of the element-specific loops. This
  // prevents moving a valid marker to a different control or duplicating it.
  for (const tag of openingTags) {
    const { value: marker, count } = markerFor(tag);
    if (!marker) continue;
    if (count !== 1) errors.push(`${rel}: architecture exception marker must appear exactly once on an element`);
    exceptionUseCount.set(marker, (exceptionUseCount.get(marker) ?? 0) + 1);
    const lower = tag.name.toLowerCase();
    const tagKind = lower.split(".").at(-1);
    const interactionMarker = (tagKind === "div" || tagKind === "span") && interactionExceptions.has(marker);
    if (!interactionMarker) {
      errors.push(`${rel}: invalid architecture exception marker ${marker}`);
    }
  }
  for (const [marker, count] of exceptionUseCount) {
    if (count > 1) errors.push(`${rel}: architecture exception marker ${marker} may be used only once per file`);
  }
  for (const tag of openingTags) {
    const rawName = tag.name;
    const kind = rawName.toLowerCase();
    const tagKind = kind.split(".").at(-1);
    const intrinsicKind = ["button", "input", "textarea", "select"].includes(tagKind) &&
      (rawName === tagKind || rawName.includes("."))
      ? tagKind
      : aliases.get(rawName) ?? aliases.get(kind);
    if (intrinsicKind) {
      if (!allowedNativeKinds.has(intrinsicKind)) {
        errors.push(`${rel}: raw <${rawName}> is not allowed; use a shared control`);
      }
      if (intrinsicKind === "button" && !hasType(tag.attrs)) {
        errors.push(`${rel}: button is missing an explicit type`);
      }
    }

    if (tagKind === "div" || tagKind === "span") {
      const { value: marker } = markerFor(tag);
      const allowedMarker = marker && interactionExceptions.has(marker) && isAllowedInteractionMarker(marker, tag.attrs);
      const allowedStructuralSpread = genericInteractionSpreadFiles.has(rel) &&
        hasUnsafeSpread(tag.attrs) && !hasExplicitNonSemanticInteraction(tag.attrs);
      if (hasNonSemanticInteraction(tag.attrs) && !allowedMarker && !allowedStructuralSpread) {
        errors.push(`${rel}: clickable <${tagKind}> must be a semantic/shared control`);
      }
      if (marker && interactionExceptions.has(marker) && !hasNonSemanticInteraction(tag.attrs)) {
        errors.push(`${rel}: interaction exception ${marker} must mark its exact interactive element`);
      }
    }

    if (tagKind === "a") {
      const href = getAttribute(tag.attrs, "href");
      if ((!href || isEmptyAttributeValue(href) || /^#(?:$|["'])/.test(href.trim())) && hasNonSemanticInteraction(tag.attrs)) {
        errors.push(`${rel}: anchor without href cannot act as a button; use a shared control`);
      }
      if (/\brole\s*=\s*["']button["']/i.test(tag.attrs)) {
        errors.push(`${rel}: anchor role=button must use a semantic/shared control`);
      }
    }
  }

  for (const call of findCreateElementCalls(text)) {
    const intrinsicKind = ["button", "input", "textarea", "select", "div", "span", "a"].includes(call.kind)
      ? call.kind
      : aliases.get(call.kind) ?? aliases.get(call.kind.toLowerCase());
    if (!intrinsicKind) continue;
    const props = call.props;
    if (["button", "input", "textarea", "select"].includes(intrinsicKind)) {
      if (!allowedNativeKinds.has(intrinsicKind)) errors.push(`${rel}: React.createElement(${call.kind}) is a raw native control; use a shared control`);
      if (intrinsicKind === "button" && !hasType(props)) errors.push(`${rel}: button is missing an explicit type`);
    } else if (hasNonSemanticInteraction(props) || (intrinsicKind === "a" && (!/\bhref\s*:/i.test(props) || /\brole\s*:\s*["']button["']/i.test(props)))) {
      errors.push(`${rel}: React.createElement(${call.kind}) creates a non-semantic interactive element`);
    }
  }

  for (const tag of openingTags) {
    if (tag.name !== "IconButton" && tag.name !== "Button") continue;
    const iconButton = tag.name === "IconButton" || attributeMayBeIcon(tag.attrs);
    if (iconButton && rel !== "components/ui/button.tsx" && !hasAccessibleName(tag.attrs)) {
      errors.push(`${rel}: icon-only ${tag.name} is missing an accessible name (aria-label or aria-labelledby must be non-empty)`);
    }
  }
  return errors;
}

// JSX attributes can contain arrows, nested objects, spreads, and multiline
// expressions.  A small balanced scanner is intentionally used here instead
// of a regex ending at the first `>` (which makes `onClick={() => ...}` an
// easy bypass).  It is not a JSX parser; it only extracts opening elements and
// preserves their source ranges for the policy checks above.
function findOpeningTags(text) {
  const tags = [];
  for (let i = 0; i < text.length; i += 1) {
    if (text[i] !== "<" || !/[A-Za-z]/.test(text[i + 1] ?? "")) continue;
    const nameMatch = text.slice(i + 1).match(/^([A-Za-z][A-Za-z0-9_.]*)\b/);
    if (!nameMatch) continue;
    const end = findTagEnd(text, i + 1 + nameMatch[0].length);
    if (end < 0) continue;
    const name = nameMatch[1];
    const raw = text.slice(i + 1 + name.length, end);
    const selfClosing = /\/\s*$/.test(raw);
    tags.push({ name, attrs: raw.replace(/\/\s*$/, ""), start: i, end: end + 1, selfClosing });
    i = end;
  }
  return tags;
}

function findTagEnd(text, start) {
  let quote = null;
  let braces = 0;
  let escaped = false;
  for (let i = start; i < text.length; i += 1) {
    const char = text[i];
    if (quote) {
      if (escaped) escaped = false;
      else if (char === "\\") escaped = true;
      else if (char === quote) quote = null;
      continue;
    }
    if (char === "\"" || char === "'" || char === "`") { quote = char; continue; }
    if (char === "{") { braces += 1; continue; }
    if (char === "}" && braces > 0) { braces -= 1; continue; }
    if (char === ">" && braces === 0) return i;
  }
  return -1;
}

function findIntrinsicAliases(text) {
  const aliases = new Map();
  const aliasPattern = /\b(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(["'])(button|input|textarea|select|div|span|a)\2/g;
  for (const match of text.matchAll(aliasPattern)) aliases.set(match[1], match[3]);
  return aliases;
}

function findCreateElementCalls(text) {
  const calls = [];
  const callPattern = /(?<![.$\w])(?:React\.)?createElement\s*\(/g;
  for (const match of text.matchAll(callPattern)) {
    const open = (match.index ?? 0) + match[0].length - 1;
    const close = findBalancedEnd(text, open, "(", ")");
    if (close < 0) continue;
    const args = text.slice(open + 1, close);
    const parts = splitTopLevel(args, ",");
    const kind = parts[0]?.trim().replace(/^(["'])(.*?)\1$/, "$2");
    if (kind) calls.push({ kind, props: parts[1] ?? "" });
  }
  return calls;
}

function findBalancedEnd(text, start, openChar, closeChar) {
  let depth = 0;
  let quote = null;
  let escaped = false;
  for (let i = start; i < text.length; i += 1) {
    const char = text[i];
    if (quote) {
      if (escaped) escaped = false;
      else if (char === "\\") escaped = true;
      else if (char === quote) quote = null;
      continue;
    }
    if (char === "\"" || char === "'" || char === "`") { quote = char; continue; }
    if (char === openChar) depth += 1;
    if (char === closeChar) {
      depth -= 1;
      if (depth === 0) return i;
    }
  }
  return -1;
}

function splitTopLevel(text, separator) {
  const parts = [];
  let start = 0;
  let braces = 0;
  let brackets = 0;
  let parens = 0;
  let quote = null;
  let escaped = false;
  for (let i = 0; i < text.length; i += 1) {
    const char = text[i];
    if (quote) {
      if (escaped) escaped = false;
      else if (char === "\\") escaped = true;
      else if (char === quote) quote = null;
      continue;
    }
    if (char === "\"" || char === "'" || char === "`") { quote = char; continue; }
    if (char === "{") braces += 1;
    else if (char === "}") braces -= 1;
    else if (char === "[") brackets += 1;
    else if (char === "]") brackets -= 1;
    else if (char === "(") parens += 1;
    else if (char === ")") parens -= 1;
    else if (char === separator && braces === 0 && brackets === 0 && parens === 0) {
      parts.push(text.slice(start, i));
      start = i + 1;
    }
  }
  parts.push(text.slice(start));
  return parts;
}

function hasNonSemanticInteraction(attrs) {
  if (hasExplicitNonSemanticInteraction(attrs)) return true;
  // A spread can inject any interaction prop and cannot be proven inert by a
  // text-only checker. Generic primitive files are handled by the caller's
  // exact primitive allowlist; product composition code must be explicit.
  if (hasUnsafeSpread(attrs)) return true;
  return false;
}

function hasUnsafeSpread(attrs) {
  const spread = attrs.match(/\{\s*\.\.\.\s*([A-Za-z_$][\w$]*)\b/);
  return Boolean(spread && !safeInteractionSpreads.has(spread[1]));
}

function hasExplicitNonSemanticInteraction(attrs) {
  if (interactionAttribute.test(attrs)) return true;
  if (/\brole\s*[=:]\s*(?:["']button["']|\{\s*["']button["']\s*\})/i.test(attrs)) return true;
  // A tabindex by itself is used for focus management.  It becomes a
  // button-like recipe when paired with keyboard/click handling or a pointer
  // cursor style.
  if (/\btabIndex\s*[=:]/i.test(attrs) && /\b(?:onKey|onClick|onPointer|onMouse|onTouch)\w*\s*[=:]/i.test(attrs)) return true;
  if (/\bclass(?:Name)?\s*=\s*(?:["'][^"']*\bcursor-pointer\b|\{[^}]*\bcursor-pointer\b)/i.test(attrs) && /\b(?:bg-action-|rounded-|px-|py-|cursor-pointer)\b/i.test(attrs)) return true;
  return false;
}

function isAllowedInteractionMarker(marker, attrs) {
  if (marker === "modal-backdrop" || marker === "modal-panel") return interactionAttribute.test(attrs);
  return false;
}

function getAttribute(attrs, name) {
  const pattern = new RegExp(`\\b${name}\\s*=\\s*(?:(["'])(.*?)\\1|\\{([\\s\\S]*?)\\})`, "i");
  const match = attrs.match(pattern);
  return match ? (match[2] ?? match[3] ?? "") : null;
}

function isEmptyAttributeValue(value) {
  return !value || /^(?:undefined|null|["']{2}|\{\s*\})$/i.test(value.trim()) || !value.trim();
}

function hasAccessibleName(attrs) {
  for (const name of ["aria-label", "aria-labelledby"]) {
    const value = getAttribute(attrs, name);
    if (value !== null && !isEmptyAttributeValue(value)) return true;
  }
  return false;
}

function attributeMayBeIcon(attrs) {
  const match = attrs.match(/\bvariant\s*=\s*(?:(["'])(.*?)\1|\{([\s\S]*?)\})/i);
  if (!match) return false;
  if (match[2] !== undefined) return /^icon$/i.test(match[2].trim());
  // Require a name when a dynamic expression can select the icon variant.
  // Expressions that enumerate only another variant family are statically
  // proven non-icon and retain normal text-button semantics.
  return /["']icon["']/i.test(match[3]);
}

function radiusName(base) {
  const suffix = base.slice("rounded-".length);
  if (approved.radius.has(suffix)) return suffix;
  const directional = suffix.match(/^(?:[trblsexy]{1,2})-(.+)$/);
  return directional?.[1] ?? suffix;
}

function isColorUtility(base) {
  if (base.includes("[")) return false;
  const namespace = colorNamespaces.find((item) => base.startsWith(`${item}-`));
  if (!namespace) return false;
  const suffix = base.slice(namespace.length + 1).split("/")[0];
  if (!suffix) return false;
  if (namespace === "bg" && /^(?:gradient-to-|cover$|center$|bottom$|top$|left$|right$|none$|fixed$|local$|scroll$)/.test(suffix)) return false;
  if (namespace === "text" && (approved.fontSize.has(suffix) || /^(?:left|right|center|justify|wrap|ellipsis|balance|pretty|nowrap)$/.test(suffix))) return false;
  if (namespace === "border" && /^(?:0|2|3|4|6|solid|dashed|dotted|double|hidden|none|[trblxyse]{1,2}|[trblxyse]{1,2}-(?:0|2|3|4|6|solid|dashed|dotted|double|hidden|none))$/.test(suffix)) return false;
  if (namespace === "outline" && /^(?:0|1|2|4|8|none|dashed|dotted|double)$/.test(suffix)) return false;
  if (namespace === "ring" && /^(?:0|1|2|4|8|inset|offset(?:-\d+)?)$/.test(suffix)) return false;
  if (namespace === "divide" && /^divide-(?:[xy](?:-reverse|-0|-2|-4|-8)?|0|2|4|8|solid|dashed|dotted|double|none)$/.test(base)) return false;
  if (namespace === "fill" && /^(?:none|current)$/.test(suffix)) return suffix === "current";
  if (namespace === "stroke" && /^(?:0|1|2|current|none)$/.test(suffix)) return suffix === "current";
  return true;
}

function colorName(base) {
  const namespace = colorNamespaces.find((item) => base.startsWith(`${item}-`));
  if (!namespace) return base;
  let suffix = base.slice(namespace.length + 1).split("/")[0];
  if (namespace === "border") suffix = suffix.replace(/^[trblxyse]{1,2}-/, "");
  return suffix;
}

function runSelfTest() {
  const check = (rel, token) => validateToken(rel, token, token).length === 0;
  const native = (rel, source) => validateNativeControls(rel, source);
  const checks = [
    ["raw palette rejected", !check("fixtures.tsx", "bg-red-500")],
    ["unknown semantic colors rejected", colorNamespaces.every((namespace) => !check("fixtures.tsx", `${namespace}-brand-mauve`))],
    ["invalid directional radius rejected", !check("fixtures.tsx", "rounded-l-anything")],
    ["new arbitrary value in excepted file rejected", !check("components/home/InGameScene.tsx", "w-[999px]")],
    ["invalid font class rejected", !check("fixtures.tsx", "font-black")],
    ["invalid drop-shadow class rejected", !check("fixtures.tsx", "drop-shadow-xl")],
    ["exact geometry exception accepted", check("components/home/InGameScene.tsx", "w-[min(calc(100vw-1.5rem),19rem)]")],
    ["HUD countdown role accepted", check("features/game/components/overlays/IntroCountdownText.tsx", "text-hud-countdown")],
    ["raw feature button rejected", native("features/example/Control.tsx", `<button type="button">Save</button>`).some((item) => item.includes("raw <button>"))],
    ["visible raw input rejected", native("features/example/Control.tsx", `<input type="text" />`).some((item) => item.includes("raw <input>"))],
    ["product hidden-file input rejected", native("features/lobby/components/MapUploadForm.tsx", `<label>Choose<input type="file" className="hidden" /></label>`).some((item) => item.includes("raw <input>"))],
    ["file-input primitive accepted", native("components/ui/FileInputTrigger.tsx", `<label>Choose<input type="file" className="hidden" /></label>`).length === 0],
    ["clickable div rejected", native("features/example/Control.tsx", `<div onClick={save}>Save</div>`).some((item) => item.includes("clickable <div>"))],
    ["multiline JSX arrow and fragment rejected", native("features/example/Control.tsx", `<><button\n onClick={() => {\n save();\n }}\n type="button"\n /></>`).some((item) => item.includes("raw <button>"))],
    ["spread interaction rejected", native("features/example/Control.tsx", `<div\n {...props}\n className="rounded-md"\n />`).some((item) => item.includes("clickable <div>"))],
    ["non-click pointer and touch handlers rejected", native("features/example/Control.tsx", `<span onMouseDown={start} onTouchEnd={end} onPointerMove={move} onContextMenu={menu} />`).some((item) => item.includes("clickable <span>"))],
    ["role and keyboard recipe rejected", native("features/example/Control.tsx", `<div role={"button"} tabIndex={0} onKeyDown={onKeyDown} />`).some((item) => item.includes("clickable <div>"))],
    ["button-like span class rejected", native("features/example/Control.tsx", `<span className="cursor-pointer rounded-md bg-action-primary px-3 py-2" />`).some((item) => item.includes("clickable <span>"))],
    ["anchor without href rejected", native("features/example/Control.tsx", `<a onClick={save}>Save</a>`).some((item) => item.includes("anchor without href"))],
    ["hash anchor button recipe rejected", native("features/example/Control.tsx", `<a href="#" onClick={save}>Save</a>`).some((item) => item.includes("anchor without href"))],
    ["static intrinsic JSX alias rejected", native("features/example/Control.tsx", `const NativeButton = "button"; return <NativeButton type="button" />;`).some((item) => item.includes("raw <NativeButton>"))],
    ["React createElement native rejected", native("features/example/Control.tsx", `React.createElement("button", { type: "button", onClick: save })`).some((item) => item.includes("React.createElement(button)"))],
    ["React createElement pointer div rejected", native("features/example/Control.tsx", `createElement("div", { onPointerDown: start, ...props })`).some((item) => item.includes("React.createElement(div)"))],
    ["React createElement alias rejected", native("features/example/Control.tsx", `const Tag = "span"; React.createElement(Tag, { onMouseUp: end })`).some((item) => item.includes("React.createElement(Tag)"))],
    ["icon empty aria-label rejected", native("features/example/Control.tsx", `<IconButton aria-label={""} />`).some((item) => item.includes("accessible name"))],
    ["icon expression aria-label accepted", native("features/example/Control.tsx", `<IconButton aria-label={label} />`).length === 0],
    ["dynamic icon variant requires name", native("features/example/Control.tsx", `<Button variant={compact ? "icon" : "secondary"} />`).some((item) => item.includes("accessible name"))],
    ["native generic button accepted", native("components/ui/button.tsx", `<button type={type}>Save</button>`).length === 0],
    ["missing button type rejected", native("components/ui/button.tsx", `<button>Save</button>`).some((item) => item.includes("missing an explicit type"))],
    ["React native generic button accepted", native("components/ui/button.tsx", `React.createElement("button", { type: type })`).length === 0],
    ["interaction exception must be exact", native("components/ui/AppModalShell.tsx", `<div onClick={close}>Backdrop</div>`).some((item) => item.includes("clickable <div>"))],
    ["marked modal interaction accepted", native("components/ui/AppModalShell.tsx", `<motion.div data-architecture-exception="modal-backdrop" onClick={close} />`).length === 0],
    ["new generic UI clickable div rejected", native("components/ui/NewPrimitive.tsx", `<div onClick={save}>Save</div>`).some((item) => item.includes("clickable <div>"))],
    ["wrong native kind in primitive rejected", native("components/ui/input.tsx", `<button type="button">Save</button>`).some((item) => item.includes("raw <button>"))],
    ["icon accessible name required", native("features/example/Control.tsx", `<IconButton><X /></IconButton>`).some((item) => item.includes("accessible name"))],
  ];
  const failures = checks.filter(([, passed]) => !passed);
  if (failures.length) {
    for (const [name] of failures) console.error(`self-test failed: ${name}`);
    process.exit(1);
  }
  console.log(`Frontend architecture self-test: ${checks.length} checks passed.`);
}

function* walkFiles(roots) {
  for (const current of roots) {
    if (!fs.existsSync(current)) continue;
    const stat = fs.statSync(current);
    if (stat.isDirectory()) for (const entry of fs.readdirSync(current)) yield* walkFiles([path.join(current, entry)]);
    else if (sourceExtensions.has(path.extname(current))) yield current;
  }
}
function toRel(file) { return path.relative(root, file).split(path.sep).join("/"); }
