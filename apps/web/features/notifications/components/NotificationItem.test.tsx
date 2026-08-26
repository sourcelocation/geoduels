import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { NotificationItem, notificationDetails, parseNotificationEntry } from "./NotificationItem";

describe("NotificationItem", () => {
  it("uses the match destination and unread treatment for rating refunds", () => {
    render(<NotificationItem entry={{ kind: "notification", notification: { id: 7, type: "mmr_refund", payload: { matchId: "match-42" }, createdAt: "2026-08-11T12:00:00Z" } }} />);
    expect(screen.getByRole("link", { name: /rating refunded, unread/i })).toHaveAttribute("href", "/match/match-42");
    expect(screen.getByText("New")).toBeInTheDocument();
  });

  it("keeps badge detail in the shared notification presentation", () => {
    const details = notificationDetails({ id: 8, type: "badge_unlocked", payload: { badge: { label: "Pathfinder", description: "Win on five maps." } }, createdAt: "2026-08-11T12:00:00Z", readAt: "2026-08-11T12:01:00Z" });
    expect(details.title).toBe("Pathfinder unlocked");
    expect(details.body).toBe("Win on five maps.");
  });

  it("presents enforcement and reporter outcomes clearly", () => {
    expect(notificationDetails({ id: 9, type: "account_unbanned", payload: {}, createdAt: "2026-08-11T12:00:00Z" }).title).toBe("Account restriction removed");
    const report = notificationDetails({ id: 10, type: "reported_player_banned", payload: {}, createdAt: "2026-08-11T12:00:00Z" });
    expect(report.title).toBe("Report action taken");
    expect(report.body).toContain("suspended after review");
  });

  it("parses pending social actions through the same presentation", () => {
    const entry = {
      kind: "friend_request" as const,
      request: {
        id: "request-1",
        direction: "incoming" as const,
        player: { userId: "player-1", displayName: "Mira", relationship: "incoming_request" as const },
        createdAt: "2026-08-11T12:00:00Z",
        expiresAt: "2026-09-11T12:00:00Z",
      },
    };
    expect(parseNotificationEntry(entry).title).toBe("Mira sent a friend request");
    render(<NotificationItem entry={entry} onAction={() => undefined} />);
    expect(screen.getByRole("button", { name: /Accept: Mira sent a friend request/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Decline: Mira sent a friend request/i })).toBeInTheDocument();
  });
});
