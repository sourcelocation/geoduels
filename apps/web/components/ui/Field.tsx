import * as React from "react";

import { cn } from "../../lib/cn";

type FieldProps = {
  label?: string;
  error?: string;
  helper?: string;
  children: React.ReactNode;
  className?: string;
  htmlFor?: string;
};

export function Field({ label, error, helper, children, className, htmlFor }: FieldProps) {
  const child =
    React.Children.count(children) === 1 && React.isValidElement(children)
      ? children
      : null;
  const controlId = htmlFor ?? (child?.props as { id?: string } | undefined)?.id;

  return (
    <div className={cn("grid gap-1.5", className)}>
      {label ? (
        <label htmlFor={controlId} className="text-xs font-bold uppercase tracking-[0.12em] text-slate-500">
          {label}
        </label>
      ) : null}
      {children}
      {error ? (
        <span className="text-xs font-semibold text-red-300">{error}</span>
      ) : helper ? (
        <span className="text-xs text-slate-500">{helper}</span>
      ) : null}
    </div>
  );
}
