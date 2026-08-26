import { Outfit } from "next/font/google";

/** The document-owned app family, including content rendered through portals. */
export const outfit = Outfit({
  subsets: ["latin"],
  variable: "--font-body",
  weight: ["400", "500", "600", "700", "800"],
  display: "swap",
});
