import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { HELP, helpText, sectionHelp, type HelpId } from "./terms";

/**
 * The register is the only place an explanation is written, and every icon on
 * screen finds its sentence there.
 *
 * An id naming a term nobody wrote would render a control with no explanation
 * behind it. TypeScript catches that wherever the id is a literal in a typed
 * position, which is most of them; this reads the tree as well, because the
 * three ways an id is written down are easy to see and cheap to check, and a
 * guard that reads the source catches a term deleted out from under a caller
 * even where a cast or a widened type let the compiler through.
 *
 * The three ways are: the control asked for by name, the prop a component
 * hands it, and the field a table carries. This file is excluded from its own
 * scan, because it has to write the patterns down in order to apply them.
 */

const SRC = path.resolve(fileURLToPath(import.meta.url), "..", "..");
const GUARD = "help/terms.test.ts";

/** The control by name, the prop a component is given, the field of a table. */
const WRITTEN = [/<Help\s+id="([^"]+)"/g, /\bhelp="([^"]+)"/g, /\bhelp:\s*"([^"]+)"/g];

function sourceFiles(relative = ""): string[] {
  const found: string[] = [];
  for (const entry of readdirSync(relative === "" ? SRC : path.join(SRC, relative), { withFileTypes: true })) {
    const next = relative === "" ? entry.name : `${relative}/${entry.name}`;
    if (entry.isDirectory()) {
      found.push(...sourceFiles(next));
      continue;
    }
    if (/\.tsx?$/.test(entry.name) && next !== GUARD) {
      found.push(next);
    }
  }
  return found.sort();
}

function asked(): { file: string; id: string }[] {
  const rows: { file: string; id: string }[] = [];
  for (const file of sourceFiles()) {
    const contents = readFileSync(path.join(SRC, file), "utf8");
    for (const pattern of WRITTEN) {
      for (const match of contents.matchAll(pattern)) {
        rows.push({ file, id: match[1] });
      }
    }
  }
  return rows;
}

describe("the help register", () => {
  it("has a term and a sentence for every id", () => {
    for (const [id, term] of Object.entries(HELP)) {
      expect({ id, named: term.term !== "" }).toEqual({ id, named: true });
      expect({ id, said: term.text !== "" }).toEqual({ id, said: true });
      expect({ id, sentence: term.text.endsWith(".") }).toEqual({ id, sentence: true });
    }
  });

  // A term is the name as it is written on the screen, so it is a name and not
  // a sentence: the popover heads itself with it.
  it("heads every explanation with a name rather than a sentence", () => {
    for (const [id, term] of Object.entries(HELP)) {
      expect({ id, stop: term.term.endsWith(".") }).toEqual({ id, stop: false });
    }
  });

  it("answers every id the source writes down", () => {
    const rows = asked();
    expect(rows.length).toBeGreaterThan(10);
    for (const row of rows) {
      expect({ ...row, known: Object.hasOwn(HELP, row.id) }).toEqual({ ...row, known: true });
    }
  });

  // The scan asserts its own reach before it asserts anything else: one id of
  // each of the three kinds, from three different files, or a pattern that
  // stopped matching would pass this file by saying nothing.
  it("is read from every way an id is written down", () => {
    const found = new Set(asked().map((row) => row.id));
    expect(found).toContain("partner");
    expect(found).toContain("priority");
    expect(found).toContain("lane-ready");
    expect(new Set(asked().map((row) => row.file)).size).toBeGreaterThan(2);
  });
});

describe("the sections' help", () => {
  it("names one term per project section, and none for Settings or the Partner", () => {
    expect(sectionHelp("overview")).toBe("overview");
    expect(sectionHelp("project")).toBe("project");
    expect(sectionHelp("backlog")).toBe("backlog");
    expect(sectionHelp("fleet")).toBe("fleet");
    expect(sectionHelp("application")).toBe("application");
    // The rail's Decisions is what the machinery asks a human to rule on, and
    // not the project's record of what was decided; they are two terms.
    expect(sectionHelp("decisions")).toBe("decisions-section");
    expect(sectionHelp("settings")).toBeNull();
    expect(sectionHelp("brain")).toBeNull();
    expect(sectionHelp(undefined)).toBeNull();
    expect(sectionHelp("nowhere")).toBeNull();
  });
});

describe("a term read as a sentence", () => {
  it("is the register's own words, and nothing for no term", () => {
    expect(helpText("lane-ready")).toBe(HELP["lane-ready"].text);
    expect(helpText(null)).toBe("");
  });

  it("answers for every id the register carries", () => {
    for (const id of Object.keys(HELP) as HelpId[]) {
      expect({ id, said: helpText(id) !== "" }).toEqual({ id, said: true });
    }
  });
});
