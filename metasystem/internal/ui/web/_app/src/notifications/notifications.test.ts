import { describe, expect, it } from "vitest";

import {
  clockTime,
  emptyStore,
  groupByDay,
  loaded,
  merge,
  needsAHuman,
  newestID,
  oldestID,
  older,
  panelOpened,
  received,
  unreadCount,
  dismissed,
  type Notification,
} from "./notifications";
import {
  NOTIFICATIONS_SEEN_KEY,
  readNotificationsSeen,
  writeNotificationsSeen,
  type Store as Preferences,
} from "../storage";
import { titleFor, unreadPrefix } from "../title";

/**
 * The arithmetic the three surfaces share: what is unread, which day a row
 * belongs under, which toasts stay, what the tab title says, and what happens
 * when the same notification arrives twice.
 *
 * Everything here is a value in, a value out. Nothing in this file opens a
 * stream, renders an element or reads a clock it was not given.
 */

function notification(id: string, at: string, source = "steward", message = "a message"): Notification {
  return { id, at, message, source, ref: "", delivered: true, error: "" };
}

/** A local instant, so a day heading is the same wherever this test runs. */
function local(year: number, month: number, day: number, hour: number, minute: number): string {
  return new Date(year, month - 1, day, hour, minute).toISOString();
}

describe("what is unread", () => {
  const history = [notification("C", local(2026, 9, 23, 10, 31)), notification("B", local(2026, 9, 23, 9, 0)), notification("A", local(2026, 9, 22, 8, 0))];

  it("counts what sorts above the last id this browser saw", () => {
    expect(unreadCount(history, "A")).toBe(2);
    expect(unreadCount(history, "B")).toBe(1);
    expect(unreadCount(history, "C")).toBe(0);
  });

  it("counts everything for a browser that has seen nothing", () => {
    expect(unreadCount(history, null)).toBe(3);
  });

  it("counts nothing above an id newer than anything held", () => {
    expect(unreadCount(history, "Z")).toBe(0);
  });

  it("counts nothing where there is nothing", () => {
    expect(unreadCount([], null)).toBe(0);
  });

  it("names the newest and the oldest it holds, which is what opens and pages", () => {
    expect(newestID(history)).toBe("C");
    expect(oldestID(history)).toBe("A");
    expect(newestID([])).toBeNull();
    expect(oldestID([])).toBeNull();
  });
});

describe("the last-seen mark", () => {
  function store(): Preferences & { held: Record<string, string> } {
    const held: Record<string, string> = {};
    return {
      held,
      getItem: (key) => held[key] ?? null,
      setItem: (key, value) => {
        held[key] = value;
      },
    };
  }

  it("is remembered under its own key, and absent until something is read", () => {
    const kept = store();
    expect(readNotificationsSeen(kept)).toBeNull();
    writeNotificationsSeen("01M36QQEFQ0000000000000000", kept);
    expect(kept.held[NOTIFICATIONS_SEEN_KEY]).toBe("01M36QQEFQ0000000000000000");
    expect(readNotificationsSeen(kept)).toBe("01M36QQEFQ0000000000000000");
  });

  it("reads as nothing seen where the browser refuses to keep anything", () => {
    expect(readNotificationsSeen(null)).toBeNull();
    // A store that throws costs the next load a default, never an error.
    writeNotificationsSeen("A", {
      getItem: () => {
        throw new Error("site data is blocked");
      },
      setItem: () => {
        throw new Error("site data is blocked");
      },
    });
  });
});

describe("the day headings", () => {
  const now = new Date(2026, 8, 23, 12, 0);

  it("names today and yesterday, and dates everything older", () => {
    const days = groupByDay(
      [
        notification("D", local(2026, 9, 23, 10, 31)),
        notification("C", local(2026, 9, 23, 9, 5)),
        notification("B", local(2026, 9, 22, 18, 0)),
        notification("A", local(2026, 9, 20, 8, 0)),
      ],
      now,
    );
    expect(days.map((day) => day.heading)).toEqual(["Today", "Yesterday", "2026-09-20"]);
    expect(days[0].rows.map((row) => row.id)).toEqual(["D", "C"]);
    expect(days[1].rows.map((row) => row.id)).toEqual(["B"]);
    expect(days[2].rows.map((row) => row.id)).toEqual(["A"]);
  });

  it("groups nothing into nothing", () => {
    expect(groupByDay([], now)).toEqual([]);
  });

  it("keeps a row whose instant nothing recorded, under a heading that says so", () => {
    const days = groupByDay([notification("A", "")], now);
    expect(days.map((day) => day.heading)).toEqual(["Undated"]);
    expect(clockTime("")).toBe("--:--");
  });

  it("reads the wall clock to the minute", () => {
    expect(clockTime(local(2026, 9, 23, 10, 31))).toBe("10:31");
    expect(clockTime(local(2026, 9, 23, 9, 5))).toBe("09:05");
  });
});

describe("which toasts stay", () => {
  it("keeps the two that are addressed to a person", () => {
    expect(needsAHuman("alert")).toBe(true);
    expect(needsAHuman("handoff")).toBe(true);
  });

  it("lets the steward's narration leave on its own", () => {
    expect(needsAHuman("steward")).toBe(false);
    expect(needsAHuman("verdict")).toBe(false);
    // A source from a newer engine is narration until this build learns
    // otherwise: a toast that never left would be worse than one that did.
    expect(needsAHuman("something-new")).toBe(false);
  });
});

describe("the tab title", () => {
  const known = { state: "known", subject: "MetaSystem", mode: "self-hosted", conflict: false } as const;

  it("leads with what is waiting, and says nothing when nothing is", () => {
    expect(unreadPrefix(0)).toBe("");
    expect(unreadPrefix(3)).toBe("(3) ");
    expect(titleFor("Backlog", known, 3)).toBe("(3) Backlog · MetaSystem · self-hosted");
    expect(titleFor("Backlog", known, 0)).toBe("Backlog · MetaSystem · self-hosted");
    expect(titleFor("Backlog", known)).toBe("Backlog · MetaSystem · self-hosted");
  });

  it("carries the count in front of every other reading the title has", () => {
    expect(titleFor("Overview", { state: "loading" }, 2)).toBe("(2) MetaSystem interface");
    expect(titleFor("Overview", { state: "unknown" }, 1)).toBe("(1) Overview · MetaSystem interface");
    expect(titleFor("Overview", { ...known, conflict: true }, 5)).toBe("(5) Identity conflict · MetaSystem interface");
  });
});

describe("the store", () => {
  it("holds the history newest first, however the pages arrive", () => {
    const first = loaded(emptyStore, [notification("C", local(2026, 9, 23, 10, 0)), notification("B", local(2026, 9, 23, 9, 0))], 200);
    const paged = older(first, [notification("A", local(2026, 9, 22, 8, 0))], 200);
    expect(paged.history.map((row) => row.id)).toEqual(["C", "B", "A"]);
    expect(paged.loaded).toBe(true);
    expect(paged.loadingOlder).toBe(false);
  });

  it("stops offering an older page once one comes back short", () => {
    expect(loaded(emptyStore, [notification("A", local(2026, 9, 23, 9, 0))], 200).hasOlder).toBe(false);
    const full = Array.from({ length: 3 }, (_, index) => notification(`I${String(index)}`, local(2026, 9, 23, 9, index)));
    expect(loaded(emptyStore, full, 3).hasOlder).toBe(true);
    expect(older(emptyStore, full.slice(0, 2), 3).hasOlder).toBe(false);
  });

  it("ignores a notification it already holds, however it arrives again", () => {
    const held = loaded(emptyStore, [notification("B", local(2026, 9, 23, 9, 0))], 200);
    const arrived = received(held, notification("C", local(2026, 9, 23, 10, 0)));
    expect(arrived.history.map((row) => row.id)).toEqual(["C", "B"]);
    expect(arrived.toasts.map((row) => row.id)).toEqual(["C"]);

    // The browser reconnected and the server resent what it had already sent.
    const again = received(arrived, notification("C", local(2026, 9, 23, 10, 0)));
    expect(again).toBe(arrived);
    expect(again.history.map((row) => row.id)).toEqual(["C", "B"]);
    expect(again.toasts.map((row) => row.id)).toEqual(["C"]);

    // And one that was already in the history raises no toast at all.
    const known = received(arrived, notification("B", local(2026, 9, 23, 9, 0)));
    expect(known).toBe(arrived);
  });

  it("merges a page that overlaps what is already held, without repeating a row", () => {
    const held = loaded(emptyStore, [notification("C", local(2026, 9, 23, 10, 0))], 200);
    const overlapping = merge(held.history, [
      notification("C", local(2026, 9, 23, 10, 0)),
      notification("B", local(2026, 9, 23, 9, 0)),
    ]);
    expect(overlapping.map((row) => row.id)).toEqual(["C", "B"]);
    // Nothing new is nothing changed, so nothing renders again.
    expect(merge(held.history, [notification("C", local(2026, 9, 23, 10, 0))])).toBe(held.history);
  });

  it("stacks at most three toasts, newest first, and the oldest gives way", () => {
    let held = emptyStore;
    for (const id of ["A", "B", "C", "D"]) {
      held = received(held, notification(id, local(2026, 9, 23, 9, 0), "alert"));
    }
    expect(held.toasts.map((row) => row.id)).toEqual(["D", "C", "B"]);
    expect(held.history.map((row) => row.id)).toEqual(["D", "C", "B", "A"]);
  });

  it("loses a toast when it is closed, and keeps its row in the history", () => {
    const held = received(emptyStore, notification("A", local(2026, 9, 23, 9, 0)));
    const closed = dismissed(held, "A");
    expect(closed.toasts).toEqual([]);
    expect(closed.history.map((row) => row.id)).toEqual(["A"]);
    // Closing what is not there changes nothing.
    expect(dismissed(closed, "A")).toBe(closed);
  });

  it("clears every toast when the panel opens, including the ones that stay", () => {
    let held = received(emptyStore, notification("A", local(2026, 9, 23, 9, 0), "alert"));
    held = received(held, notification("B", local(2026, 9, 23, 9, 1), "handoff"));
    expect(held.toasts.map((row) => row.id)).toEqual(["B", "A"]);
    const opened = panelOpened(held);
    expect(opened.toasts).toEqual([]);
    expect(opened.history.map((row) => row.id)).toEqual(["B", "A"]);
    // An open panel with nothing on screen changes nothing.
    expect(panelOpened(opened)).toBe(opened);
  });

  it("raises no toast for the history it opens with", () => {
    const held = loaded(emptyStore, [notification("B", local(2026, 9, 23, 9, 0)), notification("A", local(2026, 9, 22, 9, 0))], 200);
    expect(held.toasts).toEqual([]);
  });
});
