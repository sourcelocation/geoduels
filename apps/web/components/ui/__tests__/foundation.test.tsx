import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { CounterBadge } from "../Badge";
import { Button, IconButton } from "../button";
import { AppChromeIconButton, AppPanel, ContentUnavailable, DangerZone, SectionCard, SettingsGroup } from "../compositions";
import { Popover } from "../Popover";
import { SettingRow } from "../patterns";
import { Switch } from "../Switch";

describe("component-system foundation", () => {
  it("maps purpose-owned compositions to stable semantic roles", () => {
    const { container } = render(<><AppPanel>Panel</AppPanel><SectionCard>Section</SectionCard><SettingsGroup>Settings</SettingsGroup><ContentUnavailable>Nothing here</ContentUnavailable><DangerZone>Danger</DangerZone></>);
    expect(screen.getByText("Panel")).toHaveClass("translucent-surface");
    expect(screen.getByText("Section")).toHaveClass("bg-surface-grouped");
    expect(screen.getByText("Nothing here")).toHaveClass("border-dashed");
    expect(container.querySelector(".bg-surface-inset")).toBeNull();
  });
  it("caps counter badges while preserving the full count for assistive technology", () => {
    render(<CounterBadge count={105} label="unread items" />);
    expect(screen.getByText("99+")).toBeInTheDocument();
    expect(screen.getByLabelText("105 unread items")).toBeInTheDocument();
  });

  it("uses the shared backdrop surface for standalone app chrome controls", () => {
    render(<AppChromeIconButton aria-label="Chrome action">C</AppChromeIconButton>);
    const control = screen.getByRole("button", { name: "Chrome action" });
    expect(control).toHaveClass("translucent-surface", "!rounded-full", "overflow-visible");
    expect(control).not.toHaveClass("bg-surface-fill");
  });

  it("exposes switch state and reports the next value", () => {
    const onCheckedChange = vi.fn();
    render(<Switch aria-label="Party invites" checked={false} onCheckedChange={onCheckedChange} />);
    const control = screen.getByRole("switch", { name: "Party invites" });
    expect(control).toHaveAttribute("aria-checked", "false");
    fireEvent.click(control);
    expect(onCheckedChange).toHaveBeenCalledWith(true);
  });

  it("opens a focus-managed popover with the document font and semantic floating surface", () => {
    render(<div data-ui-theme="operational"><Popover content={<p>Current notifications</p>}><IconButton aria-label="Notifications">N</IconButton></Popover></div>);
    fireEvent.click(screen.getByRole("button", { name: "Notifications" }));
    const popover = screen.getByRole("dialog");
    expect(popover).toHaveClass("font-body", "bg-surface-panel", "shadow-elev-3");
    expect(popover).not.toHaveClass("translucent-surface");
    expect(popover).toHaveAttribute("data-ui-theme", "operational");
    expect(screen.getByText("Current notifications")).toBeInTheDocument();
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByText("Current notifications")).not.toBeInTheDocument();
  });

  it("keeps settings labels and controls in one responsive row", () => {
    render(<SettingRow title="Allow party invites" description="Friends can invite you." control={<Button>Save</Button>} />);
    expect(screen.getByText("Allow party invites")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save" })).toBeInTheDocument();
  });
});
