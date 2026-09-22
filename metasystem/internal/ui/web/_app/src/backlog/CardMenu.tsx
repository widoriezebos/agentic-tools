import { useEffect, useRef, type KeyboardEvent } from "react";

import { itemAfter, type At, type Offer, type OfferId } from "./menu";

/**
 * A card's menu, where it was asked for.
 *
 * It is the only place the board's acts are offered, and it is not there until
 * a human asks: right-click, or Shift+F10 and the Menu key from the keyboard.
 * Nothing on a card is a control, so nothing on the board competes with the
 * work the cards are about.
 *
 * The semantics are the menu pattern's, because a popup that looks like a menu
 * and announces itself as a list of buttons is a popup that lies to anyone not
 * looking at it: one menu, one item per act, focus inside it while it is open
 * and back where it came from when it closes, the arrows moving within it, and
 * Escape leaving it. A scrim behind it closes it on a press anywhere else —
 * and is a scrim rather than a listener on the document, so the menu takes
 * nothing global down with it when it unmounts.
 *
 * The two coordinates are the only thing this component puts on an element
 * itself, because they are the only thing that cannot be written down in
 * advance: they are set as custom properties, and every rule that reads them
 * is in the stylesheet with the rest.
 */
export function CardMenu({
  at,
  label,
  offers,
  onChoose,
  onClose,
}: {
  at: At;
  /** What this menu is for, for anyone who cannot see which card it is on. */
  label: string;
  offers: Offer[];
  onChoose: (id: OfferId) => void;
  onClose: () => void;
}) {
  const menu = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const opener = globalThis.document.activeElement;
    menu.current?.querySelector<HTMLButtonElement>("button")?.focus();
    return () => {
      if (opener instanceof HTMLElement) {
        opener.focus();
      }
    };
  }, []);

  const keys = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === "Escape") {
      event.stopPropagation();
      onClose();
      return;
    }
    const items = [...(menu.current?.querySelectorAll<HTMLButtonElement>("button") ?? [])];
    const at_ = items.indexOf(globalThis.document.activeElement as HTMLButtonElement);
    const next = itemAfter(event.key, at_ < 0 ? 0 : at_, items.length);
    if (next === null) {
      return;
    }
    event.preventDefault();
    items[next].focus();
  };

  return (
    <>
      <div
        className="ms-card-scrim"
        aria-hidden="true"
        onPointerDown={onClose}
        onContextMenu={(event) => {
          // A second right-click closes this one rather than stacking the
          // browser's own menu on top of it.
          event.preventDefault();
          onClose();
        }}
      />
      <div
        className="ms-card-popup"
        role="menu"
        aria-label={label}
        ref={(element) => {
          menu.current = element;
          element?.style.setProperty("--ms-menu-x", `${String(at.x)}px`);
          element?.style.setProperty("--ms-menu-y", `${String(at.y)}px`);
        }}
        onKeyDown={keys}
      >
        {offers.map((offer) => (
          <button
            key={offer.id}
            type="button"
            role="menuitem"
            className="ms-card-popup-item"
            onClick={() => {
              onChoose(offer.id);
            }}
          >
            {offer.label}
          </button>
        ))}
      </div>
    </>
  );
}
