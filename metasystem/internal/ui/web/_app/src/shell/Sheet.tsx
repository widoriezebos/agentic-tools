import * as Dialog from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import { useRef, type ReactNode } from "react";

import { IconButton } from "./controls";
import { DEFAULT_MODALITY, useWorkModal, type Modality } from "./workmodal";
import { useOpenSheet } from "../partner/store";

/**
 * A sheet from an edge: the rail from the left on a phone, the font chooser
 * and the "Seeing:" block from the right, and sign-in.
 *
 * Radix owns the hard parts — the return of focus to whatever opened it,
 * Escape, and the accessible dialog itself.
 *
 * How much it is modal for is the sheet's own (see workmodal.tsx). Every sheet
 * but sign-in is modal for the work area: Radix is told modal={false}, so
 * nothing about the body's pointer events or its scroll is touched and no
 * focus trap is installed, and the scrim below is our own element inside the
 * work area's layer rather than Radix's window-wide overlay. The drawer
 * beneath stays live. Sign-in keeps Radix's own modality, and with it the
 * scroll lock whose one inline style the page's policy allows through the
 * nonce main.tsx sets before the first render.
 *
 * Two things have to be refused by hand while it is not window-modal, because
 * Radix listens on the document for both: an Escape pressed in the drawer,
 * which belongs to the drawer, and a click outside, which is the scrim's or
 * the drawer's and never closes a form half filled in.
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
  modal = DEFAULT_MODALITY,
  sheetName,
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
  /** How much of the window this is modal for. Sign-in's is the exception. */
  modal?: Modality;
  /**
   * What the capture calls this sheet while it is open, or "" for a sheet that
   * names nothing — the one showing the capture itself.
   */
  sheetName?: string;
  actions?: ReactNode;
  children: ReactNode;
}) {
  const host = useWorkModal(modal, open);
  useOpenSheet(open ? (sheetName ?? title) : "");
  const content = useRef<HTMLDivElement | null>(null);

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange} modal={host === null}>
      <Dialog.Portal container={host ?? undefined}>
        {/* Our own scrim where the sheet is the work area's: Radix renders no
            overlay at all while it is not modal, and the one it renders when
            it is covers the window. Both go through the portal, so the scrim
            and the sheet land in one place in one order. */}
        {host === null ? (
          <Dialog.Overlay className="ms-scrim" />
        ) : (
          <div className="ms-scrim ms-scrim--work" aria-hidden="true" />
        )}
        <Dialog.Content
          id={id}
          ref={content}
          className={side === "left" ? "ms-sheet ms-sheet--left" : "ms-sheet ms-sheet--right"}
          aria-label={label}
          aria-describedby={undefined}
          onEscapeKeyDown={(event) => {
            // Escape acts on whichever of the sheet and the drawer has focus.
            if (host !== null && content.current?.contains(globalThis.document.activeElement) !== true) {
              event.preventDefault();
            }
          }}
          // A click outside is the scrim's or the drawer's while this is the
          // work area's: the drawer is meant to be used, and a form half
          // filled in is not something to lose to a stray click. A sheet
          // modal for the whole window keeps Radix's own dismissal, which is
          // what sign-in has always had.
          onPointerDownOutside={(event) => {
            if (host !== null) {
              event.preventDefault();
            }
          }}
          onFocusOutside={(event) => {
            if (host !== null) {
              event.preventDefault();
            }
          }}
          onInteractOutside={(event) => {
            if (host !== null) {
              event.preventDefault();
            }
          }}
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
