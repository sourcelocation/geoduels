export const GEODUELS_EXTENSION_PROTOCOL_VERSION = 1;
export const GEODUELS_EXTENSION_SOURCE = "geoduels-extension";
export const GEODUELS_APP_SOURCE = "geoduels-app";
export const GEODUELS_MIN_EXTENSION_VERSION = "0.1.3";

export type GeoDuelsExtensionCapabilities = {
  heading: boolean;
  roadLabels: boolean;
};

export type GeoDuelsExtensionMessage =
  | {
      source: typeof GEODUELS_EXTENSION_SOURCE;
      version: typeof GEODUELS_EXTENSION_PROTOCOL_VERSION;
      extensionVersion: string;
      type: "ready";
      capabilities: GeoDuelsExtensionCapabilities;
    }
  | {
      source: typeof GEODUELS_EXTENSION_SOURCE;
      version: typeof GEODUELS_EXTENSION_PROTOCOL_VERSION;
      extensionVersion: string;
      type: "pov";
      heading: number;
    }
  | {
      source: typeof GEODUELS_EXTENSION_SOURCE;
      version: typeof GEODUELS_EXTENSION_PROTOCOL_VERSION;
      extensionVersion: string;
      type: "configured";
      ruleset: "moving" | "no_move" | "nmpz";
      streetNames: "shown" | "hidden";
    };

export type GeoDuelsExtensionConfigMessage = {
  source: typeof GEODUELS_APP_SOURCE;
  version: typeof GEODUELS_EXTENSION_PROTOCOL_VERSION;
  type: "configure";
  ruleset: "moving" | "no_move" | "nmpz";
  streetNames: "shown" | "hidden";
};

export function isGeoDuelsExtensionMessage(
  value: unknown,
): value is GeoDuelsExtensionMessage {
  if (!value || typeof value !== "object") return false;
  const message = value as Partial<GeoDuelsExtensionMessage>;
  if (
    message.source !== GEODUELS_EXTENSION_SOURCE ||
    message.version !== GEODUELS_EXTENSION_PROTOCOL_VERSION ||
    !isExtensionVersionSupported(message.extensionVersion)
  ) {
    return false;
  }
  if (message.type === "ready") {
    return (
      typeof message.capabilities?.heading === "boolean" &&
      typeof message.capabilities?.roadLabels === "boolean"
    );
  }
  if (message.type === "pov") {
    return typeof message.heading === "number";
  }
  return (
    message.type === "configured" &&
    (message.ruleset === "moving" ||
      message.ruleset === "no_move" ||
      message.ruleset === "nmpz") &&
    (message.streetNames === "shown" || message.streetNames === "hidden")
  );
}

export function compareExtensionVersions(a: string, b: string) {
  const left = a.split(".").map((part) => Number.parseInt(part, 10));
  const right = b.split(".").map((part) => Number.parseInt(part, 10));
  for (let index = 0; index < Math.max(left.length, right.length); index += 1) {
    const diff = (left[index] || 0) - (right[index] || 0);
    if (diff !== 0) return diff;
  }
  return 0;
}

export function isExtensionVersionSupported(version: unknown) {
  return (
    typeof version === "string" &&
    /^\d+\.\d+\.\d+$/.test(version) &&
    compareExtensionVersions(version, GEODUELS_MIN_EXTENSION_VERSION) >= 0
  );
}

export function isTrustedGoogleMapsOrigin(origin: string) {
  try {
    const hostname = new URL(origin).hostname;
    return hostname === "google.com" || hostname.endsWith(".google.com");
  } catch {
    return false;
  }
}
