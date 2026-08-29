import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { InviteFriendsModal } from "./InviteFriendsModal";
import type { CompactPlayer } from "../types";

function friend(partial: Partial<CompactPlayer> & Pick<CompactPlayer, "userId" | "displayName">): CompactPlayer {
  return {
    relationship: "friends",
    ...partial,
  };
}

function renderModal(memberUserIds: string[] = ["self"]) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const onClose = vi.fn();
  render(
    <QueryClientProvider client={queryClient}>
      <InviteFriendsModal
        accessToken="token"
        partyId="party-1"
        memberUserIds={memberUserIds}
        onClose={onClose}
      />
    </QueryClientProvider>,
  );
  return { onClose };
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("InviteFriendsModal", () => {
  it("lists friends who are not already in the party and invites through the shared party invite action", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("/v1/me/friends-page?partyId=party-1")) {
        return new Response(
          JSON.stringify({
            friends: [
              friend({ userId: "self", displayName: "Self", presenceStatus: "online" }),
              friend({ userId: "online-friend", displayName: "Online Friend", presenceStatus: "online" }),
              friend({ userId: "offline-friend", displayName: "Offline Friend", presenceStatus: "offline" }),
            ],
            requests: { incoming: [], outgoing: [] },
            recentPlayers: [],
          }),
          { status: 200 },
        );
      }
      if (url.includes("/v1/parties/party-1/invitations") && init?.method === "POST") {
        return new Response(
          JSON.stringify({ id: "invite-1", createdAt: new Date().toISOString(), expiresAt: new Date(Date.now() + 20 * 60_000).toISOString() }),
          { status: 201 },
        );
      }
      return new Response("not found", { status: 404 });
    });
    vi.stubGlobal("fetch", fetchMock);

    renderModal(["self"]);

    expect(await screen.findByText("Online Friend")).toBeInTheDocument();
    expect(screen.getByText("Offline Friend")).toBeInTheDocument();
    expect(screen.queryByText("Self")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Invite Online Friend to party" }));
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Invite sent to Online Friend" })).toHaveTextContent("Invite Sent");
    });
    expect(
      fetchMock.mock.calls.some(
        ([input, init]) =>
          String(input).includes("/v1/parties/party-1/invitations") &&
          init?.method === "POST" &&
          String(init.body).includes("online-friend"),
      ),
    ).toBe(true);
  });

  it("restores Invite Sent from friends-page party invite status", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input).includes("/v1/me/friends-page?partyId=party-1")) {
          return new Response(
            JSON.stringify({
              friends: [
                friend({
                  userId: "online-friend",
                  displayName: "Ada",
                  presenceStatus: "online",
                  partyInvite: {
                    id: "invite-1",
                    createdAt: new Date().toISOString(),
                    expiresAt: new Date(Date.now() + 20 * 60_000).toISOString(),
                  },
                }),
              ],
              requests: { incoming: [], outgoing: [] },
              recentPlayers: [],
            }),
            { status: 200 },
          );
        }
        return new Response("not found", { status: 404 });
      }),
    );
    renderModal(["self"]);
    const button = await screen.findByRole("button", { name: "Invite sent to Ada" });
    expect(button).toHaveTextContent("Invite Sent");
    expect(button).toBeDisabled();
  });
});
