# Frontend Architecture Rules

## Ownership

- New files under `components/ui` should be generic primitives.
- Existing gameplay-specific components under `components/ui` are transitional legacy ownership and are explicitly tracked by the architecture checker. New gameplay UI belongs under its feature directory.
- Feature UI belongs under `features/<feature>/components`.
- Feature state used by one panel belongs in that panel or a nearby hook.
- `LobbyScreen` should remain a route/composition shell, not a feature owner.

## Styling

Inline `className` is for layout and spacing:

- `flex`, `grid`, `gap-*`, `w-*`, `max-w-*`
- responsive layout classes
- small spacing overrides when a component API would be awkward

Inline `className` must not create visual recipes:

- no `bg-[#...]`
- no `rounded-[...]`
- no `shadow-[...]`
- no `backdrop-blur-*`
- no direct `glass-panel` usage outside approved primitive/legacy files
- no repeated `border border-white/10 bg-black/20` style clusters

Add or reuse a primitive/variant instead:

- `LobbyPanel`
- `LobbyInset`
- `LobbyMutedBox`
- `LobbyDangerNotice`
- `LobbyActionButton`
- `LobbyActionLink`
- `LobbyCardButton`
- `LobbyPill`
- `LobbyNotice`
- `LobbySectionHeader`
- `LobbySegmentedControl`
- `LobbyFieldLabel`
- `LobbyIconButton`
- `LobbyLoadingState`
- `LobbyInput`, `LobbyTextarea`, `LobbySelect`

## Shared Component Index

Use these existing components before creating new visual recipes. Keep this index short and update it when a component becomes a preferred pattern.

### Generic UI

- `Surface`: base framed surface with `gameGlass`, `gameSolid`, `operational`, `danger`, and `subtle` variants.
- `Button`: generic button with `primary`, `secondary`, `ghost`, `danger`, and `icon` variants.
- `Input`, `Textarea`, `Select`: generic form controls; use `variant="game"` for game/lobby surfaces and default operational styling elsewhere.
- `Field`: label, helper text, and error wrapper for forms.
- `TableShell`, `Table`, `TableHead`: operational table primitives.
- `Metric`: compact label/value metric for operational dashboards.
- `StatusPill`: small status label for admin and operational state.
- `Tabs`: compact tab selector for switching between small view sets.
- `Toolbar`: horizontal action container for grouped controls.
- `EmptyState`: reusable empty/loading-adjacent state with optional action.
- `Tooltip` and `TooltipProvider`: hover/focus explanation for icon-only or unfamiliar controls.
- `AppModalShell` / `Modal`: standard modal shell with header, escape close, backdrop handling, and responsive placement.
- `PageShell`: generic page wrapper for non-lobby pages.
- `MarkdownContent`: render trusted markdown-style content with app typography.
- `RelativeTime`: render timestamps as relative labels.
- `IconMetric`: icon plus metric text for compact stat displays.
- `Eyebrow`, `SectionTitle`, `BodyText`, `MutedText`, `HelperText`: shared typography atoms.

### Player And Identity UI

- `PlayerProfileLink`: use for any player/user name or avatar that should navigate to `/players/[id]`; pass `userId` and `nickname`, and set `disabled` for guests or system users.
- `PlayerNameWithBadge`: player name plus selected badge, already wrapped with `PlayerProfileLink`.
- `PlayerBadge`: render a selected badge; use `badgeTitle` for tooltip/title text.
- `AvatarBadge`: avatar image with fallback initials and size variants.
- `ParticipantAvatar`, `ParticipantName`, `ParticipantIdentityRow`, `ParticipantIdentityCard`: match participant identity blocks for player/team-aware gameplay UI.
- `MmrDisplay`: rating display with trophy icon sizing.

### Lobby Primitives

- `LobbyPanel`: standard lobby/game surface wrapper; prefer over direct glass/surface class clusters.
- `LobbyInset`: nested subtle/danger/etc. inset surface inside lobby panels.
- `LobbyMutedBox`: quiet informational box or empty state inside lobby panels.
- `LobbyDangerNotice`: destructive/error notice for lobby flows.
- `LobbyNotice`: success, warning, danger, or muted feedback notice.
- `LobbyActionButton`: lobby-styled command button; use for primary and secondary actions.
- `LobbyActionLink`: link styled like a lobby action button.
- `LobbyCardButton`: large clickable lobby card.
- `LobbyPill`: compact status/category pill.
- `LobbyFieldLabel`: uppercase lobby form label.
- `LobbySegmentedControl`: mutually exclusive small option set, such as difficulty or visibility.
- `LobbyIconButton`: circular icon-only lobby button.
- `LobbyLoadingState`: spinner/text loading row.
- `LobbySectionHeader`: section eyebrow, title, and optional description.
- `LobbyInput`, `LobbyTextarea`, `LobbySelect`: lobby/game variants of generic form controls.

### Specialized Or Transitional

- `ChatPanel`, `MinimapPanel`, `GameHUD`, `GameStartOverlay`, `RoundResultOverlay`, `EndMatchOverlay`, `PrematchVersusOverlay`, `MatchSideHPCard`, `MatchSideProfile`, `TopHeader`, and `LobbyScreen` are gameplay/lobby-specific or transitional legacy components. Reuse only when extending the same surface; new gameplay UI belongs under its feature directory.

## Operational UI

Admin, legal, and document surfaces should use operational primitives:

- `Surface variant="operational"`
- `Button`
- `Input`
- `Select`
- `Textarea`
- `Table`
- `Metric`
- `StatusPill`

Admin should not use decorative glass, hero-style sections, or ad hoc gradients.

## Budgets

Run:

```bash
npm --prefix apps/web run lint:architecture
```

The checker prints an advisory report by default. Existing legacy debt is tracked with per-file line and visual-recipe budgets so future changes must hold or reduce the count. A budget is temporary compatibility accounting, not approval to copy the recipe into another file.

Use strict mode when you want the report to fail the command:

```bash
npm --prefix apps/web run lint:architecture:strict
```

The checker is currently a manual/local guardrail and is not invoked by `.github/workflows/ci.yml`. CI runs the frontend unit tests and production build. Run strict architecture checks before submitting frontend standardization changes.
