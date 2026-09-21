import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import type { ButtonHTMLAttributes, ReactNode } from "react";

/**
 * The small controls the shell is built from. Every one of them keeps the
 * focus ring, every icon-only control carries a label, and a tooltip is never
 * the only place a label exists.
 */

export const TOOLTIP_DELAY = 300;

function classes(...names: (string | false | undefined)[]): string {
  return names.filter((name) => typeof name === "string" && name !== "").join(" ");
}

/** A tooltip on hover and on focus. The label is a repetition, never the name. */
export function Hint({ label, children }: { label: string; children: ReactNode }) {
  return (
    <TooltipPrimitive.Root>
      <TooltipPrimitive.Trigger asChild>{children}</TooltipPrimitive.Trigger>
      <TooltipPrimitive.Portal>
        <TooltipPrimitive.Content className="ms-tooltip" sideOffset={6}>
          {label}
        </TooltipPrimitive.Content>
      </TooltipPrimitive.Portal>
    </TooltipPrimitive.Root>
  );
}

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  /** Primary is the composer's Send, and nothing else in this build. */
  primary?: boolean;
};

export function Button({ primary = false, className, type = "button", children, ...rest }: ButtonProps) {
  return (
    <button type={type} className={classes("ms-button", primary && "ms-button--primary", className)} {...rest}>
      {children}
    </button>
  );
}

type IconButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  /** The accessible name. An icon-only control cannot be built without one. */
  label: string;
  /** The tooltip, when the reason to show differs from the name. */
  hint?: string;
};

export function IconButton({ label, hint, className, type = "button", children, ...rest }: IconButtonProps) {
  return (
    <Hint label={hint ?? label}>
      <button type={type} className={classes("ms-icon-button", className)} {...rest} aria-label={label}>
        {children}
      </button>
    </Hint>
  );
}

export function Chip({ marker = false, children }: { marker?: boolean; children: ReactNode }) {
  return <span className={classes("ms-chip", marker && "ms-chip--marker")}>{children}</span>;
}

/** Loading has a shape, not a spinner; it is decoration, so it is hidden. */
export function Skeleton() {
  return <span className="ms-skeleton" aria-hidden="true" />;
}
