import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import vm from "node:vm";
import { describe, expect, it } from "vitest";

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

function loadGoogleStreetViewScript(referrer: string) {
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
    location: { search: string; hash: string };
    addEventListener: (type: string, handler: (event: MessageEvent) => void) => void;
    document: {
      referrer: string;
      addEventListener: () => void;
      getElementById: (id: string) => StyleElement | null;
      createElement: () => StyleElement;
      head: { appendChild: (node: StyleElement) => void };
      documentElement: { appendChild: (node: StyleElement) => void };
    };
    google?: unknown;
    setTimeout: typeof setTimeout;
  } = {
    top: parent,
    location: { search: "", hash: "" },
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
    const { posted, send } = loadGoogleStreetViewScript("");

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
          extensionVersion: "0.1.4",
          type: "ready",
          capabilities: { heading: true, roadLabels: true },
        },
        targetOrigin: "*",
      },
      {
        data: {
          source: "geoduels-extension",
          version: 1,
          extensionVersion: "0.1.4",
          type: "configured",
          ruleset: "no_move",
          streetNames: "hidden",
        },
        targetOrigin: "*",
      },
    ]);
  });

  it("hides Google's native Street View compass and zoom controls", () => {
    const { styles } = loadGoogleStreetViewScript("");
    const css = styles.get("geoduels-hidden-native-chrome")?.textContent ?? "";

    expect(css).toContain(".gm-compass");
    expect(css).toContain(".gm-bundled-control");
    expect(css).toContain("Zoom in");
    expect(css).toContain("Zoom out");
  });

  it("ignores configure messages from untrusted origins even with an empty referrer", () => {
    const { posted, send } = loadGoogleStreetViewScript("");

    send("https://evil.example", {
      source: "geoduels-app",
      version: 1,
      type: "configure",
      ruleset: "no_move",
      streetNames: "hidden",
    });

    expect(posted).toEqual([]);
  });
});
