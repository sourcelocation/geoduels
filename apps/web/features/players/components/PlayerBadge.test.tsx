import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import PlayerBadge from "./PlayerBadge";

afterEach(() => cleanup());

describe("PlayerBadge", () => {
  it("renders the V1 best rank near the bottom of the badge", () => {
    render(
      <PlayerBadge
        badge={{
          id: "geoduels-v1-top-finish",
          kind: "legacy_top_finish",
          label: "GeoDuels V1 Top Finish",
          description: "Finished in the global top 100 during GeoDuels V1.",
          imageUrl: "/badges/geoduels-v1-top-finish-badge.v1.png",
          extra: 42,
        }}
        size="lg"
      />,
    );

    const rank = screen.getByText("#42");
    expect(rank).toHaveClass("bottom-0");
  });

  it("does not overlay a rank on the leveled top-finish badge", () => {
    render(
      <PlayerBadge
        badge={{
          id: "top-finish",
          kind: "top_finish",
          label: "Top Finisher (3)",
          description: "Finished in the global top 100.",
          imageUrl: "/badges/top-finish-3-badge.v1.png",
          level: 3,
          maxLevel: 3,
          extra: 5,
        }}
      />,
    );

    expect(screen.queryByText("#5")).not.toBeInTheDocument();
  });
});
