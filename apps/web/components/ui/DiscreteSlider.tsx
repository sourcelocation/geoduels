import * as React from "react";

import { cn } from "../../lib/cn";

export type DiscreteSliderOption<T extends string> = {
  value: T;
  label: string;
};

export type DiscreteSliderProps<T extends string> = {
  "aria-label": string;
  className?: string;
  disabled?: boolean;
  options: readonly DiscreteSliderOption<T>[];
  value: T;
  onValueChange: (value: T) => void;
};

export function DiscreteSlider<T extends string>({
  "aria-label": ariaLabel,
  className,
  disabled = false,
  options,
  value,
  onValueChange,
}: DiscreteSliderProps<T>) {
  const selectedIndex = Math.max(0, options.findIndex((option) => option.value === value));
  const max = Math.max(0, options.length - 1);
  const progress = max === 0 ? 0 : (selectedIndex / max) * 100;

  if (options.length === 0) return null;

  return (
    <div className={cn("grid gap-2", disabled && "opacity-50", className)}>
      <div className="relative flex h-5 items-center">
        <div className="absolute inset-x-0 h-1.5 overflow-hidden rounded-full bg-surface-inset">
          <div className="h-full rounded-full bg-status-success" style={{ width: `${progress}%` }} />
        </div>
        <input
          type="range"
          aria-label={ariaLabel}
          aria-valuetext={options[selectedIndex]?.label}
          className="relative m-0 h-5 w-full cursor-pointer appearance-none bg-transparent disabled:cursor-not-allowed [&::-moz-range-thumb]:h-4 [&::-moz-range-thumb]:w-4 [&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:border-0 [&::-moz-range-thumb]:bg-status-success [&::-moz-range-track]:bg-transparent [&::-webkit-slider-runnable-track]:h-1.5 [&::-webkit-slider-runnable-track]:bg-transparent [&::-webkit-slider-thumb]:-mt-[5px] [&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:w-4 [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-status-success"
          min={0}
          max={max}
          step={1}
          value={selectedIndex}
          disabled={disabled}
          onChange={(event) => {
            const option = options[Number(event.target.value)];
            if (option) onValueChange(option.value);
          }}
        />
      </div>
      <div className="flex justify-between gap-1 text-label font-semibold text-content-secondary">
        {options.map((option, index) => (
          <button
            key={option.value}
            type="button"
            className={cn(
              "min-w-0 truncate text-center transition hover:text-content-primary disabled:cursor-not-allowed",
              index === selectedIndex && "text-content-primary",
            )}
            disabled={disabled}
            onClick={() => onValueChange(option.value)}
            aria-label={`Set ${ariaLabel.toLowerCase()} to ${option.label}`}
          >
            {option.label}
          </button>
        ))}
      </div>
    </div>
  );
}
