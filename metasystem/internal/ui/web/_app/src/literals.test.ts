import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import {
  ACCEPT_ON_THE_GOAL,
  ACCEPTED_LANDING,
  changedSince,
  choiceSaid,
  correctionsImpact,
  risksImpact,
  DECISIONS,
  END_WITHOUT_SAID,
  findingsSummary,
  LEAVE_SAID,
  nodPlan,
  nothingRaised,
  pillOf,
  recordedLine,
  TRY_IT,
  VERDICT_MOVED,
  VERDICT_ON_OLDER,
  LANDED_UNFINISHED,
  UNDO,
  wayConsequence,
  type RoomFinding,
} from "./review/room";
import { FIX } from "./partner/sitting";

/**
 * One place writes a colour, and it is src/tokens.css.
 *
 * A colour written anywhere else is a colour that exists in one theme only,
 * that no contrast test ever sees, and that nobody can find again. The scan
 * covers everything the bundle includes; a test file ships nothing to a
 * browser, so the two guards below scan it only for a stylesheet.
 */

const SRC = path.resolve(fileURLToPath(import.meta.url), "..");
const TABLE = "tokens.css";

/** A hexadecimal colour. Written so that this pattern does not match itself. */
const HEX = /#([0-9a-fA-F]{3,8})\b/;
const FUNCTIONAL = /\b(rgb|rgba|hsl|hsla|oklch|lab|color)\(/;

function files(relative = ""): string[] {
  const found: string[] = [];
  for (const entry of readdirSync(relative === "" ? SRC : path.join(SRC, relative), { withFileTypes: true })) {
    const next = relative === "" ? entry.name : `${relative}/${entry.name}`;
    if (entry.isDirectory()) {
      found.push(...files(next));
      continue;
    }
    found.push(next);
  }
  return found.sort();
}

function offenders(candidates: string[]): string[] {
  const found: string[] = [];
  for (const file of candidates) {
    const contents = readFileSync(path.join(SRC, file), "utf8");
    for (const [number, line] of contents.split("\n").entries()) {
      if (HEX.test(line) || FUNCTIONAL.test(line)) {
        found.push(`${file}:${number + 1}: ${line.trim()}`);
      }
    }
  }
  return found;
}

describe("colour literals", () => {
  it("appear in no shipped source but the token table", () => {
    const shipped = files().filter(
      (file) => /\.(ts|tsx|css)$/.test(file) && !file.endsWith(".test.ts") && file !== TABLE,
    );
    expect(shipped.length).toBeGreaterThan(10);
    expect(offenders(shipped)).toEqual([]);
  });

  it("appear in no stylesheet but the token table, test or not", () => {
    const stylesheets = files().filter((file) => file.endsWith(".css") && file !== TABLE);
    expect(stylesheets).toContain("shell/shell.css");
    expect(offenders(stylesheets)).toEqual([]);
  });

  it("are what the table itself is made of", () => {
    expect(offenders([TABLE]).length).toBeGreaterThan(30);
  });
});

/**
 * The review room's words (review-findings-read-as-decisions §5): what the room
 * says on screen is in a person's terms, so none of the room's own sentences
 * says tip, candidate, anchor, desk, Dismiss or unanswered. Ids and paths live
 * in a finding's evidence fold, which is the reviewer's own words.
 */
describe("the review room's words", () => {
  const BANNED = /\b(tip|candidate|anchor|desk|dismiss|unanswered)\b/iu;
  const finding = (over: Partial<RoomFinding>): RoomFinding => ({
    id: "deposit:t1#0", title: "A press that dies halfway leaves the ledger locked.", why: "The next press waits forever.",
    severity: "blocks", recommend: "must-fix", reason: "Release the lock.", evidence: "", anchor: "", consequence: "",
    answer: "", recorded: false, ...over,
  });
  const findings = [
    finding({}), finding({ id: "b", severity: "fix", recommend: "fix-later" }), finding({ id: "c", severity: "note", recommend: "not-a-problem" }),
    finding({ id: "d", answer: FIX, recorded: true, severity: "fix" }),
  ];
  const running = { goal: "g", state: "running", readiness: "answering", address: "127.0.0.1:7981", commit: "a".repeat(40), said: "" };

  it("say none of the words a person was never taught", () => {
    const said = [
      ...DECISIONS.flatMap((one) => [one.label, one.consequence]),
      ACCEPT_ON_THE_GOAL, END_WITHOUT_SAID, LEAVE_SAID, TRY_IT, VERDICT_MOVED, ACCEPTED_LANDING,
      VERDICT_ON_OLDER, LANDED_UNFINISHED, UNDO, wayConsequence("land", findings, "reviewing"), wayConsequence("land", findings, "failed"),
      ...(["complete", "reviewing", "stopped", "failed"] as const).flatMap((one) => Object.values(nothingRaised(one))),
      ...(["complete", "reviewing"] as const).flatMap((one) => [
        wayConsequence("land", findings, one), wayConsequence("send back", findings, one),
        wayConsequence("land", [], one), wayConsequence("send back", [], one),
      ]),
      correctionsImpact(nodPlan(findings).corrections), risksImpact(findings), risksImpact(findings.slice(0, 1)),
      wayConsequence("land", [findings[0]], "complete"),
      ...Object.values(findingsSummary(findings)),
      ...nodPlan(findings).following.map((one) => one.said),
      ...findings.flatMap((one) => (["fix", "follow-up", "not a problem", "accepted"] as const).map((decision) => choiceSaid(decision, one))),
      pillOf(null, "a".repeat(40)).words, pillOf(running, "a".repeat(40)).words, pillOf({ ...running, commit: "b".repeat(40) }, "a".repeat(40)).words,
      pillOf({ ...running, state: "starting" }, "a".repeat(40)).words, pillOf({ ...running, readiness: "silent" }, "a".repeat(40)).words,
      pillOf({ refusal: "no", code: "no-contract" }, "a".repeat(40)).words,
      changedSince("- Reviewed: x\n", "- Reviewed: y\n"),
      recordedLine({ verdict: "clear-to-land", tip: "a".repeat(40), by: "Wido" }, 0, "g", ["f"]),
      recordedLine({ verdict: "send-back", tip: "a".repeat(40), by: "Wido" }, 1, "g"),
    ];
    expect(said.filter((one) => BANNED.test(one))).toEqual([]);
  });
});
