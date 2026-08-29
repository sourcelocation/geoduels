import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useNotificationActions } from "./useNotificationActions";

const respondPartyInvite = vi.fn();
const joinParty = vi.fn();
const routerReplace = vi.fn();

vi.mock("../../social/lib/social-client", () => ({
  socialClient: {
    respondPartyInvite: (...args: unknown[]) => respondPartyInvite(...args),
    respondRequest: vi.fn(),
  },
}));

vi.mock("../../auth/auth-gateway", () => ({
  getAuthGateway: () => ({ bootstrap: vi.fn(async () => {}) }),
}));

vi.mock("next/router", () => ({
  useRouter: () => ({ asPath: "/friends", replace: routerReplace }),
}));

function HookHost({ onReady }: { onReady: (mutate: ReturnType<typeof useNotificationActions>["mutateAsync"]) => void }) {
  const action = useNotificationActions("token", joinParty);
  onReady(action.mutateAsync);
  return null;
}

afterEach(() => {
  vi.clearAllMocks();
});

describe("useNotificationActions", () => {
  it("joins the party through the coordinator instead of a full page redirect", async () => {
    respondPartyInvite.mockResolvedValue({ id: "invite-1", inviteCode: "ABCD12" });
    joinParty.mockResolvedValue(true);
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    let mutateAsync: ReturnType<typeof useNotificationActions>["mutateAsync"] | undefined;
    render(
      <QueryClientProvider client={queryClient}>
        <HookHost onReady={(mutate) => { mutateAsync = mutate; }} />
      </QueryClientProvider>,
    );

    await act(async () => {
      await mutateAsync?.({ kind: "party", id: "invite-1", value: "accept" });
    });

    expect(joinParty).toHaveBeenCalledWith("ABCD12");
    expect(routerReplace).toHaveBeenCalledWith("/party/ABCD12", undefined, { shallow: true });
  });

  it("does not join when the invitation is declined", async () => {
    respondPartyInvite.mockResolvedValue({ id: "invite-1", inviteCode: "ABCD12" });
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    let mutateAsync: ReturnType<typeof useNotificationActions>["mutateAsync"] | undefined;
    render(
      <QueryClientProvider client={queryClient}>
        <HookHost onReady={(mutate) => { mutateAsync = mutate; }} />
      </QueryClientProvider>,
    );

    await act(async () => {
      await mutateAsync?.({ kind: "party", id: "invite-1", value: "decline" });
    });

    expect(joinParty).not.toHaveBeenCalled();
    expect(routerReplace).not.toHaveBeenCalled();
  });
});
