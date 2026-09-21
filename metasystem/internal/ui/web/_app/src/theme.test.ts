import { describe, expect, it } from "vitest";

import { effectiveTheme, normalizeTheme, THEME_ATTRIBUTE, THEME_KEY } from "./theme";

describe("normalizeTheme", () => {
  it("keeps the three preferences", () => {
    expect(normalizeTheme("system")).toBe("system");
    expect(normalizeTheme("light")).toBe("light");
    expect(normalizeTheme("dark")).toBe("dark");
  });

  it("reads anything else as following the system", () => {
    for (const value of ["", "Dark", "auto", "{}", null, undefined]) {
      expect({ value, preference: normalizeTheme(value) }).toEqual({ value, preference: "system" });
    }
  });
});

describe("effectiveTheme", () => {
  it("follows the system only when the human has not chosen", () => {
    expect(effectiveTheme("system", true)).toBe("dark");
    expect(effectiveTheme("system", false)).toBe("light");
    expect(effectiveTheme("light", true)).toBe("light");
    expect(effectiveTheme("dark", false)).toBe("dark");
  });
});

describe("the names the no-flash script shares", () => {
  it("are the ones this module exports", () => {
    expect(THEME_KEY).toBe("ms.ui.theme");
    expect(THEME_ATTRIBUTE).toBe("data-theme");
  });
});
