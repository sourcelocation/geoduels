import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type RefObject,
} from "react";
import {
  GEODUELS_APP_SOURCE,
  GEODUELS_EXTENSION_PROTOCOL_VERSION,
  GEODUELS_EXTENSION_SOURCE,
  isGeoDuelsExtensionMessage,
  isExtensionVersionSupported,
  isTrustedGoogleMapsOrigin,
  type GeoDuelsExtensionCapabilities,
  type GeoDuelsExtensionConfigMessage,
} from "../lib/geoduels-extension-protocol";

export function useGeoDuelsExtension(
  streetViewFrameRef: RefObject<HTMLIFrameElement>,
  streetViewSrc: string,
  ruleset: "moving" | "no_move" | "nmpz",
  streetNames: "shown" | "hidden",
) {
  const [capabilities, setCapabilities] =
    useState<GeoDuelsExtensionCapabilities | null>(null);
  const [heading, setHeading] = useState(0);
  const headingRef = useRef(0);
  const [configured, setConfigured] = useState(false);
  const [unsupportedVersion, setUnsupportedVersion] = useState<string | null>(null);
  const [timedOut, setTimedOut] = useState(false);

  const sendConfiguration = useCallback(() => {
    const target = streetViewFrameRef.current?.contentWindow;
    if (!target) return;
    const message: GeoDuelsExtensionConfigMessage = {
      source: GEODUELS_APP_SOURCE,
      version: GEODUELS_EXTENSION_PROTOCOL_VERSION,
      type: "configure",
      ruleset,
      streetNames,
    };
    target.postMessage(message, "https://www.google.com");
  }, [ruleset, streetNames, streetViewFrameRef]);

  const retry = useCallback(() => {
    setCapabilities(null);
    setConfigured(false);
    setUnsupportedVersion(null);
    setTimedOut(false);
  }, []);

  useEffect(() => {
    retry();
  }, [streetViewSrc, ruleset, streetNames, retry]);

  useEffect(() => {
    const onMessage = (event: MessageEvent<unknown>) => {
      if (
        event.source !== streetViewFrameRef.current?.contentWindow ||
        !isTrustedGoogleMapsOrigin(event.origin) ||
        !event.data ||
        typeof event.data !== "object"
      ) {
        return;
      }
      const rawMessage = event.data as Record<string, unknown>;
      if (
        rawMessage.source === GEODUELS_EXTENSION_SOURCE &&
        rawMessage.version === GEODUELS_EXTENSION_PROTOCOL_VERSION &&
        typeof rawMessage.type === "string" &&
        !isExtensionVersionSupported(rawMessage.extensionVersion)
      ) {
        setUnsupportedVersion(
          typeof rawMessage.extensionVersion === "string"
            ? rawMessage.extensionVersion
            : null,
        );
        return;
      }
      if (!isGeoDuelsExtensionMessage(event.data)) {
        return;
      }
      const message = event.data;
      setUnsupportedVersion(null);

      if (message.type === "ready") {
        setCapabilities(message.capabilities);
        window.setTimeout(sendConfiguration, 0);
        return;
      }

      if (message.type === "configured") {
        setConfigured(
          message.ruleset === ruleset &&
            message.streetNames === streetNames,
        );
        return;
      }

      {
        const current = headingRef.current;
        const normalizedCurrent = ((current % 360) + 360) % 360;
        let delta = message.heading - normalizedCurrent;
        if (delta > 180) delta -= 360;
        if (delta < -180) delta += 360;
        headingRef.current = current + delta;
        setHeading(headingRef.current);
      }
    };

    window.addEventListener("message", onMessage);
    return () => window.removeEventListener("message", onMessage);
  }, [ruleset, sendConfiguration, streetNames, streetViewFrameRef]);

  // Actively drive the handshake instead of waiting for the extension's
  // one-shot `ready` message. That message can arrive before this hook's
  // listener is attached (slow hydration / slow devices), in which case it is
  // lost and nothing ever retries — leaving the player stuck on "Preparing
  // official extension…" forever. Re-sending `configure` on an interval makes a
  // missed `ready` or a dropped `configure` self-correct, and a timeout
  // surfaces an actionable error instead of hanging indefinitely.
  useEffect(() => {
    if (configured || unsupportedVersion) return;
    let attempts = 0;
    const maxAttempts = 40; // ~12s at 300ms
    sendConfiguration();
    const interval = window.setInterval(() => {
      attempts += 1;
      if (attempts >= maxAttempts) {
        window.clearInterval(interval);
        setTimedOut(true);
        return;
      }
      sendConfiguration();
    }, 300);
    return () => window.clearInterval(interval);
  }, [configured, unsupportedVersion, sendConfiguration]);

  return {
    available: capabilities !== null,
    capabilities,
    heading,
    configured,
    unsupportedVersion,
    timedOut,
    retry,
  };
}
