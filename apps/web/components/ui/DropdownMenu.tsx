import Link from "next/link";
import * as React from "react";

import { cn } from "../../lib/cn";
import { Popover } from "./Popover";
import { Separator } from "./Separator";

export type DropdownMenuItem = {
  label: React.ReactNode;
  onSelect?: () => void;
  href?: string;
  disabled?: boolean;
  destructive?: boolean;
  separatorBefore?: boolean;
};

export function DropdownMenu({
  trigger,
  items,
  align = "end",
  className,
}: {
  trigger: React.ReactElement;
  items: DropdownMenuItem[];
  align?: "start" | "end";
  className?: string;
}) {
  return (
    <Popover placement={`bottom-${align}`} className={cn("min-w-44 p-1", className)} content={
      <div role="menu" className="grid gap-1">
        {items.map((item, index) => (
          <React.Fragment key={index}>
            {item.separatorBefore ? <Separator className="my-1" /> : null}
            {item.href ? (
              <Link role="menuitem" href={item.href} className={cn("rounded-md px-3 py-2 text-body-sm font-semibold text-content-primary transition hover:bg-surface-fill", item.destructive && "text-status-danger", item.disabled && "pointer-events-none opacity-50")}>
                {item.label}
              </Link>
            ) : (
              <button type="button" role="menuitem" disabled={item.disabled} onClick={item.onSelect} className={cn("rounded-md px-3 py-2 text-left text-body-sm font-semibold text-content-primary transition hover:bg-surface-fill disabled:cursor-not-allowed disabled:opacity-50", item.destructive && "text-status-danger")}>
                {item.label}
              </button>
            )}
          </React.Fragment>
        ))}
      </div>
    }>
      {trigger}
    </Popover>
  );
}
