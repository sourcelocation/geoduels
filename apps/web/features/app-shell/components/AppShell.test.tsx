import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { AppShell } from "./AppShell";

afterEach(() => {
  cleanup();
  window.sessionStorage.clear();
});

describe("AppShell", () => {
  it("orders navigation routes and renders separate online presence", () => {
    render(
      <AppShell activeNavRoute="play" onlinePlayers={42}>
        <div>Content</div>
      </AppShell>,
    );

    const navigation = screen.getByRole("navigation", {
      name: "Primary navigation",
    });
    expect(
      Array.from(navigation.querySelectorAll("a")).map((link) =>
        link.textContent?.trim(),
      ),
    ).toEqual(["Play", "Maps", "Friends", "Top"]);
    expect(screen.getByLabelText("42 players online")).toHaveTextContent("42");
    expect(screen.queryByText("42 Playing")).not.toBeInTheDocument();
  });

  it("marks the active route in desktop and mobile navigation", async () => {
    render(
      <AppShell activeNavRoute="maps">
        <div>Content</div>
      </AppShell>,
    );

    const mapLinks = await screen.findAllByRole("link", { name: "Maps" });
    expect(mapLinks).toHaveLength(1);
    expect(mapLinks.every((link) => link.getAttribute("aria-current") === "page")).toBe(true);
  });

  it("supports contextual pages with no selected navigation item", () => {
    render(
      <AppShell activeNavRoute={null}>
        <div>Profile</div>
      </AppShell>,
    );

    expect(
      screen
        .getAllByRole("link")
        .filter((link) => link.hasAttribute("aria-current")),
    ).toHaveLength(0);
  });

  it("replaces navigation links with disabled entries while queueing", () => {
    render(
      <AppShell activeNavRoute="play" navigationDisabled>
        <div>Queueing</div>
      </AppShell>,
    );

    expect(screen.queryByRole("link", { name: "Play" })).not.toBeInTheDocument();
    expect(screen.getAllByText("Play")).toHaveLength(1);
    expect(screen.getAllByText("Play")[0].closest("[aria-disabled=true]")).toBeTruthy();
  });
});
