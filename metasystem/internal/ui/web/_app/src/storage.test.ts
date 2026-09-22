import { describe, expect, it } from "vitest";

import {
  BACKLOG_VIEW_KEY,
  DEFAULT_DOCK_HEIGHT,
  DOCK_HEIGHT_KEY,
  DOCK_KEY,
  GOAL_TAB_KEY,
  PROJECT_TAB_KEY,
  RAIL_KEY,
  readBacklogView,
  readDockHeight,
  readDockOpen,
  readGoalTab,
  readProjectTab,
  readRailExpanded,
  readTheme,
  writeBacklogView,
  writeDockHeight,
  writeDockOpen,
  writeGoalTab,
  writeProjectTab,
  writeRailExpanded,
  type Store,
} from "./storage";
import { THEME_KEY } from "./theme";

/** A store that answers from a map, the way a browser's would. */
function store(values: Record<string, string>): Store {
  const written = new Map(Object.entries(values));
  return {
    getItem: (key) => written.get(key) ?? null,
    setItem: (key, value) => {
      written.set(key, value);
    },
  };
}

/** The store of a browser with site data blocked: every access throws. */
const throwing: Store = {
  getItem: () => {
    throw new Error("site data is blocked");
  },
  setItem: () => {
    throw new Error("site data is blocked");
  },
};

describe("the stored view state", () => {
  it("reads the seven keys", () => {
    const written = store({
      [THEME_KEY]: "dark",
      [RAIL_KEY]: "collapsed",
      [DOCK_KEY]: "closed",
      [DOCK_HEIGHT_KEY]: "55.5",
      [BACKLOG_VIEW_KEY]: "list",
      [PROJECT_TAB_KEY]: "designs",
      [GOAL_TAB_KEY]: "questions",
    });

    expect(readTheme(written)).toBe("dark");
    expect(readRailExpanded(written)).toBe(false);
    expect(readDockOpen(written)).toBe(false);
    expect(readDockHeight(written)).toBe(55.5);
    expect(readBacklogView(written)).toBe("list");
    expect(readProjectTab(written)).toBe("designs");
    expect(readGoalTab(written)).toBe("questions");
  });

  // The drawer is closed until a human opens it: only the word it was opened
  // under opens it again. The board is the Backlog's default the other way
  // round: it is the day-to-day surface, so only the word "list" leaves it.
  it("defaults where nothing is stored", () => {
    const empty = store({});

    expect(readTheme(empty)).toBe("system");
    expect(readRailExpanded(empty)).toBe(true);
    expect(readDockOpen(empty)).toBe(false);
    expect(readDockHeight(empty)).toBe(DEFAULT_DOCK_HEIGHT);
    expect(readBacklogView(empty)).toBe("board");
    expect(readProjectTab(empty)).toBeNull();
    expect(readGoalTab(empty)).toBeNull();
  });

  it("defaults where something invalid is stored", () => {
    const nonsense = store({
      [THEME_KEY]: "{}",
      [RAIL_KEY]: "maybe",
      [DOCK_KEY]: "ajar",
      [DOCK_HEIGHT_KEY]: "half",
      [BACKLOG_VIEW_KEY]: "outline",
    });

    expect(readTheme(nonsense)).toBe("system");
    expect(readRailExpanded(nonsense)).toBe(true);
    expect(readDockOpen(nonsense)).toBe(false);
    expect(readDockHeight(nonsense)).toBe(DEFAULT_DOCK_HEIGHT);
    expect(readBacklogView(nonsense)).toBe("board");
  });

  it("remembers the Backlog's view the way the shell remembers the drawer", () => {
    const written = store({});

    writeBacklogView("list", written);

    expect(written.getItem(BACKLOG_VIEW_KEY)).toBe("list");
    expect(readBacklogView(written)).toBe("list");

    writeBacklogView("board", written);

    expect(readBacklogView(written)).toBe("board");
  });

  it("opens the drawer for a browser that was left with it open", () => {
    const opened = store({ [DOCK_KEY]: "open" });

    expect(readDockOpen(opened)).toBe(true);
  });

  // The drawer is remembered under the key the dock used, so a browser that
  // already carries an open dock opens with an open drawer.
  it("remembers the drawer under the key the dock left behind", () => {
    const written = store({});

    writeDockOpen(true, written);

    expect(written.getItem(DOCK_KEY)).toBe("open");
    expect(readDockOpen(written)).toBe(true);

    writeDockOpen(false, written);

    expect(written.getItem(DOCK_KEY)).toBe("closed");
    expect(readDockOpen(written)).toBe(false);
  });

  it("reads and writes a store that throws without surfacing an error", () => {
    expect(readTheme(throwing)).toBe("system");
    expect(readRailExpanded(throwing)).toBe(true);
    expect(readDockOpen(throwing)).toBe(false);
    expect(readDockHeight(throwing)).toBe(DEFAULT_DOCK_HEIGHT);
    expect(readBacklogView(throwing)).toBe("board");
    expect(readProjectTab(throwing)).toBeNull();
    expect(readGoalTab(throwing)).toBeNull();
    expect(() => {
      writeBacklogView("list", throwing);
    }).not.toThrow();
    expect(() => {
      writeProjectTab("designs", throwing);
    }).not.toThrow();
    expect(() => {
      writeGoalTab("designs", throwing);
    }).not.toThrow();
    expect(() => {
      writeRailExpanded(false, throwing);
    }).not.toThrow();
    expect(() => {
      writeDockOpen(false, throwing);
    }).not.toThrow();
    expect(() => {
      writeDockHeight(60, throwing);
    }).not.toThrow();
  });

  it("reads and writes nothing at all where there is no store", () => {
    expect(readProjectTab(null)).toBeNull();
    expect(readGoalTab(null)).toBeNull();
    expect(() => {
      writeProjectTab("designs", null);
    }).not.toThrow();
    expect(readDockOpen(null)).toBe(false);
    expect(readDockHeight(null)).toBe(DEFAULT_DOCK_HEIGHT);
    expect(() => {
      writeDockOpen(false, null);
    }).not.toThrow();
    expect(() => {
      writeDockHeight(60, null);
    }).not.toThrow();
  });

  // The divider writes a share of the work area's height, which arrives from
  // the panel group with more precision than a pixel is worth: a tenth of a
  // per cent is under a pixel on any window this runs in.
  it("remembers the drawer's height to a tenth of a per cent", () => {
    const written = store({});

    writeDockHeight(37.4321, written);

    expect(written.getItem(DOCK_HEIGHT_KEY)).toBe("37.4");
    expect(readDockHeight(written)).toBe(37.4);
  });

  // A height outside what the drawer may ever be is not the drawer's height:
  // the whole window and a sliver are both answers to a question nobody asked.
  it("keeps a stored height inside what the drawer may be", () => {
    expect(readDockHeight(store({ [DOCK_HEIGHT_KEY]: "99" }))).toBe(90);
    expect(readDockHeight(store({ [DOCK_HEIGHT_KEY]: "0" }))).toBe(10);
    expect(readDockHeight(store({ [DOCK_HEIGHT_KEY]: "-20" }))).toBe(DEFAULT_DOCK_HEIGHT);
  });

  // The two pages with tabs keep a key each: a human working through the
  // project's designs and a human checking a goal's open questions are not
  // asking for the same tab, and one key would have each reopening the
  // other's.
  it("remembers the tab of each page under its own key", () => {
    const written = store({});

    writeProjectTab("designs", written);
    writeGoalTab("questions", written);

    expect(written.getItem(PROJECT_TAB_KEY)).toBe("designs");
    expect(written.getItem(GOAL_TAB_KEY)).toBe("questions");
    expect(readProjectTab(written)).toBe("designs");
    expect(readGoalTab(written)).toBe("questions");

    writeProjectTab("documents", written);

    expect(readProjectTab(written)).toBe("documents");
    expect(readGoalTab(written)).toBe("questions");
  });

  // Whether a stored name is still a tab of the page is the page's question,
  // asked where the page's own tabs are known; the store answers with what it
  // was given and invents nothing.
  it("answers with the stored tab name, whatever it is", () => {
    expect(readProjectTab(store({ [PROJECT_TAB_KEY]: "nonsense" }))).toBe("nonsense");
  });
});
