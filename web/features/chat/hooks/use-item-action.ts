import { useRouter } from "next/router";
import { useCallback } from "react";
import { useAppNotice } from "../../../components/ui/AppNotice";
import { apiFetch, authHeaders, readError } from "../../../lib/http";
import { homeRuntime } from "../../home/state/home-runtime";
import type { ItemAction } from "../model/types";

/** Does what an item's button says. No client code knows what any of them are for. */
export function useItemAction() {
  const router = useRouter();
  const { show } = useAppNotice();
  return useCallback(async (action: ItemAction) => {
    try {
      if (action.post) {
        const session = await homeRuntime.sessionController.ensureFreshSession();
        const response = await apiFetch(action.post, { method: "POST", headers: authHeaders(session?.accessToken) });
        if (!response.ok) throw new Error(await readError(response, "That's no longer available."));
      }
      if (action.open) await router.push(action.open);
    } catch (error) {
      show(error instanceof Error ? error.message : "That's no longer available.");
    }
  }, [router, show]);
}
