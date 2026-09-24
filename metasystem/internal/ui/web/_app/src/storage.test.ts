import { describe, expect, it } from "vitest";

import { ANY, DEFAULT_WINDOW, named, noFilters, NONE } from "./backlog/filters";
import { DEFAULT_LEADING, DEFAULT_SIZE, DEFAULT_TYPEFACE, INTERFACE, MONO } from "./partner/typeface";
import {
  BACKLOG_ARC_KEY,
  BACKLOG_DONE_KEY,
  BACKLOG_PRIORITY_KEY,
  BACKLOG_SEAT_KEY,
  BACKLOG_TEXT_KEY,
  BACKLOG_TIER_KEY,
  BACKLOG_VIEW_KEY,
  DEFAULT_DOCK_HEIGHT,
  DOCK_HEIGHT_KEY,
  DOCK_KEY,
  GOAL_TAB_KEY,
  PARTNER_FONT_KEY,
  PARTNER_FONT_SIZE_KEY,
  PARTNER_LINE_HEIGHT_KEY,
  PROJECT_SCOPE_KEY,
  PROJECT_TAB_KEY,
  RAIL_KEY,
  readBacklogFilters,
  readBacklogView,
  readDockHeight,
  readDockOpen,
  readDoneWindow,
  readGoalTab,
  readPartnerTypeface,
  readProjectScope,
  readProjectTab,
  readRailExpanded,
  readTheme,
  writeBacklogFilters,
  writeBacklogView,
  writeDockHeight,
  writeDockOpen,
  writeDoneWindow,
  writeGoalTab,
  writePartnerTypeface,
  writeProjectScope,
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
  it("reads the fourteen keys", () => {
    const written = store({
      [THEME_KEY]: "dark",
      [RAIL_KEY]: "collapsed",
      [DOCK_KEY]: "closed",
      [DOCK_HEIGHT_KEY]: "55.5",
      [BACKLOG_VIEW_KEY]: "list",
      [PROJECT_TAB_KEY]: "designs",
      [PROJECT_SCOPE_KEY]: "goals",
      [GOAL_TAB_KEY]: "questions",
      [BACKLOG_TEXT_KEY]: "ledger",
      [BACKLOG_PRIORITY_KEY]: "1",
      [BACKLOG_TIER_KEY]: "3",
      [BACKLOG_SEAT_KEY]: named("m1e"),
      [BACKLOG_ARC_KEY]: NONE,
      [BACKLOG_DONE_KEY]: "7",
    });

    expect(readTheme(written)).toBe("dark");
    expect(readRailExpanded(written)).toBe(false);
    expect(readDockOpen(written)).toBe(false);
    expect(readDockHeight(written)).toBe(55.5);
    expect(readBacklogView(written)).toBe("list");
    expect(readProjectTab(written)).toBe("designs");
    expect(readProjectScope(written)).toBe("goals");
    expect(readGoalTab(written)).toBe("questions");
    expect(readBacklogFilters(written)).toEqual({
      text: "ledger",
      priority: "1",
      tier: "3",
      seat: named("m1e"),
      arc: NONE,
    });
    expect(readDoneWindow(written)).toBe(7);
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
    expect(readProjectScope(empty)).toBe("project");
    expect(readGoalTab(empty)).toBeNull();
    expect(readBacklogFilters(empty)).toEqual(noFilters);
    expect(readDoneWindow(empty)).toBe(DEFAULT_WINDOW);
  });

  it("defaults where something invalid is stored", () => {
    const nonsense = store({
      [THEME_KEY]: "{}",
      [RAIL_KEY]: "maybe",
      [DOCK_KEY]: "ajar",
      [DOCK_HEIGHT_KEY]: "half",
      [BACKLOG_VIEW_KEY]: "outline",
      [PROJECT_SCOPE_KEY]: "everything",
      [BACKLOG_PRIORITY_KEY]: "4",
      [BACKLOG_TIER_KEY]: "high",
      [BACKLOG_DONE_KEY]: "yesterday",
    });

    expect(readTheme(nonsense)).toBe("system");
    expect(readRailExpanded(nonsense)).toBe(true);
    expect(readDockOpen(nonsense)).toBe(false);
    expect(readDockHeight(nonsense)).toBe(DEFAULT_DOCK_HEIGHT);
    expect(readBacklogView(nonsense)).toBe("board");
    expect(readProjectScope(nonsense)).toBe("project");
    expect(readBacklogFilters(nonsense).priority).toBe(ANY);
    expect(readBacklogFilters(nonsense).tier).toBe(ANY);
    expect(readDoneWindow(nonsense)).toBe(DEFAULT_WINDOW);
  });

  /**
   * The scope the Project page was last left on.
   *
   * It is one key for the page and not one per tab: scope is what a human is
   * doing — settling the project's own shape, or working through what a goal
   * is about — and a human who asked for the goal-scoped designs is asking
   * the same of the decisions beside them.
   */
  it("remembers which records the Project page is showing", () => {
    const written = store({});

    expect(readProjectScope(written)).toBe("project");

    writeProjectScope("goals", written);

    expect(written.getItem(PROJECT_SCOPE_KEY)).toBe("goals");
    expect(readProjectScope(written)).toBe("goals");

    writeProjectScope("all", written);

    expect(readProjectScope(written)).toBe("all");
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

  // Five independent answers under five keys: clearing the text is not a
  // change of mind about the seat, and a value an older build never wrote is
  // read as no preference rather than throwing the other four away.
  it("remembers each filter under its own key, and reads a missing one as any", () => {
    const written = store({});

    writeBacklogFilters({ text: "ledger", priority: "2", tier: "", seat: NONE, arc: named("harvest") }, written);

    expect(written.getItem(BACKLOG_TEXT_KEY)).toBe("ledger");
    expect(written.getItem(BACKLOG_PRIORITY_KEY)).toBe("2");
    expect(written.getItem(BACKLOG_TIER_KEY)).toBe("");
    expect(written.getItem(BACKLOG_SEAT_KEY)).toBe(NONE);
    expect(written.getItem(BACKLOG_ARC_KEY)).toBe(named("harvest"));
    expect(readBacklogFilters(written)).toEqual({
      text: "ledger",
      priority: "2",
      tier: "",
      seat: NONE,
      arc: named("harvest"),
    });

    expect(readBacklogFilters(store({ [BACKLOG_TEXT_KEY]: "ledger" }))).toEqual({ ...noFilters, text: "ledger" });
  });

  // "all" is stored as a word and read back as null, which is a window this
  // build offers and not a window it failed to find.
  it("remembers every window the Done lane offers, including all", () => {
    const written = store({});

    writeDoneWindow(null, written);

    expect(written.getItem(BACKLOG_DONE_KEY)).toBe("all");
    expect(readDoneWindow(written)).toBeNull();

    writeDoneWindow(30, written);

    expect(written.getItem(BACKLOG_DONE_KEY)).toBe("30");
    expect(readDoneWindow(written)).toBe(30);
  });

  // The conversation's face, its size and its line spacing: three keys,
  // because a human who changed the size has not changed their mind about the
  // face. All three are validated on the way out, so a name nothing could
  // render, a size nothing could read and a spacing nothing ever offered
  // arrive as the defaults rather than as a broken column.
  it("remembers the conversation's face, size and spacing under their own keys", () => {
    const written = store({});

    writePartnerTypeface({ face: "Meslo LG M for Powerline", size: 18, leading: 1.8 }, written);

    expect(written.getItem(PARTNER_FONT_KEY)).toBe("Meslo LG M for Powerline");
    expect(written.getItem(PARTNER_FONT_SIZE_KEY)).toBe("18");
    expect(written.getItem(PARTNER_LINE_HEIGHT_KEY)).toBe("1.8");
    expect(readPartnerTypeface(written)).toEqual({
      face: "Meslo LG M for Powerline",
      size: 18,
      leading: 1.8,
    });

    writePartnerTypeface({ face: MONO, size: 12, leading: 1.2 }, written);

    expect(readPartnerTypeface(written)).toEqual({ face: MONO, size: 12, leading: 1.2 });
  });

  it("reads the conversation's defaults where nothing is stored", () => {
    expect(readPartnerTypeface(store({}))).toEqual(DEFAULT_TYPEFACE);
    expect(readPartnerTypeface(store({ [PARTNER_FONT_KEY]: MONO }))).toEqual({
      face: MONO,
      size: DEFAULT_SIZE,
      leading: DEFAULT_LEADING,
    });
  });

  it("reads the conversation's defaults where something invalid is stored", () => {
    const nonsense = store({
      [PARTNER_FONT_KEY]: "x; color: red",
      [PARTNER_FONT_SIZE_KEY]: "enormous",
      [PARTNER_LINE_HEIGHT_KEY]: "roomy",
    });

    expect(readPartnerTypeface(nonsense)).toEqual(DEFAULT_TYPEFACE);
    expect(readPartnerTypeface(store({ [PARTNER_FONT_KEY]: '"Menlo"' })).face).toBe(INTERFACE);
    expect(readPartnerTypeface(store({ [PARTNER_FONT_SIZE_KEY]: "400" })).size).toBe(28);
    expect(readPartnerTypeface(store({ [PARTNER_FONT_SIZE_KEY]: "2" })).size).toBe(12);
    expect(readPartnerTypeface(store({ [PARTNER_LINE_HEIGHT_KEY]: "3" })).leading).toBe(DEFAULT_LEADING);
    expect(readPartnerTypeface(store({ [PARTNER_LINE_HEIGHT_KEY]: "1" })).leading).toBe(DEFAULT_LEADING);
  });

  it("keeps the conversation's face out of a store that refuses to keep it", () => {
    expect(readPartnerTypeface(throwing)).toEqual(DEFAULT_TYPEFACE);
    expect(readPartnerTypeface(null)).toEqual(DEFAULT_TYPEFACE);
    expect(() => {
      writePartnerTypeface({ face: MONO, size: 20, leading: 1.6 }, throwing);
      writePartnerTypeface({ face: MONO, size: 20, leading: 1.6 }, null);
    }).not.toThrow();
  });

  it("keeps the board's narrowing out of a store that refuses to keep it", () => {
    expect(readBacklogFilters(throwing)).toEqual(noFilters);
    expect(readDoneWindow(throwing)).toBe(DEFAULT_WINDOW);
    expect(() => {
      writeBacklogFilters(noFilters, throwing);
      writeDoneWindow(null, throwing);
    }).not.toThrow();
  });
});
