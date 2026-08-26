import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { BadgeGrantsRoute } from "./BadgeGrantsRoute";

const mocks = vi.hoisted(() => ({
  requestAdminBadgeDefinitions: vi.fn(),
  requestAdminGrantBadge: vi.fn(),
}));

vi.mock("../lib/admin-client", () => mocks);

function renderRoute() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <BadgeGrantsRoute config={{} as never} accessToken="access-token" canManageAdmin />
    </QueryClientProvider>,
  );
}

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("BadgeGrantsRoute", () => {
  it("loads the server-authoritative grant catalog", async () => {
    mocks.requestAdminBadgeDefinitions.mockResolvedValue({
      badges: [{ id: "event-winner-2026", label: "2026 Event Winner", description: "Event award", maxLevel: 1 }],
    });
    renderRoute();
    expect(await screen.findByRole("option", { name: "2026 Event Winner" })).toBeInTheDocument();
    expect(mocks.requestAdminBadgeDefinitions).toHaveBeenCalledWith(expect.anything(), "access-token");
  });

  it("grants the selected badge to the entered player", async () => {
    mocks.requestAdminBadgeDefinitions.mockResolvedValue({
      badges: [{ id: "event-winner-2026", label: "2026 Event Winner", description: "Event award", maxLevel: 1 }],
    });
    mocks.requestAdminGrantBadge.mockResolvedValue({
      badge: { id: "event-winner-2026", label: "2026 Event Winner" },
      changed: true,
    });
    renderRoute();
    await screen.findByRole("option", { name: "2026 Event Winner" });
    fireEvent.change(screen.getByLabelText("Nickname"), { target: { value: "MapMaster" } });
    fireEvent.change(screen.getByLabelText("Badge"), { target: { value: "event-winner-2026" } });
    fireEvent.click(screen.getByRole("button", { name: "Give Badge" }));
    await waitFor(() => expect(mocks.requestAdminGrantBadge).toHaveBeenCalledWith(
      expect.anything(),
      "access-token",
      { nickname: "MapMaster", badgeId: "event-winner-2026" },
    ));
    expect(await screen.findByText("2026 Event Winner granted to MapMaster.")).toBeInTheDocument();
  });

  it("reports an idempotent repeat without claiming a new grant", async () => {
    mocks.requestAdminBadgeDefinitions.mockResolvedValue({
      badges: [{ id: "event-winner-2026", label: "2026 Event Winner", description: "Event award", maxLevel: 1 }],
    });
    mocks.requestAdminGrantBadge.mockResolvedValue({
      badge: { id: "event-winner-2026", label: "2026 Event Winner" },
      changed: false,
    });
    renderRoute();
    await screen.findByRole("option", { name: "2026 Event Winner" });
    fireEvent.change(screen.getByLabelText("Nickname"), { target: { value: "MapMaster" } });
    fireEvent.change(screen.getByLabelText("Badge"), { target: { value: "event-winner-2026" } });
    fireEvent.click(screen.getByRole("button", { name: "Give Badge" }));
    expect(await screen.findByText("2026 Event Winner is already owned by MapMaster.")).toBeInTheDocument();
  });
});
