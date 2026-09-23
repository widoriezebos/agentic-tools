import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

import { loadNotifications, PAGE } from "./api";
import { NotificationsPanel } from "./Panel";
import {
  emptyStore,
  failed,
  loaded,
  newestID,
  older,
  oldestID,
  panelOpened,
  received,
  unreadCount,
  dismissed as withoutToast,
  type Store,
} from "./notifications";
import { openNotificationStream } from "./stream";
import { Toasts } from "./Toasts";
import { readNotificationsSeen, writeNotificationsSeen } from "../storage";

/**
 * What the steward has said, held once for the whole page.
 *
 * There is one history, one stream and one unread count, and the bell, the
 * panel and the toast stack all read them from here — three surfaces onto one
 * fact, rather than three requests and three opinions. It is the shape the
 * identity provider already has, for the same reason: the shell renders the
 * panel and the toasts beside its children, so nothing below has to know they
 * exist in order to be underneath them.
 *
 * The history is read once, when the page loads, and again only when a human
 * presses "Load older". Everything after the load arrives on the stream, which
 * the server pushes. Nothing here polls, and nothing here sets a timer.
 */

type Notifications = {
  store: Store;
  /** How many the viewer has not seen. Drives the badge and the tab title. */
  unread: number;
  /** True while the panel stands open. */
  panelIsOpen: boolean;
  /**
   * Open the panel, optionally at one row — which is what clicking a toast
   * does. Opening marks everything held as seen.
   */
  openPanel: (at?: string) => void;
  closePanel: () => void;
  /** The row the panel should bring into view, or null. */
  focused: string | null;
  /** Close one toast, by its own control or by its life running out. */
  dismiss: (id: string) => void;
  /** Ask for the page behind the oldest row held. */
  loadOlder: () => void;
};

const NotificationsContext = createContext<Notifications>({
  store: emptyStore,
  unread: 0,
  panelIsOpen: false,
  openPanel: () => {},
  closePanel: () => {},
  focused: null,
  dismiss: () => {},
  loadOlder: () => {},
});

export function useNotifications(): Notifications {
  return useContext(NotificationsContext);
}

export function NotificationsProvider({ children }: { children: ReactNode }) {
  const [store, setStore] = useState<Store>(emptyStore);
  const [seen, setSeen] = useState<string | null>(() => readNotificationsSeen());
  const [open, setOpen] = useState(false);
  const [focused, setFocused] = useState<string | null>(null);

  // The history, once.
  useEffect(() => {
    const aborter = new AbortController();
    loadNotifications(null, aborter.signal)
      .then((page) => {
        setStore((held) => loaded(held, page, PAGE));
      })
      .catch((error: unknown) => {
        if (!aborter.signal.aborted) {
          setStore((held) => failed(held, reason(error)));
        }
      });
    return () => {
      aborter.abort();
    };
  }, []);

  // The stream, for the life of the page. The browser reconnects it on its
  // own and resumes from the last id it was sent, so there is nothing to
  // retry here and nothing to schedule.
  useEffect(
    () =>
      openNotificationStream((notification) => {
        setStore((held) => received(held, notification));
      }),
    [],
  );

  // Opening the panel is reading: everything held is marked as seen, and the
  // toasts go, because the panel shows the same messages in full. It stays
  // true while the panel is open, so a notification that arrives with the
  // panel in front of the human is seen as it lands.
  useEffect(() => {
    if (!open) {
      return;
    }
    setStore(panelOpened);
    const newest = newestID(store.history);
    if (newest !== null && (seen === null || newest > seen)) {
      setSeen(newest);
      writeNotificationsSeen(newest);
    }
  }, [open, store.history, seen]);

  const openPanel = useCallback((at?: string) => {
    setFocused(at ?? null);
    setOpen(true);
  }, []);

  const closePanel = useCallback(() => {
    setOpen(false);
    setFocused(null);
  }, []);

  const dismiss = useCallback((id: string) => {
    setStore((held) => withoutToast(held, id));
  }, []);

  const oldest = oldestID(store.history);
  const asking = store.loadingOlder;
  const loadOlder = useCallback(() => {
    if (oldest === null || asking) {
      return;
    }
    setStore((held) => ({ ...held, loadingOlder: true }));
    loadNotifications(oldest)
      .then((page) => {
        setStore((held) => older(held, page, PAGE));
      })
      .catch(() => {
        // An older page that did not arrive leaves what is on screen alone
        // and stops offering; the history route is read again on the next
        // load, which is when a human would ask for it anyway.
        setStore((held) => ({ ...held, loadingOlder: false, hasOlder: false }));
      });
  }, [oldest, asking]);

  const unread = unreadCount(store.history, seen);
  const value = useMemo(
    () => ({ store, unread, panelIsOpen: open, openPanel, closePanel, focused, dismiss, loadOlder }),
    [store, unread, open, openPanel, closePanel, focused, dismiss, loadOlder],
  );

  return (
    <NotificationsContext.Provider value={value}>
      {children}
      <NotificationsPanel />
      <Toasts />
    </NotificationsContext.Provider>
  );
}

function reason(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
