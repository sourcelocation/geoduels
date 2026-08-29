import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { RelationshipActions } from "./RelationshipActions";
import type { CompactPlayer } from "../types";

const friend: CompactPlayer = {
  userId: "friend-1",
  displayName: "Ada",
  relationship: "friends",
};

describe("RelationshipActions", () => {
  it("does not show a party invite on the friends list", () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <RelationshipActions accessToken="token" player={friend} />
      </QueryClientProvider>,
    );
    expect(screen.queryByRole("button", { name: "Invite" })).not.toBeInTheDocument();
  });
});
