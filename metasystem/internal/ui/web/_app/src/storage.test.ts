import { describe, expect, it } from "vitest";

import {
  clampDockWidth,
  DEFAULT_DOCK_WIDTH,
  DOCK_KEY,
  DOCK_WIDTH_KEY,
  fitDockWidth,
  RAIL_KEY,
  readDockOpen,
  readDockWidth,
  readRailExpanded,
  readTheme,
  writeDockWidth,
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

describe("clampDockWidth", () => {
  it("clamps a stored width to what a dock may be", () => {
    expect(clampDockWidth("12")).toBe(320);
    expect(clampDockWidth("9999")).toBe(640);
    expect(clampDockWidth("abc")).toBe(400);
    expect(clampDockWidth("400")).toBe(400);
    expect(clampDockWidth("512")).toBe(512);
    expect(clampDockWidth(" 512 ")).toBe(512);
    expect(clampDockWidth(null)).toBe(DEFAULT_DOCK_WIDTH);
    expect(clampDockWidth("")).toBe(DEFAULT_DOCK_WIDTH);
    expect(clampDockWidth("-40")).toBe(DEFAULT_DOCK_WIDTH);
    expect(clampDockWidth("400px")).toBe(DEFAULT_DOCK_WIDTH);
  });
});

describe("fitDockWidth", () => {
  it("leaves the work area its minimum", () => {
    // 960 - 56 - 8 - 480 = 416, the widest dock a collapsed rail allows there.
    expect(fitDockWidth(640, 960, 56)).toBe(416);
    // 1,128 - 240 - 8 - 480 = 400: the expanded rail's own threshold.
    expect(fitDockWidth(640, 1128, 240)).toBe(400);
    // From 1,184 collapsed the dock can have all 640 it asks for.
    expect(fitDockWidth(640, 1184, 56)).toBe(640);
    expect(fitDockWidth(400, 1368, 240)).toBe(400);
  });

  it("never returns less than the dock's own minimum", () => {
    expect(fitDockWidth(400, 600, 56)).toBe(320);
  });
});

describe("the stored view state", () => {
  it("reads the four keys", () => {
    const written = store({
      [THEME_KEY]: "dark",
      [RAIL_KEY]: "collapsed",
      [DOCK_KEY]: "closed",
      [DOCK_WIDTH_KEY]: "512",
    });

    expect(readTheme(written)).toBe("dark");
    expect(readRailExpanded(written)).toBe(false);
    expect(readDockOpen(written)).toBe(false);
    expect(readDockWidth(written)).toBe(512);
  });

  it("defaults where nothing is stored", () => {
    const empty = store({});

    expect(readTheme(empty)).toBe("system");
    expect(readRailExpanded(empty)).toBe(true);
    expect(readDockOpen(empty)).toBe(true);
    expect(readDockWidth(empty)).toBe(400);
  });

  it("defaults where something invalid is stored", () => {
    const nonsense = store({ [THEME_KEY]: "{}", [RAIL_KEY]: "maybe", [DOCK_KEY]: "ajar", [DOCK_WIDTH_KEY]: "abc" });

    expect(readTheme(nonsense)).toBe("system");
    expect(readRailExpanded(nonsense)).toBe(true);
    expect(readDockOpen(nonsense)).toBe(true);
    expect(readDockWidth(nonsense)).toBe(400);
  });

  it("writes a width as whole pixels", () => {
    const written = store({});

    writeDockWidth(512.4, written);

    expect(readDockWidth(written)).toBe(512);
  });

  it("reads and writes a store that throws without surfacing an error", () => {
    expect(readTheme(throwing)).toBe("system");
    expect(readRailExpanded(throwing)).toBe(true);
    expect(readDockOpen(throwing)).toBe(true);
    expect(readDockWidth(throwing)).toBe(400);
    expect(() => {
      writeRailExpanded(false, throwing);
    }).not.toThrow();
    expect(() => {
      writeDockWidth(512, throwing);
    }).not.toThrow();
  });

  it("reads and writes nothing at all where there is no store", () => {
    expect(readDockWidth(null)).toBe(400);
    expect(() => {
      writeDockWidth(512, null);
    }).not.toThrow();
  });
});
