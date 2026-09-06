(() => {
  "use strict";
  const EXTENSION_VERSION = "0.1.5";
  const ready = () => {
    window.postMessage({ source: "geoduels-extension", version: 1, extensionVersion: EXTENSION_VERSION, type: "extension_ready" }, location.origin);
  };
  addEventListener("message", (event) => {
    if (
      event.source === window &&
      event.origin === location.origin &&
      event.data?.source === "geoduels-app" &&
      event.data?.version === 1 &&
      event.data?.type === "extension_ping"
    ) ready();
  });
  ready();
})();
