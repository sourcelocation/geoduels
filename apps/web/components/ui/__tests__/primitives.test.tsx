import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Button } from "../button";
import { EmptyState } from "../EmptyState";
import { Field } from "../Field";
import { Surface } from "../Surface";
import { Tabs } from "../Tabs";

describe("shared UI primitives", () => {
  it("renders button variants without dropping native button props", () => {
    render(
      <Button variant="primary" disabled aria-label="Create map">
        Create
      </Button>,
    );

    expect(screen.getByRole("button", { name: "Create map" })).toBeDisabled();
  });

  it("renders a surface as the requested semantic element", () => {
    render(
      <Surface as="article" variant="operational">
        Admin content
      </Surface>,
    );

    expect(screen.getByText("Admin content").closest("article")).not.toBeNull();
  });

  it("notifies consumers when a tab changes", () => {
    const onChange = vi.fn();
    render(
      <Tabs
        value="maps"
        items={[
          { id: "maps", label: "Maps" },
          { id: "party", label: "Party" },
        ]}
        onChange={onChange}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Party" }));
    expect(onChange).toHaveBeenCalledWith("party");
  });

  it("associates field labels with controls", () => {
    render(
      <Field label="Display name" helper="Visible to other players">
        <input id="display-name" />
      </Field>,
    );

    expect(screen.getByLabelText("Display name")).toBeInTheDocument();
    expect(screen.getByText("Visible to other players")).toBeInTheDocument();
  });

  it("renders empty states with optional actions", () => {
    render(<EmptyState title="No maps" message="Create or browse a map." action={<Button>Upload</Button>} />);

    expect(screen.getByText("No maps")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Upload" })).toBeInTheDocument();
  });
});
