import { existsSync, readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * The cut guard.
 *
 * The first cut makes one request, GET /api/workspace, on load and on Retry,
 * and nothing else: no event stream, no socket, no XHR, no beacon, no polling
 * timer, and no window event that refetches. The connection indicator, the
 * health poll, the rebuilt-executable notice, and the invalidation
 * subscription are the second cut's, and none of their files exists yet.
 *
 * The guard reads code, not text. Every file is tokenized first: comments are
 * dropped, string and template literals are collected separately from the
 * identifiers, and a regular expression is skipped as a literal rather than
 * read as division. So a mention in a comment is not a call, and neither is a
 * fixture string; an alias (`const send = fetch`) still names fetch and is
 * caught; and a property-handler spelling (`window.onfocus = …`) names onfocus
 * and is caught too. What the guard cannot see through — a name assembled at
 * runtime — is covered by the string rules below, which refuse the names
 * themselves as literals.
 *
 * This file is excluded from its own scan, deliberately: it has to write the
 * forbidden names down in order to forbid them. Everything else under src/ is
 * scanned, and the scan asserts its own reach before it asserts anything else.
 */

const SRC = path.resolve(fileURLToPath(import.meta.url), "..");
const GUARD = "cuts.test.ts";

/** Every way this build could reach the network. */
const NETWORK = ["fetch", "EventSource", "WebSocket", "XMLHttpRequest", "sendBeacon"];

/** A timer is how a poll is written when it cannot be called a poll. */
const TIMERS = ["setInterval", "setTimeout"];

/** The window events a refetch hides behind. */
const LIFECYCLE_EVENTS = ["focus", "online", "offline", "visibilitychange", "pageshow"];
const LIFECYCLE_HANDLERS = ["onfocus", "ononline", "onoffline", "onvisibilitychange", "onpageshow"];

const CALL_SITE = "shell/workspace.ts";
const RESOURCE = "/api/workspace";
const HEALTH = "/-/health";

/** The second cut's files, which do not exist in this one. */
const SECOND_CUT = ["shell/ConnectionIndicator.tsx", "shell/Notice.tsx", "shell/health.ts", "shell/events.ts"];

type Scanned = {
  /** Every identifier in the code, with how often it occurs. */
  identifiers: Map<string, number>;
  /** Every string and template chunk, as written. */
  strings: string[];
};

/** True for the first character of an identifier. */
function startsIdentifier(character: string): boolean {
  return /[A-Za-z_$]/.test(character);
}

function continuesIdentifier(character: string): boolean {
  return /[A-Za-z0-9_$]/.test(character);
}

/**
 * Reads a quoted string from `index`, which is the quote, and returns its
 * contents and the offset after the closing quote.
 */
function readQuoted(source: string, index: number, quote: string): [string, number] {
  let value = "";
  let cursor = index + 1;
  while (cursor < source.length) {
    const character = source[cursor];
    if (character === "\\") {
      value += source[cursor + 1] ?? "";
      cursor += 2;
      continue;
    }
    if (character === quote) {
      return [value, cursor + 1];
    }
    value += character;
    cursor += 1;
  }
  return [value, cursor];
}

/** Skips a regular expression literal from its opening slash. */
function skipRegex(source: string, index: number): number {
  let cursor = index + 1;
  let inClass = false;
  while (cursor < source.length) {
    const character = source[cursor];
    if (character === "\\") {
      cursor += 2;
      continue;
    }
    if (character === "[") {
      inClass = true;
    } else if (character === "]") {
      inClass = false;
    } else if (character === "/" && !inClass) {
      cursor += 1;
      break;
    } else if (character === "\n") {
      break;
    }
    cursor += 1;
  }
  while (cursor < source.length && /[a-z]/.test(source[cursor])) {
    cursor += 1;
  }
  return cursor;
}

export function scan(source: string): Scanned {
  const identifiers = new Map<string, number>();
  const strings: string[] = [];
  const templateDepths: number[] = [];
  let braces = 0;
  let index = 0;
  let mode: "code" | "template" = "code";
  // A slash after a value is division; after anything else it opens a regular
  // expression. That is the whole ambiguity, and this is the whole rule.
  let afterValue = false;

  while (index < source.length) {
    if (mode === "template") {
      let chunk = "";
      while (index < source.length) {
        const character = source[index];
        if (character === "\\") {
          chunk += source[index + 1] ?? "";
          index += 2;
          continue;
        }
        if (character === "`") {
          index += 1;
          mode = "code";
          afterValue = true;
          break;
        }
        if (character === "$" && source[index + 1] === "{") {
          index += 2;
          templateDepths.push(braces);
          mode = "code";
          afterValue = false;
          break;
        }
        chunk += character;
        index += 1;
      }
      strings.push(chunk);
      continue;
    }

    const character = source[index];
    const next = source[index + 1] ?? "";

    if (character === "/" && next === "/") {
      while (index < source.length && source[index] !== "\n") {
        index += 1;
      }
      continue;
    }
    if (character === "/" && next === "*") {
      index += 2;
      while (index < source.length && !(source[index] === "*" && source[index + 1] === "/")) {
        index += 1;
      }
      index += 2;
      continue;
    }
    if (character === '"' || character === "'") {
      const [value, after] = readQuoted(source, index, character);
      strings.push(value);
      index = after;
      afterValue = true;
      continue;
    }
    if (character === "`") {
      index += 1;
      mode = "template";
      continue;
    }
    if (character === "}" && templateDepths.length > 0 && braces === templateDepths[templateDepths.length - 1]) {
      templateDepths.pop();
      index += 1;
      mode = "template";
      continue;
    }
    if (character === "{") {
      braces += 1;
      index += 1;
      afterValue = false;
      continue;
    }
    if (character === "}") {
      braces -= 1;
      index += 1;
      afterValue = true;
      continue;
    }
    if (character === "/" && !afterValue) {
      index = skipRegex(source, index);
      afterValue = true;
      continue;
    }
    if (startsIdentifier(character)) {
      let name = "";
      while (index < source.length && continuesIdentifier(source[index])) {
        name += source[index];
        index += 1;
      }
      identifiers.set(name, (identifiers.get(name) ?? 0) + 1);
      afterValue = true;
      continue;
    }
    if (/[0-9]/.test(character)) {
      while (index < source.length && /[0-9a-zA-Z_.]/.test(source[index])) {
        index += 1;
      }
      afterValue = true;
      continue;
    }
    // Whitespace says nothing about what came before it, so it must not make
    // the next slash look like the start of a regular expression.
    if (/\s/.test(character)) {
      index += 1;
      continue;
    }
    // A slash after any of these is division, or the close of a JSX tag, and
    // never a regular expression. Reading it as division is the safe error:
    // it keeps scanning code, where a hidden call would still be seen.
    afterValue = character === ")" || character === "]" || character === ">" || character === "<";
    index += 1;
  }

  return { identifiers, strings };
}

function sourceFiles(): string[] {
  const found: string[] = [];
  const walk = (relative: string) => {
    for (const entry of readdirSync(relative === "" ? SRC : path.join(SRC, relative), { withFileTypes: true })) {
      const next = relative === "" ? entry.name : `${relative}/${entry.name}`;
      if (entry.isDirectory()) {
        walk(next);
        continue;
      }
      if (/\.(ts|tsx|js|jsx|mjs|cjs)$/.test(entry.name) && next !== GUARD) {
        found.push(next);
      }
    }
  };
  walk("");
  return found.sort();
}

const files = sourceFiles();
const scanned = new Map(files.map((file) => [file, scan(readFileSync(path.join(SRC, file), "utf8"))]));

function filesNaming(names: string[]): string[] {
  return files.filter((file) => names.some((name) => (scanned.get(file)?.identifiers.get(name) ?? 0) > 0));
}

/** Every string literal under src/ that a rule objects to, named by its file. */
function stringsMatching(objectionable: (value: string) => boolean): string[] {
  const offenders: string[] = [];
  for (const file of files) {
    for (const value of scanned.get(file)?.strings ?? []) {
      if (objectionable(value)) {
        offenders.push(`${file}: ${value}`);
      }
    }
  }
  return offenders;
}

describe("the scan itself", () => {
  it("reads the whole source tree, and not its own source", () => {
    expect(files).toContain(CALL_SITE);
    expect(files).toContain("routes.ts");
    expect(files.length).toBeGreaterThan(10);
    expect(files).not.toContain(GUARD);
    expect(existsSync(path.join(SRC, GUARD))).toBe(true);
  });

  it("reads code, and not comments, strings, or regular expressions", () => {
    const fixture = [
      "// fetch(url) in a comment",
      "/* new EventSource(url) in a block comment */",
      'const mention = "setInterval";',
      "const pattern = /new WebSocket\\//;",
      "const ratio = total / count / 2;",
      "const send = fetch;",
      "const label = `an ${XMLHttpRequest} in a template`;",
    ].join("\n");

    const result = scan(fixture);

    expect(result.identifiers.get("fetch")).toBe(1);
    expect(result.identifiers.get("EventSource")).toBeUndefined();
    expect(result.identifiers.get("setInterval")).toBeUndefined();
    expect(result.identifiers.get("WebSocket")).toBeUndefined();
    expect(result.identifiers.get("XMLHttpRequest")).toBe(1);
    expect(result.strings).toContain("setInterval");
    expect(result.identifiers.get("count")).toBe(1);
  });
});

describe("the first cut", () => {
  it("reaches the network from exactly one place", () => {
    const sites = new Map<string, number>();
    for (const file of files) {
      const total = NETWORK.reduce((count, name) => count + (scanned.get(file)?.identifiers.get(name) ?? 0), 0);
      if (total > 0) {
        sites.set(file, total);
      }
    }

    expect([...sites.entries()]).toEqual([[CALL_SITE, 1]]);
    expect(scanned.get(CALL_SITE)?.identifiers.get("fetch")).toBe(1);
    expect(scanned.get(CALL_SITE)?.strings).toContain(RESOURCE);
  });

  it("opens no stream, socket, request object, or beacon, under any name", () => {
    expect(filesNaming(["EventSource"])).toEqual([]);
    expect(filesNaming(["WebSocket"])).toEqual([]);
    expect(filesNaming(["XMLHttpRequest"])).toEqual([]);
    expect(filesNaming(["sendBeacon"])).toEqual([]);
    // A name assembled at runtime still has to be written down somewhere.
    expect(stringsMatching((value) => NETWORK.includes(value.trim()))).toEqual([]);
  });

  it("sets no timer", () => {
    expect(filesNaming(["setInterval"])).toEqual([]);
    expect(filesNaming(TIMERS)).toEqual([]);
    expect(stringsMatching((value) => TIMERS.includes(value.trim()))).toEqual([]);
  });

  it("listens for no lifecycle event, as a listener or as a handler property", () => {
    expect(filesNaming(LIFECYCLE_HANDLERS)).toEqual([]);
    expect(stringsMatching((value) => LIFECYCLE_EVENTS.includes(value.trim()))).toEqual([]);
  });

  it("names no health path", () => {
    expect(stringsMatching((value) => value.includes(HEALTH))).toEqual([]);
  });

  it("carries none of the second cut's files", () => {
    const present = SECOND_CUT.filter((file) => existsSync(path.join(SRC, file)));
    expect(present).toEqual([]);
  });
});
