import { useEffect, useRef, useState } from "react";
import { isExtensionVersionSupported } from "../lib/geoduels-extension-protocol";

const PING_INTERVAL_MS = 2_000;
const EXPIRY_MS = 5_000;
const INITIAL_DISCOVERY_MS = 750;

export type ExtensionAvailabilityStatus = {
  state: "checking" | "ready" | "missing" | "outdated";
  version: string | null;
};

export function useExtensionAvailability() {
  const [status, setStatus] = useState<ExtensionAvailabilityStatus>({
    state: "checking",
    version: null,
  });
  const lastSeenRef = useRef(0);

  useEffect(() => {
    const ping = () => {
      window.postMessage(
        { source: "geoduels-app", version: 1, type: "extension_ping" },
        window.location.origin,
      );
      if (
        lastSeenRef.current > 0 &&
        Date.now() - lastSeenRef.current > EXPIRY_MS
      ) {
        setStatus({ state: "missing", version: null });
      }
    };
    const onMessage = (event: MessageEvent<unknown>) => {
      if (
        event.source !== window ||
        event.origin !== window.location.origin ||
        !event.data ||
        typeof event.data !== "object"
      ) {
        return;
      }
      const message = event.data as Record<string, unknown>;
      if (
        message.source === "geoduels-extension" &&
        message.version === 1 &&
        message.type === "extension_ready"
      ) {
        const extensionVersion =
          typeof message.extensionVersion === "string"
            ? message.extensionVersion
            : null;
        lastSeenRef.current = Date.now();
        setStatus({
          state: isExtensionVersionSupported(extensionVersion)
            ? "ready"
            : "outdated",
          version: extensionVersion,
        });
      }
    };

    window.addEventListener("message", onMessage);
    ping();
    const timer = window.setInterval(ping, PING_INTERVAL_MS);
    const discoveryTimer = window.setTimeout(() => {
      if (lastSeenRef.current === 0) setStatus({ state: "missing", version: null });
    }, INITIAL_DISCOVERY_MS);
    return () => {
      window.removeEventListener("message", onMessage);
      window.clearInterval(timer);
      window.clearTimeout(discoveryTimer);
    };
  }, []);

  return status;
}
