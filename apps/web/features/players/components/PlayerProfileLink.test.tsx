import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import PlayerProfileLink from "./PlayerProfileLink";

describe("PlayerProfileLink", () => {
  afterEach(cleanup);

  it("builds a public player route", () => {
    render(<PlayerProfileLink userId="player-1">Explorer</PlayerProfileLink>);
    expect(screen.getByRole("link", { name: "Explorer" })).toHaveAttribute("href", "/players/Explorer");
  });

  it("renders disabled identities without a link", () => {
    render(<PlayerProfileLink userId="guest-1" disabled>Guest</PlayerProfileLink>);
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(screen.getByText("Guest")).toBeInTheDocument();
  });
});
