import { cn } from "../../lib/cn";

export type TabItem<T extends string> = { id: T; label: string; disabled?: boolean };

type TabsProps<T extends string> = {
  value: T;
  items: Array<TabItem<T>>;
  onChange: (value: T) => void;
  className?: string;
  appearance?: "tabs" | "segmented";
  "aria-label"?: string;
};

/** Small controlled choice set. Use for views/options, not page navigation. */
export function Tabs<T extends string>({ value, items, onChange, className, appearance = "tabs", "aria-label": ariaLabel = "Options" }: TabsProps<T>) {
  const move = (id: T, direction: 1 | -1) => {
    const enabled = items.filter((item) => !item.disabled);
    if (!enabled.length) return;
    const current = enabled.findIndex((item) => item.id === id);
    onChange(enabled[(current + direction + enabled.length) % enabled.length].id);
  };
  return (
    <div role="group" aria-label={ariaLabel} className={cn(appearance === "segmented" ? "flex" : "grid gap-2", "rounded-lg border border-border-default bg-surface-grouped p-1", className)}>
      {items.map((item) => (
        <button
          key={item.id}
          type="button"
          aria-pressed={value === item.id}
          disabled={item.disabled}
          onClick={() => onChange(item.id)}
          onKeyDown={(event) => {
            if (event.key === "ArrowRight" || event.key === "ArrowDown") { event.preventDefault(); move(item.id, 1); }
            if (event.key === "ArrowLeft" || event.key === "ArrowUp") { event.preventDefault(); move(item.id, -1); }
          }}
          className={cn(
            "min-h-9 flex-1 whitespace-nowrap rounded-md px-3 text-label font-strong transition disabled:cursor-not-allowed disabled:opacity-50",
            value === item.id
              ? "bg-content-primary text-content-inverse shadow-elev-1"
              : "text-content-secondary hover:bg-surface-fill hover:text-content-primary",
          )}
        >
          {item.label}
        </button>
      ))}
    </div>
  );
}
