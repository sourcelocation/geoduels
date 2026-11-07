import { useEffect, type ReactNode } from "react";
import { useAuthState } from "../auth/components/AuthProvider";
import { liveConnection } from "./live-connection";

/** Connects everyone, signed in or not, once it's known who they are, and again if that changes. */
export function LiveProvider({ children }: { children: ReactNode }) {
  const auth = useAuthState();
  const known = auth.status !== "bootstrapping";
  useEffect(() => {
    if (!known) return;
    liveConnection.connect();
    return () => liveConnection.disconnect();
  }, [known, auth.userId]);
  return children;
}
