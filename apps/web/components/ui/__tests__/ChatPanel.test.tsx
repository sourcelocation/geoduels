import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CHAT_MUTED_STORAGE_KEY } from "../../../features/chat/lib/chat-preferences";
import ChatPanel from "../ChatPanel";

describe("ChatPanel", () => {
  beforeEach(() => {
    const values = new Map<string, string>();
    Object.defineProperty(window, "localStorage", {
      configurable: true,
      value: {
        clear: () => values.clear(),
        getItem: (key: string) => values.get(key) ?? null,
        removeItem: (key: string) => values.delete(key),
        setItem: (key: string, value: string) => values.set(key, value),
      },
    });
    window.localStorage.clear();
  });

  afterEach(() => {
    cleanup();
    window.localStorage.clear();
  });

  it("persists mute and unmutes when chat is opened again", () => {
    render(
      <ChatPanel
        messages={[]}
        selfUserId="self"
        onSendMessage={vi.fn(() => true)}
        onSendEmote={vi.fn(() => true)}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Open chat" }));
    fireEvent.click(screen.getByRole("button", { name: "Mute chat" }));

    expect(window.localStorage.getItem(CHAT_MUTED_STORAGE_KEY)).toBe("true");
    expect(screen.getByRole("button", { name: "Open chat and unmute" })).toBeInTheDocument();
    expect(screen.getByText("Chat muted")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Open chat and unmute" }));

    expect(window.localStorage.getItem(CHAT_MUTED_STORAGE_KEY)).toBe("false");
    expect(screen.getByRole("button", { name: "Mute chat" })).toBeInTheDocument();
  });

  it("restores the muted preference", async () => {
    window.localStorage.setItem(CHAT_MUTED_STORAGE_KEY, "true");

    render(
      <ChatPanel
        messages={[]}
        selfUserId="self"
        onSendMessage={vi.fn(() => true)}
        onSendEmote={vi.fn(() => true)}
      />,
    );

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Open chat and unmute" })).toBeInTheDocument();
    });
  });

  it("renders review mode without interactive controls", () => {
    render(
      <ChatPanel
        mode="review"
        messages={[{
          id: "message-1",
          matchId: "match-1",
          senderUserId: "player-1",
          senderDisplayName: "Explorer",
          kind: "text",
          body: "hello",
          createdAt: "2026-06-20T12:00:00Z",
        }]}
        selfUserId=""
      />,
    );

    expect(screen.getByText("Explorer")).toBeInTheDocument();
    expect(screen.getByText("hello")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Mute chat" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Send message" })).not.toBeInTheDocument();
    expect(screen.queryByPlaceholderText("Message")).not.toBeInTheDocument();
  });
});
