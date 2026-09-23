import { Bell } from "lucide-react";
import { useEffect, useRef } from "react";

import { useNotifications } from "./store";
import { IconButton } from "../shell/controls";

/**
 * The bell, in the header's right cluster before who the server is acting as.
 *
 * It is one control and one number. The number is what has arrived since this
 * browser last looked, and it is a badge rather than a word because a header
 * is read at a glance and a glance is all a count needs; the same count leads
 * the tab title, for the human who is looking at another tab entirely.
 *
 * The badge carries its own name, so what a screen reader hears is "3 unread"
 * rather than the digit three, and the button's own name stays "Notifications"
 * whether anything is waiting or not.
 */

/** Past this the exact number stops mattering and the badge stops growing. */
const TOO_MANY = 99;

export function NotificationsBell() {
  const { unread, panelIsOpen, openPanel } = useNotifications();
  const holder = useRef<HTMLSpanElement>(null);
  const wasOpen = useRef(false);

  // Closing the panel puts the keyboard back where it came from.
  //
  // The sheet's own restore leaves focus on the document body, because the
  // element that had it — the sheet's close control — is removed with the
  // sheet. So the bell takes it back itself: a human who opened this with the
  // keyboard must not have to tab in from the top of the page to carry on.
  useEffect(() => {
    if (wasOpen.current && !panelIsOpen) {
      holder.current?.querySelector("button")?.focus();
    }
    wasOpen.current = panelIsOpen;
  }, [panelIsOpen]);

  return (
    <span className="ms-bell" ref={holder}>
      <IconButton
        label="Notifications"
        aria-expanded={panelIsOpen}
        aria-controls="notifications-panel"
        onClick={() => {
          openPanel();
        }}
      >
        <Bell size={16} strokeWidth={1.75} aria-hidden="true" />
      </IconButton>
      {unread > 0 && (
        <span className="ms-bell-count" role="status" aria-label={`${String(unread)} unread`}>
          {unread > TOO_MANY ? `${String(TOO_MANY)}+` : unread}
        </span>
      )}
    </span>
  );
}
