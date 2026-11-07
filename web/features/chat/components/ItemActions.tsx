import { useState } from "react";
import { Button } from "../../../components/ui/button";
import { useItemAction } from "../hooks/use-item-action";
import type { ItemAction } from "../model/types";

export function ItemActions({ actions }: { actions: ItemAction[] }) {
  const run = useItemAction();
  const [busy, setBusy] = useState(false);
  return (
    <>
      {actions.map((action, index) => (
        <Button
          key={action.label}
          type="button"
          size="sm"
          variant={index === 0 ? "primary" : "secondary"}
          disabled={busy}
          onClick={() => {
            setBusy(true);
            void run(action).finally(() => setBusy(false));
          }}
        >
          {action.label}
        </Button>
      ))}
    </>
  );
}
