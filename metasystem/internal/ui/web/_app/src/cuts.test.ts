import { existsSync, readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * The cut guard.
 *
 * This build requests only what a human asked for: each listed call site makes
 * its listed calls when its view mounts and again on Refresh or Retry, and
 * nothing else happens on its own. No socket, no XHR, no beacon, no polling
 * timer, and no window event that refetches. Freshness is the server's: its
 * own loop keeps the accepted ledger current, so the page never polls to stay
 * up to date. The connection indicator, the health poll, the rebuilt-executable
 * notice, and the invalidation subscription are the second cut's, and none of
 * their files exists yet.
 *
 * There is one event stream, and it is the one exception this guard carries.
 * The steward's notifications are not something a human asks for: they are the
 * steward speaking, at a moment nobody on this side chose, and the only ways
 * to learn of one are to be told or to keep asking. Being told is one stream,
 * opened once, that the browser reconnects by itself; asking is the poll this
 * whole guard exists to prevent. So EventSource is allowed in exactly one
 * file, for exactly one resource, and the rule that it is allowed nowhere else
 * is asserted rather than relaxed.
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
 * A name that belongs to some value rather than naming one in scope is a
 * field, not a global: a response whose payload carries a `fetch` clause is
 * describing what the server did, and declaring or reading that clause is not
 * a request. So a name reached through a dot, and a name that opens a member
 * of an object or a type, are fields. A field that is called
 * (`window.fetch(…)`) is a call all the same, and is counted.
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

/**
 * Every place this build reaches the network: the file, how many call sites it
 * has, and the resources it names.
 *
 * This list is appended to and never replaced. A slice that adds a reader adds
 * its own row to the one the slice before it left, so the diff shows a new way
 * to the network as an entry a reviewer reads rather than as a count that
 * quietly went up, and every earlier call site keeps being counted.
 */
const CALL_SITES: readonly (readonly [string, number, readonly string[]])[] = [
  ["shell/workspace.ts", 1, ["/api/workspace"]],
  // The board's four acts join the backlog's one read at the call site it
  // already had: an act is the same request with a body, so there is still
  // exactly one place in this build that reaches the network from here. Three
  // of the act routes name a goal by its id between the prefix and their own
  // suffix, which is why those suffixes are listed on their own; the fourth
  // opens a goal and so names the collection, which has no id in it at all.
  // The query is Refresh's and nobody else's: it asks the server to fetch the
  // canonical branch once before it answers. It is the same resource and the
  // same call site — a read a human asked for, not a read anything repeats.
  [
    "backlog/api.ts",
    1,
    [
      "/api/backlog",
      "?fetch=1",
      "/api/backlog/goals",
      "/api/backlog/goals/",
      "/approve",
      "/withdraw",
      "/priority",
    ],
  ],
  // The Project section's writes join its two reads at the one call site it
  // already had: a write is the same request with a body, so there is still
  // exactly one place in this build that reaches the network from here. The
  // two status routes name an id between their prefix and the suffix both
  // share, which is why "/status" is listed on its own, and naming a goal on a
  // record names an id the same way, which is why "/goals" is. Editing a
  // document in place adds two more resources to the same call site and no
  // second call site: the save names a document by the read route's own id
  // with "/edit" after it, and the render names its own path and no document
  // at all.
  [
    "project/api.ts",
    1,
    [
      "/api/project",
      "/api/documents/",
      "/api/project/records",
      "/api/project/questions",
      "/status",
      "/goals",
      "/edit",
      "/api/documents/preview",
    ],
  ],
  // Who the server is acting as, and the two acts that change it. It is read
  // beside the workspace and again only when a sign-in or a sign-out answers
  // with a new one: a session changes when a human changes it, so there is
  // nothing here to poll for. The three resources are written out in full
  // rather than composed, so this row names what the file actually requests.
  ["shell/session.ts", 1, ["/api/session", "/api/session/sign-in", "/api/session/sign-out"]],
  // The steward's notification history. One read on load, one more when a
  // human presses "Load older", both through the one request below; what
  // arrives after the load arrives on the stream named beside this list,
  // which is why there is no second read here and no timer anywhere.
  ["notifications/api.ts", 1, ["/api/notifications"]],
  // The landing page. One read when the pane mounts and one more when a human
  // presses the section's refresh in the header, both through the one request
  // below. The page holds no timer and no second reader: everything on it is
  // composed on the server from what that server already reads, so there is
  // one resource here and not five.
  ["overview/api.ts", 1, ["/api/overview"]],
  // The Project Partner. Three requests through one call site: the
  // conversation, read when the page loads and again on every reconnect of the
  // one stream below; a send, when a human presses Send; and a stop, when they
  // press Stop. Nothing here polls — the turn's own beats arrive on that
  // stream — and nothing here renders Markdown either: a complete answer goes
  // through the reader's own preview route, which is project/api.ts's call
  // site and not a second one.
  ["partner/api.ts", 1, ["/api/partner", "/api/partner/turns", "/stop"]],
];

/**
 * The one place this build opens an event stream, and the one resource it
 * opens. Both are asserted below: the file, so no second file can open one,
 * and the resource, so this one cannot quietly become a stream over something
 * else.
 */
const STREAM_SITE = "notifications/stream.ts";
const STREAM_RESOURCE = "/api/notifications/stream";

const HEALTH = "/-/health";

/**
 * Every way a string could become markup. The document view renders elements
 * and nothing else: the engine parses Markdown into a typed tree, React builds
 * the elements, and no HTML string is constructed anywhere under src/.
 */
const INJECTION = [
  "dangerouslySetInnerHTML",
  "innerHTML",
  "outerHTML",
  "insertAdjacentHTML",
  "DOMParser",
  "createContextualFragment",
];

/**
 * `write` is on the same list, because document.write parses a string as
 * markup. One file predates the rule and has its own local `write`, which
 * stores a preference, so it is named here rather than the rule dropped — and
 * the rule that keeps the carve-out honest is asserted beside it: a file
 * allowed to say `write` may not also say `document`, so the pair cannot meet.
 */
const WRITE = "write";
const WRITES_A_PREFERENCE = "storage.ts";

/** The second cut's files, which do not exist in this one. */
const SECOND_CUT = ["shell/ConnectionIndicator.tsx", "shell/Notice.tsx", "shell/health.ts", "shell/events.ts"];

type Scanned = {
  /**
   * Every identifier in the code that names a value in scope, with how often
   * it occurs: a bare name, or a field that is called.
   */
  identifiers: Map<string, number>;
  /** Every field read through a dot without being called. */
  fields: Map<string, number>;
  /** Every string and template chunk, as written. */
  strings: string[];
};

/** True when the name starting here is reached through a dot. */
function reachedThroughADot(source: string, start: number): boolean {
  let back = start - 1;
  while (back >= 0 && /\s/.test(source[back])) {
    back -= 1;
  }
  // A spread is three dots and reaches nothing.
  return back >= 0 && source[back] === "." && source[back - 1] !== ".";
}

/**
 * True when a DOTTED member is used rather than read as inert data.
 *
 * This fails closed: it names the characters after which nothing can invoke the
 * value, and treats everything else as use. Sol's F4 found four evasions
 * against a rule that asked only "is the next character a `(`" -- `?.()`,
 * `.bind(x)`, a comment between the name and its parentheses, and a
 * destructured binding. The first three are closed here; the fourth is closed
 * in isMemberName.
 *
 * A dot is the one ambiguous follower: `ledger.fetch.outcome` walks further
 * into data, while `window.fetch.bind(w)` reaches a way to call the value. So
 * the name after the dot decides.
 */
const INERT_AFTER_A_MEMBER = [",", ";", ")", "}", "]"];
const INVOKERS = ["bind", "call", "apply"];

function isUsed(source: string, end: number): boolean {
  const next = nextCharacter(source, end);
  if (INERT_AFTER_A_MEMBER.includes(next)) {
    return false;
  }
  if (next === "." || next === "?") {
    const after = source.indexOf(next, end) + (next === "?" ? 2 : 1);
    if (nextCharacter(source, after) === "(") {
      return true;
    }
    let index = after;
    while (index < source.length && /\s/.test(source[index])) {
      index += 1;
    }
    let word = "";
    while (index < source.length && /[A-Za-z_$0-9]/.test(source[index])) {
      word += source[index];
      index += 1;
    }
    return INVOKERS.includes(word);
  }
  return true;
}

/**
 * True when the name between these offsets is a member's own name: it opens a
 * member of an object or a type and is followed by that member's value. The
 * middle of a conditional (`ready ? fetch : none`) is followed by a colon too,
 * and is a reference, so what comes before the name decides.
 *
 * A destructuring pattern looks identical to an object literal from the name
 * outwards -- `const { fetch: send } = window` binds the global under a new
 * name, and Sol's F4 showed it escaping as a member. So the brace that opens
 * the group is found and what precedes IT decides: a binder there makes this a
 * pattern, which is a reference, not a declaration.
 */
function isMemberName(source: string, start: number, end: number): boolean {
  if (nextCharacter(source, end) !== ":") {
    return false;
  }
  let back = start - 1;
  while (back >= 0 && /\s/.test(source[back])) {
    back -= 1;
  }
  if (back < 0) {
    return true;
  }
  if (source[back] === "{") {
    return !opensABindingPattern(source, back);
  }
  if (source[back] === "," || source[back] === ";") {
    const brace = enclosingBrace(source, back);
    return brace < 0 || !opensABindingPattern(source, brace);
  }
  return false;
}

/** The unbalanced `{` that opens the group this offset sits in, or -1. */
function enclosingBrace(source: string, from: number): number {
  let depth = 0;
  for (let index = from; index >= 0; index -= 1) {
    const character = source[index];
    if (character === "}") {
      depth += 1;
    } else if (character === "{") {
      if (depth === 0) {
        return index;
      }
      depth -= 1;
    }
  }
  return -1;
}

/** True when the word before this `{` makes it a binding pattern. */
function opensABindingPattern(source: string, brace: number): boolean {
  let back = brace - 1;
  while (back >= 0 && /\s/.test(source[back])) {
    back -= 1;
  }
  if (back < 0) {
    return false;
  }
  // A parameter list destructures too: function f({ fetch }) {}.
  if (source[back] === "(" || source[back] === ",") {
    return true;
  }
  const end = back + 1;
  while (back >= 0 && /[A-Za-z_$]/.test(source[back])) {
    back -= 1;
  }
  return ["const", "let", "var"].includes(source.slice(back + 1, end));
}

function nextCharacter(source: string, from: number): string {
  let ahead = from;
  while (ahead < source.length && /\s/.test(source[ahead])) {
    ahead += 1;
  }
  return source[ahead] ?? "";
}

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
  const fields = new Map<string, number>();
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
      const start = index;
      const dotted = reachedThroughADot(source, index);
      let name = "";
      while (index < source.length && continuesIdentifier(source[index])) {
        name += source[index];
        index += 1;
      }
      const declared = isMemberName(source, start, index);
      if (declared || (dotted && !isUsed(source, index))) {
        fields.set(name, (fields.get(name) ?? 0) + 1);
      } else {
        identifiers.set(name, (identifiers.get(name) ?? 0) + 1);
      }
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

  return { identifiers, fields, strings };
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
    for (const [file] of CALL_SITES) {
      expect(files).toContain(file);
    }
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
    expect(result.fields.get("fetch")).toBeUndefined();
    expect(result.identifiers.get("EventSource")).toBeUndefined();
    expect(result.identifiers.get("setInterval")).toBeUndefined();
    expect(result.identifiers.get("WebSocket")).toBeUndefined();
    expect(result.identifiers.get("XMLHttpRequest")).toBe(1);
    expect(result.strings).toContain("setInterval");
    expect(result.identifiers.get("count")).toBe(1);
  });

  it("tells a field named after a global from the global itself", () => {
    const fixture = [
      "const clause = ledger.fetch;",
      "const outcome = ledger.fetch.outcome;",
      "const optional = ledger?.fetch;",
      "type Ledger = { tip: string; fetch: Clause };",
      "const payload = { problems: [], fetch: { outcome: 'never' } };",
      "const spread = { ...fetch };",
      "const chosen = ready ? fetch : none;",
      "const called = window.fetch(url);",
    ].join("\n");

    const result = scan(fixture);

    // Three reads through a dot and two member names are fields. The spread,
    // the middle of the conditional, and the call through window name the
    // global itself.
    expect(result.fields.get("fetch")).toBe(5);
    expect(result.identifiers.get("fetch")).toBe(3);
  });
});

describe("the first cut", () => {
  it("reaches the network from exactly the listed places, each with its count, each naming its resources", () => {
    const sites = new Map<string, number>();
    for (const file of files) {
      const total = NETWORK.reduce((count, name) => count + (scanned.get(file)?.identifiers.get(name) ?? 0), 0);
      if (total > 0) {
        sites.set(file, total);
      }
    }

    // The stream site reaches the network once and by another name, so it is
    // counted here and its own rules are asserted below.
    const expected = new Map(CALL_SITES.map(([file, calls]): [string, number] => [file, calls]));
    expected.set(STREAM_SITE, (expected.get(STREAM_SITE) ?? 0) + 1);
    expect(Object.fromEntries(sites)).toEqual(Object.fromEntries(expected));
    for (const [file, calls, resources] of CALL_SITES) {
      expect({ file, fetches: scanned.get(file)?.identifiers.get("fetch") }).toEqual({ file, fetches: calls });
      for (const resource of resources) {
        expect({ file, resource, named: scanned.get(file)?.strings.includes(resource) }).toEqual({
          file,
          resource,
          named: true,
        });
      }
    }
  });

  it("builds no markup from a string, under any name", () => {
    expect(filesNaming(INJECTION)).toEqual([]);
    expect(stringsMatching((value) => INJECTION.includes(value.trim()))).toEqual([]);
  });

  it("writes to no document, and the one file that says write says nothing else", () => {
    expect(filesNaming([WRITE])).toEqual([WRITES_A_PREFERENCE]);
    expect(scanned.get(WRITES_A_PREFERENCE)?.identifiers.get("document")).toBeUndefined();
    expect(stringsMatching((value) => value.trim() === WRITE)).toEqual([]);
  });

  it("opens one stream, in one file, for one resource, and no other", () => {
    expect(filesNaming(["EventSource"])).toEqual([STREAM_SITE]);
    expect(scanned.get(STREAM_SITE)?.identifiers.get("EventSource")).toBe(1);
    expect(scanned.get(STREAM_SITE)?.strings).toContain(STREAM_RESOURCE);
    // The one file that may open a stream may not also fetch: a file with
    // both would be a second reader hiding behind the exception.
    expect(scanned.get(STREAM_SITE)?.identifiers.get("fetch")).toBeUndefined();
  });

  it("opens no socket, request object, or beacon, under any name", () => {
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
