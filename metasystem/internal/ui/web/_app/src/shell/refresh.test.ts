import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { showsInHeader } from "./refresh";

/**
 * The offered re-read's one contract: it keeps the page mounted.
 *
 * This is not a nicety. An offered read is made on somebody else's behalf — the
 * Partner's card asks for it after a confirmed act — over whatever page the human
 * happens to be looking at, with whatever they have half typed into it. Fleet's
 * failed-launch Retry form is inline in the page and in no sheet; the goal page's
 * edit sheet renders over the very columns a read would replace. A read that
 * passed through a loading state would unmount both and throw the keystrokes away
 * (Astra S58-03 and S58-10).
 *
 * So the rule is asserted over the source tree rather than over one page: every
 * function a pane offers as its re-read is read here, and a body that puts the
 * page into a loading state fails, with the file and the function named. A pane
 * added later cannot quietly offer a blanking read.
 *
 * It reads code the plain way — the declaration and its braces — because that is
 * enough for the shape these panes are written in, and the scan asserts its own
 * reach before it asserts anything else: the six call sites it expects to find
 * are named below, so a pattern that stopped matching fails rather than passing
 * silently.
 */

const SRC = path.resolve(fileURLToPath(import.meta.url), "..", "..");

/**
 * The panes that offer a read, the function each one offers, and the file that
 * function is written in.
 *
 * The third column is there because one of them is not written in its own pane:
 * the board's reads live in the hook that owns its state, and the pane
 * destructures them. Naming the file is what lets this guard read the body
 * without having to resolve an import.
 */
const OFFERED: readonly (readonly [string, string, string])[] = [
  ["overview/OverviewPane.tsx", "again", "overview/OverviewPane.tsx"],
  ["application/ApplicationPane.tsx", "again", "application/ApplicationPane.tsx"],
  ["decisions/DecisionsPane.tsx", "again", "decisions/DecisionsPane.tsx"],
  // Fleet's is the in-place read it already had, which is the whole of the fold:
  // its own `reload` blanks the page and takes the inline Retry form with it.
  ["fleet/FleetPane.tsx", "again", "fleet/FleetPane.tsx"],
  ["backlog/BacklogPane.tsx", "again", "backlog/state.ts"],
  ["project/ProjectPane.tsx", "again", "project/ProjectPane.tsx"],
];

/** What a body that unmounts its page says. */
const BLANKS = 'state: "loading"';

/** What a failure that asks whether there is a reading to keep says. */
const CONSULTS = /\.state === "(read|known)"/;

/** And what keeping that reading, with the refusal's words beside it, says. */
const KEEPS = /\{ \.\.\.\w+, problem: /;

/** And what the same failure says for a first read, which has nothing to keep. */
const FAILS = 'state: "failed"';

function sourceFiles(relative = ""): string[] {
  const found: string[] = [];
  for (const entry of readdirSync(relative === "" ? SRC : path.join(SRC, relative), { withFileTypes: true })) {
    const next = relative === "" ? entry.name : `${relative}/${entry.name}`;
    if (entry.isDirectory()) {
      found.push(...sourceFiles(next));
      continue;
    }
    if (/\.tsx?$/.test(entry.name) && !/\.test\.tsx?$/.test(entry.name)) {
      found.push(next);
    }
  }
  return found.sort();
}

/** Every `useOffersRefresh(name, …)` under src/, with the file it is in. */
function offers(): { file: string; offered: string }[] {
  const rows: { file: string; offered: string }[] = [];
  for (const file of sourceFiles()) {
    const source = readFileSync(path.join(SRC, file), "utf8");
    for (const match of source.matchAll(/useOffersRefresh\(\s*([A-Za-z_$][\w$]*)\s*,/g)) {
      rows.push({ file, offered: match[1] });
    }
  }
  return rows;
}

/**
 * The body of `const <name> = …` in this file, from its first `{` to the brace
 * that closes it. It is the declaration's own text and nothing after it, so a
 * later function that does blank the page is not read as this one's.
 */
function bodyOf(source: string, name: string): string {
  const at = source.indexOf(`const ${name} = `);
  if (at < 0) {
    return "";
  }
  const opens = source.indexOf("{", at);
  return opens < 0 ? "" : bracedAt(source, opens);
}

/** The block that opens at `opens`, up to the brace that closes it. */
function bracedAt(source: string, opens: number): string {
  let depth = 0;
  for (let index = opens; index < source.length; index += 1) {
    if (source[index] === "{") {
      depth += 1;
    } else if (source[index] === "}") {
      depth -= 1;
      if (depth === 0) {
        return source.slice(opens, index + 1);
      }
    }
  }
  return source.slice(opens);
}

/**
 * The setter of the pane's own state: the one declared holding the loading
 * state a page starts in.
 *
 * Naming it is what tells the read's own failure path from the others. A page
 * full of acts has a `.catch` for each of them — a sheet that could not be
 * opened, a status that was refused — and every one of them sets some state to
 * failed; only one of them sets the state of the page.
 */
function paneSetter(source: string): string {
  const declared = /const \[\w+, (set\w+)\] = useState<\w+>\(\{ state: "loading" \}\)/.exec(source);
  return declared === null ? "" : declared[1];
}

/** The body of the `.catch` that sets that state: the read's failure path. */
function failurePath(source: string, setter: string): string {
  for (const match of source.matchAll(/\.catch\(\s*\([^)]*\)\s*=>\s*\{/g)) {
    const body = bracedAt(source, match.index + match[0].length - 1);
    if (body.includes(`${setter}(`)) {
      return body;
    }
  }
  return "";
}

describe("what a pane offers as its re-read", () => {
  it("is found in every pane that offers one, and nowhere else", () => {
    const found = offers();
    expect(found.length).toBe(OFFERED.length);
    for (const [file, offered] of OFFERED) {
      expect(found).toContainEqual({ file, offered });
    }
  });

  it("keeps the page mounted, in every pane that offers one", () => {
    for (const [file, offered, declared] of OFFERED) {
      const source = readFileSync(path.join(SRC, declared), "utf8");
      const body = bodyOf(source, offered);
      expect({ file, offered, found: body !== "" }).toEqual({ file, offered, found: true });
      expect({ file, offered, blanks: body.includes(BLANKS) }).toEqual({ file, offered, blanks: false });
    }
  });

  /**
   * And the blanking read is still there, for the press that has nothing on
   * screen to keep: Retry after a failure, and the strip's own Refresh.
   *
   * It is asserted so that "keeps the page mounted" cannot be satisfied by a
   * pane losing its loading state altogether — which would be a first read with
   * nothing to look at.
   */
  it("is not the pane's own Retry, which does blank it", () => {
    for (const [file, blanking] of [
      ["overview/OverviewPane.tsx", "reload"],
      ["application/ApplicationPane.tsx", "reload"],
      ["decisions/DecisionsPane.tsx", "reload"],
      ["fleet/FleetPane.tsx", "reload"],
      ["project/ProjectPane.tsx", "reload"],
      // The board's blanking read is its toolbar's own Refresh, which is also
      // the one that asks the server to fetch the canonical branch first.
      ["backlog/state.ts", "refresh"],
    ]) {
      const source = readFileSync(path.join(SRC, file), "utf8");
      expect({ file, blanks: bodyOf(source, blanking).includes(BLANKS) }).toEqual({ file, blanks: true });
    }
  });

  /**
   * And a re-read that FAILS keeps the page too.
   *
   * Keeping the page mounted while the read is in flight was only half of the
   * promise. The other half was missing: a refusal set the whole pane to failed,
   * the error view replaced the page, and everything inside it went with it —
   * Fleet's failed-launch Retry form, and the authorization word and the review
   * date a human had typed into it. Apply an unrelated proposal in the Partner's
   * drawer, let the Fleet re-read it asks for fail, and the draft was gone; a
   * read that then worked drew a new, empty form (Astra C-05). The payload was
   * good the whole time. Only the request for a newer one was not.
   *
   * So the failure path consults what the pane already holds, keeps that reading
   * and records the refusal's words beside it; and only a first read's failure —
   * where there is nothing on screen to keep — reaches the failed state. All
   * three are asserted: consulting the state alone would be satisfied by a body
   * that looks and blanks the page anyway, and keeping the reading alone would be
   * satisfied by a pane that stopped failing at all.
   */
  it("keeps what it already read when the read fails, in every pane that offers one", () => {
    for (const [file, , declared] of OFFERED) {
      const source = readFileSync(path.join(SRC, declared), "utf8");
      const setter = paneSetter(source);
      expect({ file, declaresItsPaneState: setter !== "" }).toEqual({ file, declaresItsPaneState: true });
      const failure = failurePath(source, setter);
      expect({ file, hasAFailurePath: failure !== "" }).toEqual({ file, hasAFailurePath: true });
      expect({ file, consultsWhatItHolds: CONSULTS.test(failure) }).toEqual({ file, consultsWhatItHolds: true });
      expect({ file, keepsThatReading: KEEPS.test(failure) }).toEqual({ file, keepsThatReading: true });
      expect({ file, stillFailsAFirstRead: failure.includes(FAILS) }).toEqual({ file, stillFailsAFirstRead: true });
    }
  });

  /**
   * The goal page's offered read takes BOTH of its payloads.
   *
   * The title, the state chip and the intent come from the project payload; the
   * relation between goals comes from the ledger's. A re-read of one alone left a
   * confirmed edit of the goal in view looking unchanged (Astra S58-11). Both
   * reads depend on the one attempt the offered read moves, which is what makes
   * one press two reads.
   */
  it("takes both of the goal page's payloads", () => {
    const source = readFileSync(path.join(SRC, "project/ProjectPane.tsx"), "utf8");
    expect(bodyOf(source, "again")).toContain("setAttempt");
    // The project's read, and the ledger's, each keyed on that attempt.
    expect(source).toContain("}, [attempt]);");
    expect(source).toContain("}, [goal, attempt]);");
    expect(source).toContain("loadPane(aborter.signal)");
    expect(source).toContain("loadBacklog(aborter.signal)");
  });
});

describe("where the header shows an icon for an offer", () => {
  it("shows one for a page that has no strip of its own", () => {
    expect(showsInHeader({ reread: () => undefined, hint: "Refresh" })).toBe(true);
    expect(showsInHeader({ reread: () => undefined, hint: "Refresh", inStrip: false })).toBe(true);
  });

  it("shows none for a page whose strip already has one", () => {
    expect(showsInHeader({ reread: () => undefined, hint: "Refresh", inStrip: true })).toBe(false);
  });

  it("shows none where nothing is offered", () => {
    expect(showsInHeader(null)).toBe(false);
  });

  /**
   * The two pages with a strip of their own offer their read and say so, which is
   * the whole point of the flag: without the offer a confirmed act would leave
   * the board and the goal page stale, and without the flag the header would
   * carry a second icon beside the one the strip has.
   */
  it("is what the board and the goal page ask for", () => {
    for (const file of ["backlog/BacklogPane.tsx", "project/ProjectPane.tsx"]) {
      const source = readFileSync(path.join(SRC, file), "utf8");
      expect({ file, marked: /useOffersRefresh\([^;]*,\s*true\)/.test(source) }).toEqual({ file, marked: true });
    }
  });
});
