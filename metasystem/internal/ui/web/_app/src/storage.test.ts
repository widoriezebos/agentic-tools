import { describe, expect, it } from "vitest";

import {
  DOCK_KEY,
  RAIL_KEY,
  readDockOpen,
  readRailExpanded,
  readTheme,
  writeDockOpen,
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
  it("reads the three keys", () => {
    const written = store({
      [THEME_KEY]: "dark",
      [RAIL_KEY]: "collapsed",
      [DOCK_KEY]: "closed",
    });

    expect(readTheme(written)).toBe("dark");
    expect(readRailExpanded(written)).toBe(false);
    expect(readDockOpen(written)).toBe(false);
  });

  // The drawer is closed until a human opens it: only the word it was opened
  // under opens it again.
  it("defaults where nothing is stored", () => {
    const empty = store({});

    expect(readTheme(empty)).toBe("system");
    expect(readRailExpanded(empty)).toBe(true);
    expect(readDockOpen(empty)).toBe(false);
  });

  it("defaults where something invalid is stored", () => {
    const nonsense = store({ [THEME_KEY]: "{}", [RAIL_KEY]: "maybe", [DOCK_KEY]: "ajar" });

    expect(readTheme(nonsense)).toBe("system");
    expect(readRailExpanded(nonsense)).toBe(true);
    expect(readDockOpen(nonsense)).toBe(false);
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
    expect(() => {
      writeRailExpanded(false, throwing);
    }).not.toThrow();
    expect(() => {
      writeDockOpen(false, throwing);
    }).not.toThrow();
  });

  it("reads and writes nothing at all where there is no store", () => {
    expect(readDockOpen(null)).toBe(false);
    expect(() => {
      writeDockOpen(false, null);
    }).not.toThrow();
  });
});
