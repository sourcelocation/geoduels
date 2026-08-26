import * as React from "react";

import { cn } from "../../lib/cn";
import { HelperText, Text } from "./typography";

type FieldProps = {
  label?: React.ReactNode;
  error?: string;
  helper?: string;
  children: React.ReactNode;
  className?: string;
  htmlFor?: string;
};

export function Field({ label, error, helper, children, className, htmlFor }: FieldProps) {
  const generatedId = React.useId();
  const child =
    React.Children.count(children) === 1 && React.isValidElement(children)
      ? children
      : null;
  const childProps = child?.props as { id?: string; "aria-describedby"?: string } | undefined;
  const controlId = htmlFor ?? childProps?.id ?? (label && child ? `field-${generatedId.replace(/:/g, "")}` : undefined);
  const descriptionId = controlId && (error || helper) ? `${controlId}-description` : undefined;

  const describedBy = [childProps?.["aria-describedby"], descriptionId].filter(Boolean).join(" ") || undefined;
  const control = child
    ? React.cloneElement(child, {
        ...(controlId && !childProps?.id ? { id: controlId } : {}),
        ...(describedBy ? { "aria-describedby": describedBy } : {}),
        ...(error ? { "aria-invalid": true } : {}),
      } as Partial<typeof child.props>)
    : children;

  return (
    <div className={cn("grid gap-1.5", className)}>
      {label ? (
        <Text as="label" variant="label" htmlFor={controlId}>
          {label}
        </Text>
      ) : null}
      {control}
      {error ? (
        <HelperText id={descriptionId} role="alert" className="font-semibold text-status-danger">{error}</HelperText>
      ) : helper ? (
        <HelperText id={descriptionId}>{helper}</HelperText>
      ) : null}
    </div>
  );
}
