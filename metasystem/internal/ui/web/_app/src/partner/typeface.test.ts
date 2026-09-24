import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import {
  clampLeading,
  clampSize,
  DEFAULT_LEADING,
  DEFAULT_SIZE,
  DEFAULT_TYPEFACE,
  fallbackKind,
  familyOf,
  INTERFACE,
  INTERFACE_STACK,
  isFaceName,
  isToken,
  LARGEST_LEADING,
  LARGEST_SIZE,
  leadingOf,
  leadingText,
  measuredInstalled,
  MONO,
  MONO_STACK,
  normalizeFace,
  normalizeLeading,
  normalizeSize,
  OFFERED,
  READING_STACK,
  SMALLEST_LEADING,
  SMALLEST_SIZE,
  sizeOf,
} from "./typeface";

const SRC = path.resolve(fileURLToPath(import.meta.url), "..", "..");

/**
 * What a face may be called, and what happens to a name that is not one.
 *
 * The validator is the whole of the guard: a name reaches a stylesheet as the
 * quoted half of a font-family declaration, so a name carrying a quote, a
 * semicolon or a colon would close the name or open a second declaration. It
 * is refused rather than escaped, here and in familyOf, and both are asserted
 * because the second is what actually protects the page.
 */
describe("a face name", () => {
  it("is letters, digits, spaces and hyphens", () => {
    expect(isFaceName("Meslo LG M for Powerline")).toBe(true);
    expect(isFaceName("SF Mono")).toBe(true);
    expect(isFaceName("Courier New")).toBe(true);
    expect(isFaceName("Helvetica Neue")).toBe(true);
    expect(isFaceName("IBM Plex Mono 42")).toBe(true);
    expect(isFaceName("Noto Sans-Mono")).toBe(true);
    expect(isFaceName("  Menlo  ")).toBe(true);
  });

  it("is refused where it carries anything else", () => {
    expect(isFaceName("x; color: red")).toBe(false);
    expect(isFaceName('"Menlo"')).toBe(false);
    expect(isFaceName("Menlo'")).toBe(false);
    expect(isFaceName("Menlo, serif")).toBe(false);
    expect(isFaceName("url(evil.woff2)")).toBe(false);
    expect(isFaceName("Menlo\\")).toBe(false);
    expect(isFaceName("Menlo}")).toBe(false);
    expect(isFaceName("Fira/Code")).toBe(false);
    expect(isFaceName("")).toBe(false);
    expect(isFaceName("   ")).toBe(false);
    expect(isFaceName("-leading-hyphen")).toBe(false);
    expect(isFaceName("M".repeat(65))).toBe(false);
  });

  it("names every face the chooser offers", () => {
    for (const offer of OFFERED) {
      expect({ name: offer.name, named: isFaceName(offer.name) }).toEqual({ name: offer.name, named: true });
    }
  });
});

describe("the font-family a face becomes", () => {
  it("is the stack behind each of the two this build carries", () => {
    expect(familyOf(INTERFACE)).toBe(INTERFACE_STACK);
    expect(familyOf(MONO)).toBe(MONO_STACK);
  });

  it("is the name, quoted, over the stack its kind falls back to", () => {
    expect(familyOf("Meslo LG M for Powerline")).toBe(`"Meslo LG M for Powerline", ${MONO_STACK}`);
    expect(familyOf("Menlo")).toBe(`"Menlo", ${MONO_STACK}`);
    expect(familyOf("Georgia")).toBe(`"Georgia", ${READING_STACK}`);
    expect(familyOf("Helvetica Neue")).toBe(`"Helvetica Neue", ${READING_STACK}`);
    expect(familyOf("  Menlo  ")).toBe(`"Menlo", ${MONO_STACK}`);
  });

  // A name this build refuses never reaches a stylesheet at all: the value it
  // would have been written into is the interface's own face instead.
  it("is the interface's own face where the name is refused", () => {
    expect(familyOf("x; color: red")).toBe(INTERFACE_STACK);
    expect(familyOf('Menlo"; background: url(x)')).toBe(INTERFACE_STACK);
    expect(familyOf("")).toBe(INTERFACE_STACK);
    for (const refused of ["x; color: red", '"Menlo"', "Menlo, serif", "url(evil)"]) {
      expect({ refused, carried: familyOf(refused).includes(refused) }).toEqual({ refused, carried: false });
    }
  });

  it("never carries a quote, a semicolon or a colon out of a name", () => {
    for (const written of [familyOf("x; color: red"), familyOf('"q"'), familyOf("Meslo LG M for Powerline")]) {
      expect(written.split('"').length % 2).toBe(1);
      expect(written).not.toContain(";");
      expect(written).not.toContain(":");
    }
  });
});

describe("what a face falls back to where the computer has not got it", () => {
  it("is Mono for the monospaced names and Reading for the rest", () => {
    expect(fallbackKind("Meslo LG M for Powerline")).toBe("mono");
    expect(fallbackKind("Menlo")).toBe("mono");
    expect(fallbackKind("SF Mono")).toBe("mono");
    expect(fallbackKind("Monaco")).toBe("mono");
    expect(fallbackKind("Fira Code")).toBe("mono");
    expect(fallbackKind("Source Code Pro")).toBe("mono");
    expect(fallbackKind("JetBrains Mono")).toBe("mono");
    expect(fallbackKind("Consolas")).toBe("mono");
    expect(fallbackKind("Courier New")).toBe("mono");
    expect(fallbackKind("Georgia")).toBe("reading");
    expect(fallbackKind("Helvetica Neue")).toBe("reading");
  });

  it("answers from the offered list however the name was typed", () => {
    expect(fallbackKind("menlo")).toBe("mono");
    expect(fallbackKind("  MESLO LG M FOR POWERLINE ")).toBe("mono");
  });

  // A family this build has never heard of could be anything, so it falls back
  // to the face prose is read in rather than pretending to know.
  it("is Reading for a name the chooser does not offer", () => {
    expect(fallbackKind("Comic Sans MS")).toBe("reading");
    expect(fallbackKind("Iosevka")).toBe("reading");
  });
});

describe("the size", () => {
  it("is held inside what the stepper offers", () => {
    expect(clampSize(12)).toBe(SMALLEST_SIZE);
    expect(clampSize(28)).toBe(LARGEST_SIZE);
    expect(clampSize(11)).toBe(SMALLEST_SIZE);
    expect(clampSize(0)).toBe(SMALLEST_SIZE);
    expect(clampSize(-40)).toBe(SMALLEST_SIZE);
    expect(clampSize(29)).toBe(LARGEST_SIZE);
    expect(clampSize(400)).toBe(LARGEST_SIZE);
    expect(clampSize(18.4)).toBe(18);
  });

  it("is the default where it is not a number at all", () => {
    expect(clampSize(Number.NaN)).toBe(DEFAULT_SIZE);
    expect(clampSize(Number.POSITIVE_INFINITY)).toBe(DEFAULT_SIZE);
  });

  it("reaches a stylesheet as pixels, clamped", () => {
    expect(sizeOf(18)).toBe("18px");
    expect(sizeOf(999)).toBe("28px");
  });
});

/**
 * The rhythm the conversation is read at (Wido, 2026-09-24: "the distance
 * between the lines, can I also change that?").
 *
 * The arithmetic is the part that is easy to get wrong: a tenth added to a
 * tenth in binary is not a tenth, so the stepper would drift to 1.7999999 and
 * write that to storage. Every step is rounded back to the tenth it stepped
 * in, which is what these assert.
 */
describe("the line spacing", () => {
  it("is held inside what the stepper offers", () => {
    expect(clampLeading(1.2)).toBe(SMALLEST_LEADING);
    expect(clampLeading(2)).toBe(LARGEST_LEADING);
    expect(clampLeading(1.1)).toBe(SMALLEST_LEADING);
    expect(clampLeading(0)).toBe(SMALLEST_LEADING);
    expect(clampLeading(-3)).toBe(SMALLEST_LEADING);
    expect(clampLeading(2.1)).toBe(LARGEST_LEADING);
    expect(clampLeading(40)).toBe(LARGEST_LEADING);
  });

  it("steps in tenths, without the drift a tenth carries", () => {
    expect(clampLeading(1.5 + 0.1)).toBe(1.6);
    expect(clampLeading(1.7 + 0.1)).toBe(1.8);
    expect(clampLeading(1.3 - 0.1)).toBe(1.2);
    expect(clampLeading(1.74)).toBe(1.7);
    expect(clampLeading(1.75)).toBe(1.8);
  });

  it("is the default where it is not a number at all", () => {
    expect(clampLeading(Number.NaN)).toBe(DEFAULT_LEADING);
    expect(clampLeading(Number.POSITIVE_INFINITY)).toBe(DEFAULT_LEADING);
    expect(DEFAULT_LEADING).toBe(1.5);
  });

  it("reaches a stylesheet as a multiple, with no unit on it", () => {
    expect(leadingOf(1.5)).toBe("1.5");
    expect(leadingOf(1.7 + 0.1)).toBe("1.8");
    expect(leadingOf(9)).toBe("2");
  });

  it("is said out loud to the tenth, however round it is", () => {
    expect(leadingText(1.5)).toBe("1.5");
    expect(leadingText(2)).toBe("2.0");
    expect(leadingText(1.2)).toBe("1.2");
  });

  // A spacing outside the stepper's range was never offered by anything this
  // build drew, so it came from somewhere else and the default answers it —
  // where a size outside its range is one a human once had, and is clamped.
  it("is the default where what is stored was never offered", () => {
    expect(normalizeLeading("1.8")).toBe(1.8);
    expect(normalizeLeading(" 1.2 ")).toBe(1.2);
    expect(normalizeLeading("2")).toBe(2);
    expect(normalizeLeading("1.1")).toBe(DEFAULT_LEADING);
    expect(normalizeLeading("2.4")).toBe(DEFAULT_LEADING);
    expect(normalizeLeading("loose")).toBe(DEFAULT_LEADING);
    expect(normalizeLeading("-1.5")).toBe(DEFAULT_LEADING);
    expect(normalizeLeading("")).toBe(DEFAULT_LEADING);
    expect(normalizeLeading(null)).toBe(DEFAULT_LEADING);
    expect(normalizeLeading(undefined)).toBe(DEFAULT_LEADING);
  });
});

describe("what is read back out of storage", () => {
  it("keeps the two tokens and any name this build would have written", () => {
    expect(normalizeFace(INTERFACE)).toBe(INTERFACE);
    expect(normalizeFace(MONO)).toBe(MONO);
    expect(normalizeFace("Meslo LG M for Powerline")).toBe("Meslo LG M for Powerline");
    expect(normalizeFace("  Menlo ")).toBe("Menlo");
    expect(isToken(INTERFACE)).toBe(true);
    expect(isToken("Menlo")).toBe(false);
  });

  // The chooser's Reading entry was the interface's own face drawn twice, and
  // is gone. A browser that chose it comes back to the face it was drawing.
  it("reads the gone Reading entry as the interface's own face", () => {
    expect(normalizeFace("reading")).toBe(INTERFACE);
    expect(isToken("reading")).toBe(false);
  });

  it("reads the interface's own face where what is stored is not a face", () => {
    expect(normalizeFace("x; color: red")).toBe(INTERFACE);
    expect(normalizeFace("{}")).toBe(INTERFACE);
    expect(normalizeFace("")).toBe(INTERFACE);
    expect(normalizeFace(null)).toBe(INTERFACE);
    expect(normalizeFace(undefined)).toBe(INTERFACE);
  });

  it("reads the default size where what is stored is not one, and clamps the rest", () => {
    expect(normalizeSize("18")).toBe(18);
    expect(normalizeSize(" 20 ")).toBe(20);
    expect(normalizeSize("99")).toBe(LARGEST_SIZE);
    expect(normalizeSize("4")).toBe(SMALLEST_SIZE);
    expect(normalizeSize("large")).toBe(DEFAULT_SIZE);
    expect(normalizeSize("16.5")).toBe(DEFAULT_SIZE);
    expect(normalizeSize("-18")).toBe(DEFAULT_SIZE);
    expect(normalizeSize(null)).toBe(DEFAULT_SIZE);
  });

  it("opens at the interface's own face, sixteen pixels and one and a half", () => {
    expect(DEFAULT_TYPEFACE).toEqual({ face: INTERFACE, size: DEFAULT_SIZE, leading: DEFAULT_LEADING });
    expect(DEFAULT_SIZE).toBe(16);
  });
});

/**
 * The measurement, with the canvas replaced by a table. A face the browser has
 * got renders the probe at its own width; one it has not got renders it at the
 * fallback's, and the two measurements agree.
 */
describe("whether a face is installed", () => {
  const widths: Record<string, number> = {
    monospace: 100,
    serif: 90,
    '"Menlo", monospace': 104,
    '"Menlo", serif': 104,
    '"Nowhere", monospace': 100,
    '"Nowhere", serif': 90,
    // A face whose metrics match one bare fallback and not the other is still
    // installed, which is why two fallbacks are measured and not one.
    '"Twin", monospace': 100,
    '"Twin", serif': 111,
  };
  const measure = (family: string) => widths[family] ?? 0;

  it("is yes where the wanted face measures differently from a bare fallback", () => {
    expect(measuredInstalled(measure, "Menlo")).toBe(true);
    expect(measuredInstalled(measure, "Twin")).toBe(true);
  });

  it("is no where it measures the same as every bare fallback", () => {
    expect(measuredInstalled(measure, "Nowhere")).toBe(false);
  });

  it("is no for a name this build would refuse", () => {
    expect(measuredInstalled(measure, "x; color: red")).toBe(false);
  });
});

/**
 * The three stacks are written here and again in the stylesheets, which is two
 * copies of one fact. This holds them to each other, the way themeScript.test
 * holds the no-flash script to the theme module.
 */
describe("the stacks this module names", () => {
  it("are the stacks the stylesheets declare", () => {
    const fonts = readFileSync(path.join(SRC, "fonts.css"), "utf8");
    expect(fonts).toContain(`font-family: ${INTERFACE_STACK};`);
    const shell = readFileSync(path.join(SRC, "shell", "shell.css"), "utf8");
    expect(shell).toContain(`font-family: ${MONO_STACK};`);
  });

  // Two rules read the chooser's properties and no third one does: the
  // conversation's root and the composer's field.
  it("are read by the conversation and the composer's field, and by nothing else", () => {
    const partner = readFileSync(path.join(SRC, "partner", "partner.css"), "utf8");
    const shell = readFileSync(path.join(SRC, "shell", "shell.css"), "utf8");
    expect(partner).toContain("font-size: var(--ms-partner-size, 16px);");
    expect(shell).toContain("font-size: var(--ms-partner-size, 16px);");
    const reading = readFileSync(path.join(SRC, "project", "reading.css"), "utf8");
    expect(reading).not.toContain("--ms-partner-");
  });
});
