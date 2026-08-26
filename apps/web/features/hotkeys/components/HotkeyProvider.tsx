import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { getRuntimeConfig } from "../../../lib/runtime-config";
import { setSfxMuted } from "../../../lib/audio/sfx-preferences";
import { useAuthState } from "../../auth/components/AuthProvider";
import { bindingKey, DEFAULT_HOTKEY_PREFERENCES } from "../model/defaults";
import { eventMatchesBinding, isEditableTarget } from "../lib/keyboard-event";
import { fetchPreferences, patchPreferences } from "../lib/preferences-client";
import { loadLocalPreferences, normalizePreferences, saveLocalPreferences } from "../lib/storage";
import type { HotkeyAction, HotkeyPreferences, HotkeyRegistration, KeyBinding } from "../model/types";
import HotkeySettings from "./HotkeySettings";

type HotkeyContextValue = {
  preferences: HotkeyPreferences;
  register: (registration: HotkeyRegistration) => () => void;
  setBinding: (action: HotkeyAction, binding: KeyBinding | null) => void;
  resetBinding: (action: HotkeyAction) => void;
  resetAll: () => void;
  setAudioMuted: (muted: boolean) => void;
  setSfxMuted: (muted: boolean) => void;
  settingsOpen: boolean;
  setSettingsOpen: (open: boolean) => void;
  saveStatus: "local" | "saving" | "saved" | "error";
};

const HotkeyContext = createContext<HotkeyContextValue | null>(null);
const scopePriority = { global: 0, lobby: 1, gameplay: 2, modal: 3, capture: 4 };

export function HotkeyProvider({ children }: { children: ReactNode }) {
  const [preferences, setPreferencesState] = useState<HotkeyPreferences>(() => loadLocalPreferences());
  const preferencesRef = useRef(preferences);
  const registrations = useRef(new Map<number, HotkeyRegistration>());
  const registrationId = useRef(0);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [saveStatus, setSaveStatus] = useState<HotkeyContextValue["saveStatus"]>("local");
  const syncRef = useRef<{ token: string; revision: number } | null>(null);
  const hydratedRef = useRef(false);

  const setPreferences = useCallback((next: HotkeyPreferences) => {
    preferencesRef.current = next;
    setPreferencesState(next);
    saveLocalPreferences(next);
  }, []);
  const auth = useAuthState();

  useEffect(() => {
    setSfxMuted(preferences.sfxMuted);
  }, [preferences.sfxMuted]);

  useEffect(() => {
    if (auth.status === "bootstrapping") return;
    let cancelled = false;
    void (async () => {
      const session = auth.session;
      if (cancelled || !session?.accessToken) {
        hydratedRef.current = true;
        return;
      }
      try {
        const remote = await fetchPreferences(getRuntimeConfig(), session.accessToken);
        if (cancelled) return;
        const remotePreferences = normalizePreferences(remote.preferences);
        const hasRemoteCustomization = remote.revision > 0;
        syncRef.current = { token: session.accessToken, revision: remote.revision };
        if (hasRemoteCustomization) setPreferences(remotePreferences);
        hydratedRef.current = true;
        if (!hasRemoteCustomization) {
          const saved = await patchPreferences(getRuntimeConfig(), session.accessToken, preferencesRef.current, remote.revision);
          syncRef.current.revision = saved.revision;
        }
        setSaveStatus("saved");
      } catch {
        hydratedRef.current = true;
        setSaveStatus("error");
      }
    })();
    return () => { cancelled = true; };
  }, [auth.session, auth.status, setPreferences]);

  useEffect(() => {
    if (!hydratedRef.current || !syncRef.current) return;
    setSaveStatus("saving");
    const timer = window.setTimeout(async () => {
      const sync = syncRef.current;
      if (!sync) return;
      try {
        const saved = await patchPreferences(getRuntimeConfig(), sync.token, preferencesRef.current, sync.revision);
        sync.revision = saved.revision;
        setSaveStatus("saved");
      } catch {
        setSaveStatus("error");
      }
    }, 500);
    return () => window.clearTimeout(timer);
  }, [preferences]);

  const register = useCallback((registration: HotkeyRegistration) => {
    const id = ++registrationId.current;
    registrations.current.set(id, registration);
    return () => {
      registrations.current.delete(id);
    };
  }, []);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.isComposing) return;
      const matches = [...registrations.current.values()]
        .filter((registration) => registration.enabled !== false)
        .filter((registration) => !event.repeat || registration.allowRepeat)
        .filter((registration) => registration.allowWhileTyping || !isEditableTarget(event.target))
        .filter((registration) => (preferencesRef.current.bindings[registration.action] || []).some((binding) => eventMatchesBinding(event, binding)))
        .sort((a, b) => scopePriority[b.scope || "global"] - scopePriority[a.scope || "global"]);
      const selected = matches[0];
      if (!selected) return;
      if (document.querySelector('[aria-modal="true"]') && (selected.scope || "global") !== "modal" && (selected.scope || "global") !== "capture") return;
      event.preventDefault();
      selected.run(event);
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  useEffect(() => register({
    action: "display.fullscreen",
    scope: "global",
    run: () => {
      if (document.fullscreenElement) void document.exitFullscreen();
      else void document.documentElement.requestFullscreen().catch(() => undefined);
    },
  }), [register]);

  useEffect(() => register({
    action: "help.shortcuts",
    scope: "global",
    run: () => setSettingsOpen(true),
  }), [register]);

  useEffect(() => register({
    action: "audio.toggleMute",
    scope: "global",
    run: () => setPreferences({
      ...preferencesRef.current,
      audioMuted: !preferencesRef.current.audioMuted,
    }),
  }), [register, setPreferences]);

  const value = useMemo<HotkeyContextValue>(() => ({
    preferences,
    register,
    setBinding: (action, binding) => {
      if (binding) {
        const duplicate = Object.entries(preferencesRef.current.bindings).find(([other, bindings]) =>
          other !== action && bindings.some((item) => bindingKey(item) === bindingKey(binding)));
        if (duplicate) {
          setPreferences({
            ...preferencesRef.current,
            bindings: {
              ...preferencesRef.current.bindings,
              [duplicate[0]]: [],
              [action]: [binding],
            },
          });
          return;
        }
      }
      setPreferences({
        ...preferencesRef.current,
        bindings: { ...preferencesRef.current.bindings, [action]: binding ? [binding] : [] },
      });
    },
    resetBinding: (action) => setPreferences({
      ...preferencesRef.current,
      bindings: {
        ...preferencesRef.current.bindings,
        [action]: DEFAULT_HOTKEY_PREFERENCES.bindings[action],
      },
    }),
    resetAll: () => setPreferences(normalizePreferences(DEFAULT_HOTKEY_PREFERENCES)),
    setAudioMuted: (audioMuted) => setPreferences({ ...preferencesRef.current, audioMuted }),
    setSfxMuted: (sfxMuted) => setPreferences({ ...preferencesRef.current, sfxMuted }),
    settingsOpen,
    setSettingsOpen,
    saveStatus,
  }), [preferences, register, saveStatus, setPreferences, settingsOpen]);

  return (
    <HotkeyContext.Provider value={value}>
      {children}
      {settingsOpen ? <HotkeySettings onClose={() => setSettingsOpen(false)} /> : null}
    </HotkeyContext.Provider>
  );
}

export function useHotkeys() {
  const context = useContext(HotkeyContext);
  if (!context) throw new Error("useHotkeys must be used inside HotkeyProvider");
  return context;
}

export function useOptionalHotkeys() {
  return useContext(HotkeyContext);
}
