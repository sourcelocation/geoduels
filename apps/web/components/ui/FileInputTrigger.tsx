import * as React from "react";

import { cn } from "../../lib/cn";

type FileInputTriggerProps = Omit<React.InputHTMLAttributes<HTMLInputElement>, "children" | "className" | "type"> & {
  children: React.ReactNode;
  className?: string;
};

/** Accessible visible label paired with an intentionally hidden native file picker. */
export const FileInputTrigger = React.forwardRef<HTMLInputElement, FileInputTriggerProps>(function FileInputTrigger(
  { children, className, ...inputProps },
  ref,
) {
  return (
    <label className={cn("cursor-pointer", className)}>
      {children}
      <input ref={ref} type="file" className="hidden" {...inputProps} />
    </label>
  );
});
