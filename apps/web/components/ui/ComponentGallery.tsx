import { Bell, Trophy } from "lucide-react";
import { Badge, CounterBadge } from "./Badge";
import { Button } from "./button";
import { AppPanel, ContentUnavailable, DangerZone, SectionCard, SettingsGroup } from "./compositions";
import { Notice, SettingRow } from "./patterns";
import { Switch } from "./Switch";

/** Development and review fixture for comparing the complete visual language. */
export function ComponentGallery() {
  return (
    <div data-ui-theme="game" className="grid gap-6 bg-surface-page p-6 text-content-primary">
      <AppPanel className="grid gap-4 p-5">
        <div className="flex flex-wrap items-center gap-2"><Badge>Neutral</Badge><Badge tone="success">Ready</Badge><CounterBadge count={12} label="notifications" /></div>
        <div className="grid gap-3 sm:grid-cols-2">
          {[{ icon: Trophy, label: "Duel wins", value: 24 }, { icon: Bell, label: "Notifications", value: 3 }].map(({ icon: Icon, label, value }) => (
            <div key={label} className="flex items-center gap-3 rounded-lg border border-border-default bg-surface-grouped p-3">
              <Icon className="h-4 w-4 text-status-success" aria-hidden="true" />
              <span className="text-body-sm text-content-secondary">{label}</span>
              <strong className="ml-auto text-heading-sm font-strong text-content-primary">{value}</strong>
            </div>
          ))}
        </div>
        <SectionCard className="p-4">Grouped content</SectionCard>
        <SettingsGroup><SettingRow title="Allow invites" description="Friends can invite you to a party." control={<Switch checked onCheckedChange={() => undefined} aria-label="Allow invites" />} /></SettingsGroup>
        <ContentUnavailable>No maps in this section yet.</ContentUnavailable>
        <Notice title="Connection restored" tone="success">You are back online.</Notice>
        <DangerZone><p className="text-body font-strong">Danger zone</p><Button variant="danger" className="mt-3">Delete account</Button></DangerZone>
      </AppPanel>
    </div>
  );
}
