(() => {
  "use strict";
  if (window.top === window) return;
  const VERSION = 1, EXTENSION_VERSION = "0.1.3", EXTENSION = "geoduels-extension", APP = "geoduels-app";
  const STYLE_ID = "geoduels-hidden-streetnames";
  const instances = new Set();
  const initialPano = new URLSearchParams(location.search).get("pano");
  let readySent = false;
  let config = readConfig() || { ruleset: "moving", streetNames: "shown" };
  function allowed(origin) {
    try {
      const host = new URL(origin).hostname;
      return host === "geoduels.io" || host.endsWith(".geoduels.io") || host === "localhost" || host === "127.0.0.1";
    } catch {
      return false;
    }
  }
  if (!allowed(document.referrer)) return;
  function normalizeRuleset(value) {
    return value === "no_move" || value === "nmpz" ? value : "moving";
  }
  function normalizeStreetNames(value) {
    return value === "hidden" ? "hidden" : "shown";
  }
  function readConfig() {
    try {
      const raw = new URLSearchParams(location.hash.slice(1)).get("geoduels");
      const value = raw ? JSON.parse(raw) : null;
      return value?.version === VERSION
        ? { ruleset: normalizeRuleset(value.ruleset), streetNames: normalizeStreetNames(value.streetNames) }
        : null;
    } catch {
      return null;
    }
  }
  function post(message) {
    window.top.postMessage({ source: EXTENSION, version: VERSION, extensionVersion: EXTENSION_VERSION, ...message }, "*");
  }
  function syncAddressStyle() {
    let style = document.getElementById(STYLE_ID);
    if (config.streetNames !== "hidden") {
      style?.remove();
      return;
    }
    if (!style) {
      style = document.createElement("style");
      style.id = STYLE_ID;
      (document.head || document.documentElement).appendChild(style);
    }
    style.textContent = ".gm-iv-address,.gm-iv-address-description,.gm-iv-address-link,.gm-iv-profile-url{display:none!important}";
  }
  function options(extra) {
    const hidden = config.streetNames === "hidden";
    const noMove = config.ruleset === "no_move";
    return { ...(extra || {}), addressControl: !hidden, showRoadLabels: !hidden, clickToGo: !noMove, linksControl: !noMove };
  }
  function latLng(value) {
    if (!value) return null;
    const lat = typeof value.lat === "function" ? value.lat() : value.lat;
    const lng = typeof value.lng === "function" ? value.lng() : value.lng;
    return Number.isFinite(lat) && Number.isFinite(lng) ? { lat, lng } : null;
  }
  function samePosition(a, b) {
    const one = latLng(a), two = latLng(b);
    return !!one && !!two && Math.abs(one.lat - two.lat) < 0.0000001 && Math.abs(one.lng - two.lng) < 0.0000001;
  }
  function isMovementKey(event) {
    if (event.altKey || event.ctrlKey || event.metaKey) return false;
    const key = event.key.length === 1 ? event.key.toLowerCase() : event.key;
    return ["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight", "w", "a", "s", "d"].includes(key);
  }
  function blockMovementKey(event) {
    if (config.ruleset !== "no_move" || !isMovementKey(event)) return;
    event.preventDefault();
    event.stopImmediatePropagation();
  }
  function rememberSpawn(panorama) {
    const state = panorama.__geoduels;
    if (!state.spawnPano) state.spawnPano = panorama.getPano?.() || initialPano || null;
    if (!state.spawnPosition) state.spawnPosition = panorama.getPosition?.() || null;
  }
  function reportHeading(panorama) {
    const heading = panorama.getPov?.().heading;
    if (Number.isFinite(heading)) post({ type: "pov", heading });
  }
  function apply(panorama) {
    const state = panorama.__geoduels;
    state.original.setOptions?.call(panorama, options());
    syncAddressStyle();
    if (config.ruleset !== "no_move") return;
    rememberSpawn(panorama);
    if (state.spawnPano && panorama.getPano?.() !== state.spawnPano) state.original.setPano?.call(panorama, state.spawnPano);
    const current = panorama.getPosition?.();
    if (state.spawnPosition && current && !samePosition(current, state.spawnPosition)) state.original.setPosition?.call(panorama, state.spawnPosition);
  }
  function protectedValue(key, value) {
    if (config.streetNames === "hidden" && (key === "showRoadLabels" || key === "addressControl")) return false;
    if (config.ruleset === "no_move" && (key === "clickToGo" || key === "linksControl")) return false;
    return value;
  }
  function register(panorama) {
    if (!panorama || panorama.__geoduels) return panorama;
    const original = {
      set: panorama.set,
      setOptions: panorama.setOptions,
      setPano: panorama.setPano,
      setPosition: panorama.setPosition,
    };
    panorama.__geoduels = { original, spawnPano: null, spawnPosition: null };
    panorama.set = function (key, value) {
      return original.set?.call(this, key, protectedValue(key, value));
    };
    panorama.setOptions = function (value) {
      return original.setOptions?.call(this, options(value));
    };
    panorama.setPano = function (value) {
      const spawn = panorama.__geoduels.spawnPano;
      return original.setPano?.call(this, config.ruleset === "no_move" && spawn ? spawn : value);
    };
    panorama.setPosition = function (value) {
      const spawn = panorama.__geoduels.spawnPosition;
      const locked = config.ruleset === "no_move" && spawn && !samePosition(value, spawn);
      return original.setPosition?.call(this, locked ? spawn : value);
    };
    instances.add(panorama);
    rememberSpawn(panorama);
    apply(panorama);
    panorama.addListener?.("pov_changed", () => reportHeading(panorama));
    panorama.addListener?.("status_changed", () => {
      rememberSpawn(panorama);
      apply(panorama);
      reportHeading(panorama);
    });
    panorama.addListener?.("pano_changed", () => apply(panorama));
    panorama.addListener?.("position_changed", () => apply(panorama));
    setTimeout(() => reportHeading(panorama), 0);
    return panorama;
  }
  function wrapConstructor(Constructor) {
    if (typeof Constructor !== "function" || Constructor.__geoduelsWrapped) return Constructor;
    const wrapped = new Proxy(Constructor, {
      construct(target, args, newTarget) {
        const next = [...args];
        next[1] = options(next[1]);
        return register(Reflect.construct(target, next, newTarget));
      },
    });
    wrapped.__geoduelsWrapped = true;
    if (!readySent) {
      readySent = true;
      [0, 250, 1000].forEach((delay) => setTimeout(() => post({ type: "ready", capabilities: { heading: true, roadLabels: true } }), delay));
    }
    return wrapped;
  }
  function hook(object, key, callback) {
    let value = object[key];
    const assign = (next) => {
      value = callback(next) || next;
    };
    if (value !== undefined) assign(value);
    try {
      Object.defineProperty(object, key, { configurable: true, get: () => value, set: assign });
    } catch {
      assign(value);
    }
  }
  function hookGoogle(google) {
    if (!google || typeof google !== "object") return;
    hook(google, "maps", (maps) => {
      if (maps && typeof maps === "object") hook(maps, "StreetViewPanorama", wrapConstructor);
    });
  }
  addEventListener("message", (event) => {
    if (
      event.source !== window.top ||
      !allowed(event.origin) ||
      event.data?.source !== APP ||
      event.data?.version !== VERSION ||
      event.data?.type !== "configure"
    ) return;
    config = { ruleset: normalizeRuleset(event.data.ruleset), streetNames: normalizeStreetNames(event.data.streetNames) };
    syncAddressStyle();
    instances.forEach(apply);
    instances.forEach(reportHeading);
    post({ type: "configured", ...config });
  });
  ["keydown", "keyup"].forEach((type) => {
    window.addEventListener(type, blockMovementKey, true);
    document.addEventListener(type, blockMovementKey, true);
  });
  syncAddressStyle();
  hook(window, "google", hookGoogle);
})();
