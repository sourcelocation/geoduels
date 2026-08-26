import { useEffect, useRef, useState } from "react";
import AppModalShell from "../../../components/ui/AppModalShell";
import { Button } from "../../../components/ui/button";
import type { HomeOverlaysView } from "../model/types";

declare global {
  interface Window {
    turnstile?: {
      render: (
        container: HTMLElement,
        options: {
          sitekey: string;
          theme?: "dark" | "light" | "auto";
          action?: string;
          callback?: (token: string) => void;
          "expired-callback"?: () => void;
          "error-callback"?: () => void;
        },
      ) => string;
      remove?: (widgetId: string) => void;
      reset?: (widgetId: string) => void;
    };
  }
}

type GuestVerificationOverlayProps = {
  verification: HomeOverlaysView["guestVerification"];
  onToken: (token: string) => void;
  onExpired: (message?: string) => void;
  onCancel: () => void;
};

let turnstileScriptPromise: Promise<void> | null = null;

function loadTurnstileScript() {
  if (typeof window === "undefined") {
    return Promise.resolve();
  }
  if (window.turnstile) {
    return Promise.resolve();
  }
  if (!turnstileScriptPromise) {
    turnstileScriptPromise = new Promise((resolve, reject) => {
      const existing = document.querySelector<HTMLScriptElement>(
        'script[src^="https://challenges.cloudflare.com/turnstile/v0/api.js"]',
      );
      if (existing) {
        existing.addEventListener("load", () => resolve(), { once: true });
        existing.addEventListener("error", () => reject(new Error("load failed")), {
          once: true,
        });
        return;
      }
      const script = document.createElement("script");
      script.src = "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit";
      script.async = true;
      script.defer = true;
      script.addEventListener("load", () => resolve(), { once: true });
      script.addEventListener("error", () => reject(new Error("load failed")), {
        once: true,
      });
      document.head.appendChild(script);
    });
  }
  return turnstileScriptPromise;
}

export default function GuestVerificationOverlay({
  verification,
  onToken,
  onExpired,
  onCancel,
}: GuestVerificationOverlayProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const widgetIdRef = useRef<string | null>(null);
  const callbacksRef = useRef({ onToken, onExpired });
  const [scriptReady, setScriptReady] = useState(false);

  useEffect(() => {
    callbacksRef.current = { onToken, onExpired };
  }, [onExpired, onToken]);

  useEffect(() => {
    if (!verification.open || !verification.siteKey) return;
    let cancelled = false;
    setScriptReady(false);
    loadTurnstileScript()
      .then(() => {
        if (!cancelled) setScriptReady(true);
      })
      .catch(() => {
        if (!cancelled) {
          callbacksRef.current.onExpired("Verification could not load. Try again.");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [verification.open, verification.siteKey, verification.resetKey]);

  useEffect(() => {
    if (
      !verification.open ||
      !verification.siteKey ||
      !scriptReady ||
      !containerRef.current ||
      !window.turnstile
    ) {
      return;
    }
    if (widgetIdRef.current && window.turnstile.remove) {
      window.turnstile.remove(widgetIdRef.current);
      widgetIdRef.current = null;
    }
    containerRef.current.innerHTML = "";
    widgetIdRef.current = window.turnstile.render(containerRef.current, {
      sitekey: verification.siteKey,
      theme: "dark",
      action: "guest_signup",
      callback: (token) => callbacksRef.current.onToken(token),
      "expired-callback": () =>
        callbacksRef.current.onExpired("Verification expired. Try again."),
      "error-callback": () =>
        callbacksRef.current.onExpired("Verification failed. Try again."),
    });
    return () => {
      if (widgetIdRef.current && window.turnstile?.remove) {
        window.turnstile.remove(widgetIdRef.current);
      }
      widgetIdRef.current = null;
    };
  }, [
    scriptReady,
    verification.open,
    verification.resetKey,
    verification.siteKey,
  ]);

  if (!verification.open) {
    return null;
  }

  const title =
    verification.status === "creating"
      ? "Starting guest session..."
      : "Checking your connection...";
  const canCancel = verification.status !== "creating";

  return (
    <AppModalShell
      title={title}
      placement="center"
      showHeader={false}
      zIndexClassName="z-dialog"
      maxWidthClassName="max-w-sm"
      contentClassName="text-center"
    >
      <div className="flex flex-col items-center">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-border-default border-t-action-primary" />
        <h2 className="mt-4 text-heading-sm font-strong">{title}</h2>
        <p className="mt-2 text-body-sm text-content-secondary">
          Guest play needs a quick one-time check.
        </p>
        <div className="mt-5 min-h-[70px] w-full">
          <div ref={containerRef} className="flex justify-center" />
        </div>
        {verification.error ? (
          <p className="mt-3 text-body-sm font-strong text-status-danger">
            {verification.error}
          </p>
        ) : null}
        {canCancel ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={onCancel}
            className="mt-5 uppercase tracking-control"
          >
            Cancel
          </Button>
        ) : null}
      </div>
    </AppModalShell>
  );
}
