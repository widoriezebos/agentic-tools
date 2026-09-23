import { X } from "lucide-react";
import type { AnimationEvent } from "react";

import { needsAHuman, type Notification } from "./notifications";
import { useNotifications } from "./store";
import { Chip, IconButton } from "../shell/controls";

/**
 * What just arrived, at the bottom right, above the Project Partner's drawer.
 *
 * Only what arrives on the stream while the page is open appears here. History
 * never does: a page that has just loaded has missed nothing, and a stack of
 * twelve toasts on load is not a notification, it is a wall.
 *
 * How long one stands is decided by what raised it. An alert or a handoff is
 * addressed to a person — something is wrong, or it is their turn — so it
 * stays until it is closed or until the panel is opened. Everything else is
 * the steward narrating, and narration leaves on its own after eight seconds.
 *
 * That eight seconds is a CSS animation, not a timer: the page sets none, and
 * the animation's end is what removes the toast. It pauses while the pointer
 * is over the stack or the keyboard is in it, so a message cannot leave while
 * it is being read or while its close control is focused. Under a
 * reduced-motion preference the same eight seconds pass and nothing moves.
 */

/** The animation whose end is a toast's life running out. */
const LIFE = "ms-toast-life";
const LIFE_STILL = "ms-toast-life-still";

export function Toasts() {
  const { store, dismiss, openPanel } = useNotifications();

  if (store.toasts.length === 0) {
    return null;
  }
  return (
    <div className="ms-toasts" role="status" aria-live="polite">
      {store.toasts.map((notification) => (
        <Toast
          key={notification.id}
          notification={notification}
          onOpen={() => {
            openPanel(notification.id);
          }}
          onClose={() => {
            dismiss(notification.id);
          }}
        />
      ))}
    </div>
  );
}

function Toast({
  notification,
  onOpen,
  onClose,
}: {
  notification: Notification;
  onOpen: () => void;
  onClose: () => void;
}) {
  const stays = needsAHuman(notification.source);
  return (
    <div
      className={stays ? "ms-toast ms-toast--standing" : "ms-toast ms-toast--fading"}
      onAnimationEnd={(event: AnimationEvent<HTMLDivElement>) => {
        // The entry animation ends too, and means nothing; only the life
        // running out removes a toast.
        if (event.animationName === LIFE || event.animationName === LIFE_STILL) {
          onClose();
        }
      }}
    >
      <button type="button" className="ms-toast-body" onClick={onOpen}>
        <span className="ms-toast-line">
          <Chip>{notification.source}</Chip>
        </span>
        <span className="ms-toast-message">{notification.message}</span>
        {!notification.delivered && (
          <span className="ms-toast-undelivered">not delivered to macOS: {notification.error}</span>
        )}
      </button>
      <IconButton label="Close this notification" onClick={onClose}>
        <X size={14} strokeWidth={1.75} aria-hidden="true" />
      </IconButton>
    </div>
  );
}
