import type { ReactNode } from "react";
import { cn } from "../../lib/cn";
import { Tooltip } from "./Tooltip";

export type TabItem<T extends string> = { id: T; label: string; icon?: ReactNode; disabled?: boolean };

type TabsProps<T extends string> = {
  value: T;
  items: Array<TabItem<T>>;
  onChange: (value: T) => void;
  className?: string;
  appearance?: "tabs" | "segmented" | "segmented-icons";
  "aria-label"?: string;
};

/** Small controlled choice set. Use for views/options, not page navigation. */
export function Tabs<T extends string>({ value, items, onChange, className, appearance = "tabs", "aria-label": ariaLabel = "Options" }: TabsProps<T>) {
  const iconOnly = appearance === "segmented-icons";
  const move = (id: T, direction: 1 | -1) => {
    const enabled = items.filter((item) => !item.disabled);
    if (!enabled.length) return;
    const current = enabled.findIndex((item) => item.id === id);
    onChange(enabled[(current + direction + enabled.length) % enabled.length].id);
  };
  return (
    <div role="group" aria-label={ariaLabel} className={cn(appearance === "tabs" ? "grid gap-2" : "flex gap-1", "rounded-lg border border-border-default bg-surface-grouped p-1", className)}>
      {items.map((item) => {
        const control = (
          <button
          key={item.id}
          type="button"
          aria-label={iconOnly ? item.label : undefined}
          aria-pressed={value === item.id}
          disabled={item.disabled}
          onClick={() => onChange(item.id)}
          onKeyDown={(event) => {
            if (event.key === "ArrowRight" || event.key === "ArrowDown") { event.preventDefault(); move(item.id, 1); }
            if (event.key === "ArrowLeft" || event.key === "ArrowUp") { event.preventDefault(); move(item.id, -1); }
          }}
          className={cn(
            "min-h-9 whitespace-nowrap rounded-md text-label font-strong transition disabled:cursor-not-allowed disabled:opacity-50",
            iconOnly ? "flex h-9 w-9 flex-none items-center justify-center p-0" : "flex-1 px-3",
            value === item.id
              ? "bg-content-primary text-content-inverse shadow-elev-1"
              : "text-content-secondary hover:bg-surface-fill hover:text-content-primary",
          )}
        >
          {iconOnly ? item.icon : item.label}
        </button>
        );
        return iconOnly ? <Tooltip key={item.id} content={item.label}>{control}</Tooltip> : control;
      })}
    </div>
  );
}
