import * as Dialog from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import type { ReactNode } from "react";

import { IconButton } from "./controls";

/**
 * A modal sheet from the edge of the window: the rail from the left on a
 * phone, which is the one this build opens. Radix owns the hard parts — the
 * focus trap, the return of focus to whatever opened it, Escape, and the scrim
 * — and its scroll lock appends the one inline style the page's policy allows,
 * through the nonce main.tsx sets before the first render.
 *
 * The Project Partner used to open from the right here, and is a drawer along
 * the bottom now; the side stays a parameter because a sheet from either edge
 * is one component, and the next thing to need one may come from the right.
 */
export function Sheet({
  open,
  onOpenChange,
  id,
  side,
  label,
  title,
  closeLabel,
  bodyClassName,
  actions,
  children,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The element the header's aria-controls names, where there is one. */
  id?: string;
  side: "left" | "right";
  label: string;
  title: string;
  closeLabel: string;
  bodyClassName: string;
  actions?: ReactNode;
  children: ReactNode;
}) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="ms-scrim" />
        <Dialog.Content
          id={id}
          className={side === "left" ? "ms-sheet ms-sheet--left" : "ms-sheet ms-sheet--right"}
          aria-label={label}
          aria-describedby={undefined}
        >
          <div className="ms-sheet-header">
            <Dialog.Title className="ms-sheet-title">{title}</Dialog.Title>
            <div className="ms-dock-actions">
              {actions}
              <IconButton
                label={closeLabel}
                onClick={() => {
                  onOpenChange(false);
                }}
              >
                <X size={16} strokeWidth={1.75} aria-hidden="true" />
              </IconButton>
            </div>
          </div>
          <div className={`ms-sheet-body ${bodyClassName}`}>{children}</div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
