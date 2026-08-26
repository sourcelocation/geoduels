import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Button, IconButton } from "../button";
import { EmptyState } from "../EmptyState";
import { Field } from "../Field";
import { Input } from "../input";
import { Surface } from "../Surface";
import { Tabs } from "../Tabs";
import { Heading, Text } from "../typography";

describe("shared UI primitives", () => {
  it("renders button variants without dropping native button props", () => {
    render(
      <Button variant="primary" disabled aria-label="Create map">
        Create
      </Button>,
    );

    expect(screen.getByRole("button", { name: "Create map" })).toBeDisabled();
  });

  it("supports compact icon controls without inherited text-button padding", () => {
    render(<IconButton size="icon-sm" aria-label="Report player">!</IconButton>);

    expect(screen.getByRole("button", { name: "Report player" })).toHaveClass("h-7", "w-7", "p-0");
  });

  it("renders a surface as the requested semantic element", () => {
    render(
      <Surface as="article" material="operational">
        Admin content
      </Surface>,
    );

    expect(screen.getByText("Admin content").closest("article")).not.toBeNull();
  });

  it("separates typography roles from HTML semantics", () => {
    render(<><Heading as="h1" variant="heading-sm">Page title</Heading><Text as="span" variant="body">Inline copy</Text></>);

    expect(screen.getByRole("heading", { level: 1, name: "Page title" })).toHaveClass("text-heading-sm");
    expect(screen.getByText("Inline copy").tagName).toBe("SPAN");
    expect(screen.getByText("Inline copy")).toHaveClass("text-body");
  });

  it("exposes the dedicated gameplay countdown role", () => {
    render(<Text variant="hud-countdown">3</Text>);

    expect(screen.getByText("3")).toHaveClass("text-hud-countdown");
  });

  it("adds accessible field metadata while preserving native control props", () => {
    render(<Field label="Email" error="Enter a valid email"><Input type="email" /></Field>);

    const input = screen.getByLabelText("Email");
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(input).toHaveAccessibleDescription("Enter a valid email");
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
