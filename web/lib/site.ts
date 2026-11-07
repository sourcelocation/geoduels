import { createContext, useContext } from "react";

const SiteURLContext = createContext("");

/** Set from cached pages' props (lib/cached-page.ts). */
export const SiteURLProvider = SiteURLContext.Provider;

export function useSiteURL() {
  return useContext(SiteURLContext);
}
