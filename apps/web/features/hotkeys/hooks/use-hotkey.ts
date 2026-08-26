import { useEffect, useRef } from "react";
import { useOptionalHotkeys } from "../components/HotkeyProvider";
import type { HotkeyRegistration } from "../model/types";

export function useHotkey(registration: HotkeyRegistration) {
  const latest = useRef(registration);
  latest.current = registration;
  const context = useOptionalHotkeys();
  const register = context?.register;
  useEffect(() => register?.({
    ...registration,
    run: (event) => latest.current.run(event),
  }), [
    register,
    registration.action,
    registration.scope,
    registration.enabled,
    registration.allowRepeat,
    registration.allowWhileTyping,
  ]);
}
