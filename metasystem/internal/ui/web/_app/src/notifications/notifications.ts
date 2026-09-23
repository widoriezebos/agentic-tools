/**
 * The steward's notifications, as the page holds them.
 *
 * The steward already reaches the human through the operating system: a toast
 * from notification centre, or whatever the configured command does. That does
 * not change. What this adds is the same message in the interface, at the same
 * moment, and — the part a desktop toast can never give — the history of them.
 *
 * Everything here is arithmetic over a list, with no request, no clock of its
 * own and no element in it. The store below is what the bell, the panel and
 * the toast stack all read, so the three of them cannot disagree about what
 * has arrived, what is unread, or what is still on screen.
 */

/** One delivery attempt, exactly as the server answers with it. */
export type Notification = {
  id: string;
  at: string;
  message: string;
  /**
   * Which part of the steward raised it. It is a string rather than a union
   * because a newer engine may know a source this build does not, and a row
   * whose chip reads a word this page has never seen is better than a row
   * this page refuses to render.
   */
  source: string;
  /** The alert episode or the pending nonce, where the steward knew one. */
  ref: string;
  delivered: boolean;
  /** Why it was not delivered. Empty where it was. */
  error: string;
};

/** The four sources this build has words for. */
export const SOURCES = ["alert", "handoff", "verdict", "steward"] as const;

/**
 * The sources that need a human rather than a glance.
 *
 * An alert is the steward saying something is wrong, and a handoff is a seat
 * saying it is the human's turn: both are addressed to somebody, and a toast
 * that removed itself before it was read would be the interface losing a
 * message the operating system would have kept. Everything else is the
 * steward narrating, and narration leaves on its own.
 */
export function needsAHuman(source: string): boolean {
  return source === "alert" || source === "handoff";
}

/** At most this many toasts stand at once; the oldest give way. */
export const TOAST_LIMIT = 3;

/**
 * What the page holds.
 *
 * `history` is newest first, which is the order it is read in and the order
 * the server answers in. `toasts` is newest first too, and is a subset of what
 * arrived on the stream while this page was open — never of what was loaded
 * as history, because a page that has just opened has not missed anything and
 * a stack of twelve toasts on load is not a notification, it is a wall.
 */
export type Store = {
  history: Notification[];
  toasts: Notification[];
  /** True once the history route answered, so the panel can tell empty from unread. */
  loaded: boolean;
  /** What went wrong reading the history, in one line, or "". */
  problem: string;
  /** True while an older page is being fetched. */
  loadingOlder: boolean;
  /** False once an older page came back short, so the button stops offering. */
  hasOlder: boolean;
};

export const emptyStore: Store = {
  history: [],
  toasts: [],
  loaded: false,
  problem: "",
  loadingOlder: false,
  hasOlder: false,
};

/**
 * Merge newer rows into a newest-first history, ignoring any id already held.
 *
 * The duplicate rule is what makes a reconnect safe. The browser reopens a
 * dropped EventSource by itself and sends back the last id it received; a
 * server that resends one line either side of that boundary — or a page that
 * loads its history and then receives the same row on the stream — must show
 * one row and raise one toast, not two.
 */
export function merge(history: Notification[], arriving: Notification[]): Notification[] {
  const held = new Set(history.map((notification) => notification.id));
  const fresh = arriving.filter((notification) => {
    if (held.has(notification.id)) {
      return false;
    }
    held.add(notification.id);
    return true;
  });
  if (fresh.length === 0) {
    return history;
  }
  return [...fresh, ...history].sort((left, right) => (left.id < right.id ? 1 : left.id > right.id ? -1 : 0));
}

/**
 * The history route answered: the page it gave, newest first. A page shorter
 * than the one asked for is the whole journal, so there is nothing older to
 * offer and the button that offers it does not appear.
 */
export function loaded(store: Store, page: Notification[], asked: number): Store {
  return {
    ...store,
    history: merge(store.history, page),
    loaded: true,
    problem: "",
    hasOlder: page.length >= asked,
  };
}

/** The history route refused, and the panel says so rather than showing nothing. */
export function failed(store: Store, problem: string): Store {
  return { ...store, loaded: true, problem };
}

/** An older page came back. A short page is the end of the journal. */
export function older(store: Store, page: Notification[], asked: number): Store {
  return {
    ...store,
    history: merge(store.history, page),
    loadingOlder: false,
    hasOlder: page.length >= asked,
  };
}

/**
 * One notification arrived on the stream.
 *
 * It joins the history and raises a toast — unless it is a duplicate, in which
 * case it changes nothing at all, which is the reconnect rule above.
 */
export function received(store: Store, notification: Notification): Store {
  if (store.history.some((held) => held.id === notification.id)) {
    return store;
  }
  return {
    ...store,
    history: merge(store.history, [notification]),
    toasts: [notification, ...store.toasts].slice(0, TOAST_LIMIT),
  };
}

/** One toast was closed, or its life on screen ran out. */
export function dismissed(store: Store, id: string): Store {
  if (!store.toasts.some((toast) => toast.id === id)) {
    return store;
  }
  return { ...store, toasts: store.toasts.filter((toast) => toast.id !== id) };
}

/**
 * The panel opened.
 *
 * Every toast goes: the panel is the whole record, shown in full, and a toast
 * standing over it would be the same message twice. That includes the two
 * sources that otherwise stay until they are closed — opening the panel IS
 * reading them.
 */
export function panelOpened(store: Store): Store {
  return store.toasts.length === 0 ? store : { ...store, toasts: [] };
}

/**
 * How many notifications the viewer has not seen.
 *
 * An id is a ULID, so it sorts in the order it was minted and two of them
 * compare as strings. Unread is everything above the last id this browser
 * recorded as seen. A browser that has recorded nothing has seen nothing,
 * which is why a first visit counts the whole history: the alternative is a
 * page that decides on the viewer's behalf that a fortnight of messages was
 * read by somebody.
 */
export function unreadCount(history: Notification[], lastSeen: string | null): number {
  if (lastSeen === null) {
    return history.length;
  }
  return history.filter((notification) => notification.id > lastSeen).length;
}

/** The newest id the page holds, which is what opening the panel marks as seen. */
export function newestID(history: Notification[]): string | null {
  return history.length === 0 ? null : history[0].id;
}

/** The oldest id the page holds, which is what "Load older" pages before. */
export function oldestID(history: Notification[]): string | null {
  return history.length === 0 ? null : history[history.length - 1].id;
}

/** A day's worth of rows, under the heading that names the day. */
export type Day = { heading: string; key: string; rows: Notification[] };

/**
 * The history, grouped under day headings, newest day first and newest row
 * first inside a day.
 *
 * The two nearest days are named rather than dated, because "Yesterday" is how
 * a human holds a day they remember and "2026-09-22" is how a machine holds
 * it. Everything older is dated, because by then the name has stopped helping.
 * Days are the machine's own, not UTC: the heading has to agree with the clock
 * on the wall behind the screen.
 */
export function groupByDay(history: Notification[], now: Date): Day[] {
  const days: Day[] = [];
  for (const notification of history) {
    const at = instant(notification.at);
    const key = at === null ? "unknown" : dayKey(at);
    const last = days[days.length - 1];
    if (last !== undefined && last.key === key) {
      last.rows.push(notification);
      continue;
    }
    days.push({ key, heading: at === null ? "Undated" : dayHeading(at, now), rows: [notification] });
  }
  return days;
}

/** The local calendar day, as a key two instants can be compared by. */
function dayKey(at: Date): string {
  return `${String(at.getFullYear())}-${pad(at.getMonth() + 1)}-${pad(at.getDate())}`;
}

/** What a day is called: the two nearest by name, the rest by date. */
export function dayHeading(at: Date, now: Date): string {
  const today = dayKey(now);
  if (dayKey(at) === today) {
    return "Today";
  }
  const yesterday = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1);
  if (dayKey(at) === dayKey(yesterday)) {
    return "Yesterday";
  }
  return dayKey(at);
}

/** The wall clock, to the minute, which is the precision a message has. */
export function clockTime(stamp: string): string {
  const at = instant(stamp);
  return at === null ? "--:--" : `${pad(at.getHours())}:${pad(at.getMinutes())}`;
}

function instant(stamp: string): Date | null {
  if (stamp === "") {
    return null;
  }
  const at = new Date(stamp);
  return Number.isNaN(at.getTime()) ? null : at;
}

function pad(value: number): string {
  return String(value).padStart(2, "0");
}
