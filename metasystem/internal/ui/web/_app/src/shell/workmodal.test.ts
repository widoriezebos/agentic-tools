import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { DEFAULT_MODALITY, SIGN_IN_MODALITY } from "./workmodal";

/**
 * How modal a sheet is, and who is allowed to differ.
 *
 * The rule is one sentence — every sheet is modal for the work area, and
 * sign-in is modal for the window — and a rule with one exception is a rule
 * that acquires a second one the first time it is convenient. So the default
 * and the exception are asserted as values, and the source is read to hold
 * that sign-in is the only caller that names a mode at all.
 */

const SRC = path.resolve(fileURLToPath(import.meta.url), "..", "..");

/** The one sheet that blocks the whole window, and where it says so. */
const EXCEPTION = "shell/SignInSheet.tsx";

/**
 * The three sheet primitives. They name the mode because they take it as a
 * parameter and hand it on; every other file that names one is a sheet
 * choosing, which is what this guard counts.
 */
const PRIMITIVES = ["shell/Sheet.tsx", "backlog/Panel.tsx", "project/Sheet.tsx", "shell/workmodal.tsx"];

function sourceFiles(relative = ""): string[] {
  const found: string[] = [];
  for (const entry of readdirSync(relative === "" ? SRC : path.join(SRC, relative), { withFileTypes: true })) {
    const next = relative === "" ? entry.name : `${relative}/${entry.name}`;
    if (entry.isDirectory()) {
      found.push(...sourceFiles(next));
      continue;
    }
    if (/\.tsx?$/.test(entry.name) && !entry.name.endsWith(".test.ts") && !entry.name.endsWith(".test.tsx")) {
      found.push(next);
    }
  }
  return found.sort();
}

describe("how modal a sheet is", () => {
  it("is the work area by default, and the window for sign-in", () => {
    expect(DEFAULT_MODALITY).toBe("work");
    expect(SIGN_IN_MODALITY).toBe("window");
    expect(DEFAULT_MODALITY).not.toBe(SIGN_IN_MODALITY);
  });

  it("is chosen by exactly one sheet, and that sheet is sign-in", () => {
    const choosing = sourceFiles().filter(
      (file) => !PRIMITIVES.includes(file) && /\bmodal=\{/.test(readFileSync(path.join(SRC, file), "utf8")),
    );
    expect(choosing).toEqual([EXCEPTION]);
    expect(readFileSync(path.join(SRC, EXCEPTION), "utf8")).toContain("modal={SIGN_IN_MODALITY}");
  });

  it("gives every primitive the same default, written once", () => {
    for (const primitive of PRIMITIVES) {
      if (primitive === "shell/workmodal.tsx") {
        continue;
      }
      expect({ primitive, defaults: readFileSync(path.join(SRC, primitive), "utf8") }).toEqual({
        primitive,
        defaults: expect.stringContaining("modal = DEFAULT_MODALITY") as unknown as string,
      });
    }
  });
});
