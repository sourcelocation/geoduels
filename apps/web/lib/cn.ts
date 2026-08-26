import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

const twMerge = extendTailwindMerge({
  extend: {
    theme: {
      radius: ["control", "inset", "navigation", "panel", "modal", "pill"],
      // Keep semantic type utilities intact when they are combined with
      // semantic text-color utilities (e.g. `text-body text-content-primary`).
      text: [
        "body",
        "body-sm",
        "label",
        "caption",
        "heading-sm",
        "heading-md",
        "heading-lg",
        "display-md",
        "display-lg",
        "hud-label",
        "hud-value",
        "hud-countdown",
        "hud-countdown-lg",
      ],
    },
  },
});

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
