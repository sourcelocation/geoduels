# Frontend Architecture Rules

## Ownership

- New files under `components/ui` should be generic primitives.
- Gameplay-specific components are named compositions with stable visual contracts. New gameplay UI belongs under its feature directory.
- Feature UI belongs under `features/<feature>/components`.
- Feature state used by one panel belongs in that panel or a nearby hook.
- `LobbyScreen` should remain a route/composition shell, not a feature owner.

## Styling

### Semantic design tokens

Use the kebab-case semantic roles in the `surface-*`, `content-*`, `border-*`,
`action-*`, and `status-*` families for ordinary app UI. Their values are CSS
variables in `styles/tokens/themes.css`, with game and operational contexts
selected by `data-ui-theme`. The earlier `canvas`, `panel`, `raised`, `inset`,
`content`, `muted`, `line`, `positive`, `warning`, `negative`, and `info`
names are migration aliases; do not add new usages of them.

The token layers have one-way ownership:

```text
styles/tokens/primitives.css  ->  styles/tokens/themes.css  ->  UI recipes  ->  feature UI  ->  pages/routes
```

Primitives describe raw values (neutral colors, spacing, radii, type weights,
and motion). Semantic theme tokens describe meaning (`surface-page`,
`content-primary`, `action-primary`, `status-danger`). Component recipes own
the final visual contract for controls and patterns. Feature code chooses a
component or semantic role and may compose layout; it must not reach backward
into primitive values or private material implementations.

When color itself is the intended identity rather than a UI state, use the
shared chromatic palette: `accentBlue`, `accentPink`, and `accentOrange` (plus
their named companion tokens where available). For example, a blue product CTA
uses `accentBlue`; an informational notice uses `info`. Add colors to this
palette instead of embedding hex values in feature code.

Feature code never selects a material or elevation. It chooses a component by
purpose; the component library owns its adaptive fill, border, and depth. This
mirrors iOS semantic colors and grouped content: meaning stays stable even when
the underlying appearance changes.

Typography follows the same contract. Use the approved `body`, `body-sm`,
`label`, `caption`, `heading-*`, `display-*`, and `hud-*` roles or a shared
typography component. `hud-countdown` and `hud-countdown-lg` are the dedicated
mobile/desktop roles for the centre gameplay countdown; document display roles
must not substitute for them. The app font variable and base family are owned by the
document root so browser portals inherit them; feature code must not introduce
a new family or arbitrary weight. Portal primitives must also copy the nearest
`data-ui-theme` onto their floating container so semantic colors retain their
theme outside the source DOM subtree.

### Approved exception model

Exceptions are narrow and owned, never a whole feature or directory:

- Component recipes may use exact values needed to preserve a component's
  visual contract, but those values stay in the component recipe. Gameplay HUD
  recipes own their exact bar, badge, and countdown geometry.
- Gameplay geometry, map positioning, animation keyframes, and third-party
  vendor overrides may use precise values in the owning component or a scoped
  file under `styles/vendor`/`styles/effects`.
- A one-off arbitrary Tailwind value requires a documented reason, an owner,
  and a migration/removal issue. It may not be used for a color, font, radius,
  shadow, opacity, or z-index when an approved token exists.
- Legacy aliases are compatibility-only. New code must use the kebab-case
  semantic taxonomy, and each alias must have a tracked removal path.

The strict validator reports the exact file and token violation. It has no
admin, moderator, feature, or whole-file visual exemptions. Geometry and
vendor/artwork values are allowed only through the exact file-scoped exception
table in `scripts/check-frontend-architecture.mjs`.

Inline `className` is for layout and spacing:

- `flex`, `grid`, `gap-*`, `w-*`, `max-w-*`
- responsive layout classes
- small spacing overrides when a component API would be awkward

Inline `className` must not create visual recipes:

- no `bg-[#...]`
- no `rounded-[...]`
- no `shadow-[...]`
- no `backdrop-blur-*`
- no direct material implementation classes in feature or page code
- no repeated `border border-white/10 bg-black/20` style clusters

Add or reuse a primitive/variant instead:

- `AppPanel`
- `SectionCard`
- `ContentUnavailable`
- `LobbyDangerNotice`
- `Button`, `ButtonLink`, `IconButton`
- `AppCardButton`
- `Badge`
- `LobbyNotice`
- `LobbySectionHeader`
- `LobbySegmentedControl`
- `LobbyFieldLabel`
- `LobbyLoadingState`
- `LobbyInput`, `LobbyTextarea`, `LobbySelect`

### Native control boundary

Feature and page composition code must use the shared `Button`, `IconButton`,
`Input`, `Textarea`, `Select`, `Switch`, `Checkbox`, `Dialog`, and related
controls. Raw `button`, `input`, `textarea`, and `select` elements are rejected
by the strict validator. Native elements are allowed in the actual generic
primitive implementation under `components/ui`; they are not allowed in
feature-owned wrappers that merely reproduce a standard control.

Product and page code have no native-control exceptions. File uploads use the
shared `FileInputTrigger`, which owns the hidden native input and its accessible
label contract. Domain controls that need native markup must first be extracted
as an accessible named primitive and added to the exact file-and-element-kind
allowlist in `scripts/check-frontend-architecture.mjs`.

Clickable or keyboard-handled `div` and `span` elements are rejected outside
generic interaction primitives. This includes mouse, touch, pointer, context
menu, role-button, and spread-provided interaction props; use a semantic
shared control instead. Anchors without a real destination must not be used as
buttons. The checker also rejects statically obvious native aliases and
`React.createElement` construction in product code, and feature/page code may
not import low-level `buttonClassName`/`Surface` helpers to recreate a
primitive. Every button must have an explicit type, and icon-only
`Button`/`IconButton` usage must provide a non-empty `aria-label` or
`aria-labelledby`, including expression-valued attributes.

Generic interaction exceptions are element-scoped. `AppModalShell` marks only
its backdrop and panel with `data-architecture-exception="modal-backdrop"` and
`data-architecture-exception="modal-panel"`; adding another clickable element
to that file fails validation. Exception markers are unique per file and may
not be moved to another element or reused to exempt a second control.

## Shared Component Index

Use these existing components before creating new visual recipes. Keep this index short and update it when a component becomes a preferred pattern.

### Generic UI

- `Button`, `ButtonLink`, `IconButton`: a consistent command/navigation family. Icon-only buttons require an accessible label; use `IconButton size="icon-sm"` only for dense inline metadata actions where the surrounding layout supplies the larger target context.
- `Badge`, `CounterBadge`: semantic tags and capped count indicators.
- `Surface`: private material engine for shared UI components. Do not import it from features or pages.
- `AppPanel`, `SectionCard`, `DocumentPanel`: primary app content, quiet grouped content, and operational/document content.
- `SettingsGroup`, `DangerZone`, `ContentUnavailable`: purpose-owned settings, destructive, and unavailable-content compositions.
- `Input`, `Textarea`, `Select`: generic form controls; use `variant="game"` for game/lobby surfaces and default operational styling elsewhere.
- `Field`: label, helper text, and error wrapper for forms.
- `Switch`, `Checkbox`, `DiscreteSlider`, `Kbd`, `Separator`, `ScrollArea`, `Spinner`, `Skeleton`: small accessible control and feedback primitives.
- `Popover`, `DropdownMenu`, `Dialog`, `AlertDialog`: interaction-heavy primitives. Use these instead of manual outside-click listeners or focus handling. Keep transient feedback local to the control or surface that initiated it.
- `HorizontalScroller`: native horizontal scrolling with hidden browser chrome, accessible overflow controls, and scroll snapping for repeated card collections.
- `TableShell`, `Table`, `TableHead`: operational table primitives.
- `Metric`: compact label/value metric for operational dashboards.
- `Tabs`: keyboard-operable compact selector for switching between small view sets; use `appearance="segmented"` for mutually exclusive compact choices.
- `Toolbar`: horizontal action container for grouped controls.
- `Tooltip` and `TooltipProvider`: hover/focus explanation for icon-only or unfamiliar controls.
- `AppModalShell` / `Modal` / `Dialog`: standard focus-managed responsive modal shell. Prefer `AlertDialog` for destructive confirmation. Each shell owns its independent exit lifecycle and registers it with the shared modal-dismissal coordinator. Session-ending operations await all mounted modal exits before publishing the anonymous auth state, so modals animate out before their owning authenticated UI unmounts.
- `AppActivityBanner`: app-shell-owned presentation for background activities such as matchmaking and party membership; routes supply typed activity descriptors instead of embedding activity UI in page panels.
- `PageShell`: generic page wrapper for non-lobby pages.
- `AppShell` / `AppBackground`: shared application frame and backdrop; use `backgroundBlurred` for scene-wide focus treatments.
- `MarkdownContent`: render trusted markdown-style content with app typography.
- `RelativeTime`: render timestamps as relative labels.
- `IconMetric`: icon plus metric text for compact stat displays.
- `Eyebrow`, `SectionTitle`, `CardTitle`, `BodyText`, `MutedText`, `HelperText`: shared typography atoms.
- `SectionHeader`, `PageHeader`, `ActionGroup`: standard content hierarchy and responsive actions.
- `InsetList`, `EntityRow`, `SettingRow`: reusable list and settings compositions. Domain components such as `PlayerRow` and `NotificationItem` should compose these rather than recreate their material.
- `Notice`, `AsyncState`, `EmptyState`: consistent success/warning/error/loading/empty states.

### Player And Identity UI

- `PlayerProfileLink`: use for any player/user name or avatar that should navigate to `/players/[id]`; pass `userId` and `nickname`, and set `disabled` for guests or system users.
- `PlayerNameWithBadge`: player name plus selected badge, already wrapped with `PlayerProfileLink`.
- `PlayerBadge`: render a selected badge; use `badgeTitle` for tooltip/title text.
- `AvatarBadge`: avatar image with fallback initials and size variants.
- `ParticipantAvatar`, `ParticipantName`, `ParticipantIdentityRow`, `ParticipantIdentityCard`: match participant identity blocks for player/team-aware gameplay UI.
- `MmrDisplay`: rating display with trophy icon sizing.

### Lobby Primitives

- `AppPanel`: standard primary app surface.
- `SectionCard`: grouped content within an app panel.
- `AppCardButton`: large clickable app card.
- `LobbySection`: grouped lobby content within an app panel.
- `LobbyPlaceholder`: unavailable or explanatory lobby content.
- `LobbyDangerNotice`: destructive/error notice for lobby flows.
- `LobbyNotice`: success, warning, danger, or muted feedback notice.
- `LobbyFieldLabel`: uppercase lobby form label.
- `LobbySegmentedControl`: mutually exclusive small option set, such as difficulty or visibility.
- `LobbyLoadingState`: spinner/text loading row.
- `LobbySectionHeader`: section eyebrow, title, and optional description.
- `LobbyInput`, `LobbyTextarea`, `LobbySelect`: lobby/game variants of generic form controls.

### Specialized Product Compositions

- `ChatPanel`, `MinimapPanel`, `GameHUD`, `GameStartOverlay`, `RoundResultOverlay`, `EndMatchOverlay`, `PrematchVersusOverlay`, `MatchSideHPCard`, `MatchSideProfile`, `TopHeader`, and `LobbyScreen` are gameplay/lobby-specific compositions. Reuse only when extending the same surface; new gameplay UI belongs under its feature directory.

## Operational UI

Admin, legal, and document surfaces should use operational primitives:

- `DocumentPanel`
- `Button`
- `Input`
- `Select`
- `Textarea`
- `Table`
- `Metric`
- `Badge`

Admin should not use decorative translucent surfaces, hero-style sections, or ad hoc gradients.

## Dependency direction

The frontend dependency graph is deliberately acyclic:

```text
tokens -> primitives -> patterns -> feature components -> pages/routes
```

`styles/tokens` has no product imports. Generic UI under `components/ui` may
depend on tokens and other generic UI, but never on `features/*`. Feature
components may compose generic UI and feature-local components. Pages/routes
assemble features and may select layout/theme context, but may not implement a
new shared visual recipe. Vendor CSS and game effects are leaves and cannot
become a source for application UI.

## Required Change Workflow

Product and page code maintain a zero-raw-control baseline: no native
`button`, `input`, `textarea`, or `select`, and no clickable `div`/`span`
substitutes. Reuse a named shared primitive; if none fits, add or extend the
generic primitive first instead of creating a local wrapper or exception.

A new semantic token, primitive-owned native element, geometry exception, or
validator allowance is an architecture change. It must include the rationale,
the narrowest possible ownership, documentation here, and a focused
`lint:architecture:self-test` case proving both the allowed use and the nearest
forbidden bypass. Whole-feature and whole-directory exemptions are prohibited.

Frontend changes are not complete until the architecture self-tests and strict
scan pass together with the relevant tests and TypeScript check.

## Enforcement

Run:

```bash
npm --prefix apps/web run lint:architecture
```

The equivalent repository-root invocation also works:

```bash
node apps/web/scripts/check-frontend-architecture.mjs
```

The checker is fail-closed for all product, admin, moderator, and document
surfaces. It rejects raw palettes and literals, legacy aliases, default
typography, unapproved arbitrary visual utilities, unknown named design values,
generic-UI boundary violations, and stale line budgets.

Use strict mode when you want the report to fail the command:

```bash
npm --prefix apps/web run lint:architecture:strict
```

Run the bounded validator contract checks with:

```bash
npm --prefix apps/web run lint:architecture:self-test
```

CI runs `lint:architecture:strict` before frontend tests and the production
build. A change that introduces a new design value must update the owned token
contract and its focused validator coverage in the same change.
