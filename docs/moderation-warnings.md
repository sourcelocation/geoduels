# Moderation warnings

Admins and judges can issue and withdraw warnings from a player's page in the
moderation panel. Categories are chat abuse, staff impersonation, inappropriate
nickname, and other. The message is visible to the player; optional chat-message
evidence must belong to the subject. The warning card shows the issuer, time,
acknowledgment, any nickname reset, and withdrawal status.

A warning does not restrict the account or escalate to a mute or ban. Players
see a prompt and an inbox notification; the prompt waits until an ongoing match
ends, and closing it leaves a reminder until the player acknowledges it.

With **Reset nickname**, the player's public nickname is replaced with a random
neutral name in the same transaction as the warning. Nothing else changes: the
player keeps playing and chatting and can choose a new nickname from their
profile at any time. Guests always play as "Guest", so their nickname cannot be
reset. Withdrawing a warning does not restore the old nickname.

## Version 1 API

All routes require authentication. Staff routes require report-review or
access-management permission.

- `GET /api/v1/staff/players/:id/warnings`: the latest 200 warnings.
- `POST /api/v1/staff/players/:id/warnings`: issue a warning with `category`,
  `message` (1–1000 characters), `resetNickname`, and optional
  `evidenceMessageId` (chat message UUID).
- `POST /api/v1/staff/players/:id/warnings/:warningId/withdraw`: withdraw a warning.
- `GET /api/v1/me/warnings`: the player's own warnings, excluding withdrawn
  ones and issuer or evidence details.
- `POST /api/v1/me/warnings/:warningId/acknowledge`: acknowledge one's own warning;
  repeating acknowledgment preserves its first timestamp.

The `moderation_warning` inbox type includes `warningId`, `category`, `reason`,
and `nicknameReset` when a nickname was reset; withdrawal updates that
notification with `withdrawn: true`. Older clients keep their generic
notification fallback.

Apply `002015_moderation_warnings.up.sql` before deploying these backend changes.
Migrations are applied separately from release deployments. Regenerate sqlc
before building or testing; no generated files are committed.
