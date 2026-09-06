import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import vm from "node:vm";
import { describe, expect, it, vi } from "vitest";

const script = readFileSync(
  resolve(
    dirname(fileURLToPath(import.meta.url)),
    "../../../../../extensions/geoduels-enhancer/src/google-street-view.js",
  ),
  "utf8",
);

type StyleElement = {
  id: string;
  textContent: string;
  remove: () => void;
};

class Panorama {
  setOptions = vi.fn();
  set = vi.fn();
  constructor(public element: unknown, public options: unknown) {}
}

function loadGoogleStreetViewScript(referrer: string, ancestorOrigins?: string[]) {
  const posted: Array<{ data: unknown; targetOrigin: string }> = [];
  const styles = new Map<string, StyleElement>();
  const parent = {
    postMessage(data: unknown, targetOrigin: string) {
      posted.push({ data, targetOrigin });
    },
  };
  const messageListeners: Array<(event: MessageEvent) => void> = [];
  const appendStyle = (node: StyleElement) => {
    if (node.id) styles.set(node.id, node);
  };
  const iframeWindow: {
    top: typeof parent;
    location: { search: string; hash: string; ancestorOrigins?: string[] };
    addEventListener: (type: string, handler: (event: MessageEvent) => void) => void;
    document: {
      referrer: string;
      addEventListener: () => void;
      getElementById: (id: string) => StyleElement | null;
      createElement: () => StyleElement;
      head: { appendChild: (node: StyleElement) => void };
      documentElement: { appendChild: (node: StyleElement) => void };
    };
    google?: { maps: { StreetViewPanorama: typeof Panorama } };
    setTimeout: typeof setTimeout;
  } = {
    top: parent,
    location: { search: "", hash: "", ancestorOrigins },
    addEventListener(type, handler) {
      if (type === "message") messageListeners.push(handler);
    },
    document: {
      referrer,
      addEventListener() {},
      getElementById: (id) => styles.get(id) ?? null,
      createElement: () => {
        const node: StyleElement = {
          id: "",
          textContent: "",
          remove() {
            if (node.id) styles.delete(node.id);
          },
        };
        return node;
      },
      head: { appendChild: appendStyle },
      documentElement: { appendChild: appendStyle },
    },
    setTimeout,
  };

  vm.runInNewContext(script, {
    window: iframeWindow,
    document: iframeWindow.document,
    location: iframeWindow.location,
    addEventListener: iframeWindow.addEventListener,
    setTimeout,
    URL,
    URLSearchParams,
    Proxy,
    Reflect,
    Object,
    Number,
    Set,
    JSON,
  });

  return {
    posted,
    styles,
    iframeWindow,
    send(origin: string, data: unknown) {
      const event = {
        source: parent,
        origin,
        data,
      } as MessageEvent;
      for (const listener of messageListeners) listener(event);
    },
  };
}

describe("google-street-view content script", () => {
  it("accepts GeoDuels configure messages when the iframe referrer is stripped", () => {
    const { posted, styles, iframeWindow, send } = loadGoogleStreetViewScript("");
    iframeWindow.google = { maps: { StreetViewPanorama: Panorama } };
    const options = { panControl: true, zoomControl: true, clickToGo: true };
    const panorama = new iframeWindow.google.maps.StreetViewPanorama(null, options);
    const original = (panorama as Panorama & { __geoduels: { original: Panorama } }).__geoduels.original;

    expect(styles.size).toBe(0);
    expect(panorama.options).toBe(options);
    expect(original.setOptions).not.toHaveBeenCalled();
    panorama.setOptions(options);
    panorama.set("panControl", true);
    expect(original.setOptions).toHaveBeenLastCalledWith(options);
    expect(original.set).toHaveBeenLastCalledWith("panControl", true);

    send("https://geoduels.io", {
      source: "geoduels-app",
      version: 1,
      type: "configure",
      ruleset: "no_move",
      streetNames: "hidden",
    });

    expect(posted).toEqual([
      {
        data: {
          source: "geoduels-extension",
          version: 1,
          extensionVersion: "0.1.5",
          type: "ready",
          capabilities: { heading: true, roadLabels: true },
        },
        targetOrigin: "*",
      },
      {
        data: {
          source: "geoduels-extension",
          version: 1,
          extensionVersion: "0.1.5",
          type: "configured",
          ruleset: "no_move",
          streetNames: "hidden",
        },
        targetOrigin: "*",
      },
    ]);
    expect(styles.has("geoduels-hidden-native-chrome")).toBe(true);
    expect(original.setOptions).toHaveBeenLastCalledWith(expect.objectContaining({
      panControl: false,
      zoomControl: false,
      clickToGo: false,
      showRoadLabels: false,
    }));
  });

  it("hides Google's native Street View compass and zoom controls", () => {
    const { styles } = loadGoogleStreetViewScript("https://geoduels.io/match/test");
    const css = styles.get("geoduels-hidden-native-chrome")?.textContent ?? "";

    expect(css).toContain(".gm-compass");
    expect(css).toContain(".gm-bundled-control");
    expect(css).toContain("Zoom in");
    expect(css).toContain("Zoom out");
  });

  it("ignores configure messages from untrusted origins even with an empty referrer", () => {
    const { posted, styles, send } = loadGoogleStreetViewScript("");

    send("https://evil.example", {
      source: "geoduels-app",
      version: 1,
      type: "configure",
      ruleset: "no_move",
      streetNames: "hidden",
    });

    expect(posted).toEqual([]);
    expect(styles.size).toBe(0);
  });

  it("leaves untrusted embeds untouched, including when ancestorOrigins overrides the referrer", () => {
    for (const [referrer, ancestors] of [
      ["https://geoduels.io.evil.example", undefined],
      ["https://geoduels.io", ["https://unrelated.example"]],
    ] as const) {
      const { styles, iframeWindow } = loadGoogleStreetViewScript(referrer, ancestors ? [...ancestors] : undefined);
      expect(styles.size).toBe(0);
      expect(Object.getOwnPropertyDescriptor(iframeWindow, "google")).toBeUndefined();
    }
  });

  it("enables GeoDuels embeds with an ancestor origin and no referrer", () => {
    const { styles } = loadGoogleStreetViewScript("", ["https://play.geoduels.io"]);
    expect(styles.has("geoduels-hidden-native-chrome")).toBe(true);
  });
});
