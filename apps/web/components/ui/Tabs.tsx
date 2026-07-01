import { cn } from "../../lib/cn";

export type TabItem<T extends string> = {
  id: T;
  label: string;
};

type TabsProps<T extends string> = {
  value: T;
  items: Array<TabItem<T>>;
  onChange: (value: T) => void;
  className?: string;
};

export function Tabs<T extends string>({ value, items, onChange, className }: TabsProps<T>) {
  return (
    <div className={cn("grid gap-2 rounded-lg border border-white/10 bg-black/20 p-1", className)}>
      {items.map((item) => (
        <button
          key={item.id}
          type="button"
          onClick={() => onChange(item.id)}
          className={cn(
            "min-h-9 rounded-md px-3 text-xs font-black uppercase tracking-[0.12em] transition",
            value === item.id
              ? "bg-accentPrimary text-white"
              : "text-inkMuted hover:bg-white/10 hover:text-white",
          )}
        >
          {item.label}
        </button>
      ))}
    </div>
  );
}
