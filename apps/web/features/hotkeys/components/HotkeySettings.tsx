import { RotateCcw, X } from "lucide-react";
import { useEffect, useState } from "react";
import { Dialog } from "../../../components/ui/Dialog";
import { IconButton, Button } from "../../../components/ui/button";
import { InsetList, Notice, SectionHeader, SettingRow } from "../../../components/ui/patterns";
import { Kbd } from "../../../components/ui/Kbd";
import { Switch } from "../../../components/ui/Switch";
import { Tabs } from "../../../components/ui/Tabs";
import { SocialPrivacySettings } from "../../social/components/SocialPrivacySettings";
import { AccountSettings } from "../../players/components/AccountSettingsModal";
import { bindingFromEvent, formatBinding } from "../lib/keyboard-event";
import { bindingKey, HOTKEY_ACTIONS } from "../model/defaults";
import type { HotkeyAction } from "../model/types";
import { useHotkeys } from "./HotkeyProvider";

export default function HotkeySettings({ onClose }: { onClose: () => void }) {
  const state = useHotkeys();
  const [tab, setTab] = useState<"Controls" | "Privacy" | "Audio" | "Account">("Controls");
  const [capturing, setCapturing] = useState<HotkeyAction | null>(null);
  const [notice, setNotice] = useState("");

  useEffect(() => {
    if (!capturing) return;
    const capture = (event: KeyboardEvent) => {
      event.preventDefault();
      event.stopImmediatePropagation();
      const action = capturing;
      if (event.code === "Escape") {
        setCapturing(null);
        return;
      }
      const binding = bindingFromEvent(event);
      if (!binding) return;
      const conflict = HOTKEY_ACTIONS.find((item) =>
        item.action !== action &&
        state.preferences.bindings[item.action].some((candidate) => bindingKey(candidate) === bindingKey(binding)));
      state.setBinding(action, binding);
      setNotice(conflict ? `${formatBinding(binding)} was moved from “${conflict.label}”.` : "");
      setCapturing(null);
    };
    window.addEventListener("keydown", capture, true);
    return () => window.removeEventListener("keydown", capture, true);
  }, [capturing, state]);

  return (
    <Dialog title="Settings" onClose={onClose} maxWidthClassName="max-w-2xl">
      <Tabs
        appearance="segmented"
        value={tab}
        onChange={setTab}
        items={(["Controls", "Privacy", "Audio", "Account"] as const).map((label) => ({ id: label, label }))}
        aria-label="Settings section"
        className="mb-5"
      />
      {notice ? <Notice tone="warning" className="mb-4">{notice}</Notice> : null}

      {tab === "Privacy" ? (
        <section>
          <SectionHeader title="Social privacy" description="Choose how other registered players can find and contact you." className="mb-3" />
          <SocialPrivacySettings />
        </section>
      ) : tab === "Account" ? (
        <AccountSettings profilePath={typeof window === "undefined" ? "/" : window.location.pathname} />
      ) : tab === "Audio" ? (
        <section>
          <InsetList>
            <SettingRow title="Chat alerts" info="Play an alert for incoming chat message previews." control={<Switch checked={!state.preferences.audioMuted} onCheckedChange={(checked) => state.setAudioMuted(!checked)} aria-label="Enable chat alerts" />} />
            <SettingRow title="Sound effects" info="Play interface and gameplay sound effects." control={<Switch checked={!state.preferences.sfxMuted} onCheckedChange={(checked) => state.setSfxMuted(!checked)} aria-label="Enable sound effects" />} />
          </InsetList>
        </section>
      ) : (
        <section>
          <InsetList>
            {HOTKEY_ACTIONS.map((item) => {
              const binding = state.preferences.bindings[item.action][0];
              return (
                <SettingRow key={item.action} title={item.label} info={item.description} stackControlOnMobile control={
                  <div className="flex shrink-0 items-center gap-2">
                    <Button
                      type="button"
                      data-hotkeys="off"
                      onClick={() => item.remappable !== false && setCapturing(item.action)}
                      variant="secondary"
                      size="sm"
                      className="min-w-28"
                      aria-label={`Change ${item.label} shortcut`}
                      aria-describedby={capturing === item.action ? `${item.action}-capture` : undefined}
                      disabled={item.remappable === false}
                    >
                      {capturing === item.action ? <span id={`${item.action}-capture`}>Press a key…</span> : binding ? <Kbd>{formatBinding(binding)}</Kbd> : "Unbound"}
                    </Button>
                    {item.remappable !== false ? (
                      <>
                        <IconButton type="button" onClick={() => state.resetBinding(item.action)} aria-label={`Reset ${item.label}`}><RotateCcw size={15} /></IconButton>
                        <IconButton type="button" onClick={() => state.setBinding(item.action, null)} aria-label={`Unbind ${item.label}`}><X size={15} /></IconButton>
                      </>
                    ) : null}
                  </div>
                } />
              );
            })}
          </InsetList>
        </section>
      )}

      {tab === "Controls" ? <div className="mt-5 flex items-center justify-between gap-3">
        <Button variant="ghost" onClick={state.resetAll}>Reset all</Button>
      </div> : null}
    </Dialog>
  );
}
