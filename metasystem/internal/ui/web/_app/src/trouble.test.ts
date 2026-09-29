import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * The trouble rule (g1-s68 D1).
 *
 * Everything that goes wrong on screen — a refused act, a pane that could not
 * be read, a turn that did not complete, a pane that threw, a failure notified
 * — renders through one component, `Trouble`, and so carries the one control
 * that asks the Partner what happened. The class names each site used to draw
 * its own line with are refused anywhere else under src/, in code and in the
 * stylesheets alike, so the next site cannot draw one of its own and forget
 * the control.
 *
 * The rule cannot prove a site nobody has written yet uses `Trouble` (S68-09);
 * what it proves is that the old ways are gone, and that every site this slice
 * converted still imports the component.
 */

const SRC = path.resolve(fileURLToPath(import.meta.url), "..");
const RULE = "trouble.test.ts";
const COMPONENT = "shell/Trouble.tsx";

/**
 * The shapes a trouble line used to be drawn in, one per site: a refusal, a
 * refuse and a refused line (Sol SOL-S68-01) as well as a problem, and a card's
 * refused state as much as a line of its own. A problem is refused in any
 * shape, bare or prefixed, singular or plural, alone or leading a longer name:
 * the ledger's list of problems was drawn as plain `ms-problems` (Sol's re-read).
 */
const REFUSED =
  /ms-[a-z0-9-]*-(?:refusal|refused|refuse)s?(?![a-z0-9-])|ms-(?:[a-z0-9-]*-)?problems?(?![a-z0-9])|ms-partner-failed|ms-error-[a-z]/gu;

/** The sites this slice converted, by file, each of which renders `Trouble`. */
const CONVERTED = [
  "backlog/BacklogPane.tsx",
  "backlog/Board.tsx",
  "backlog/EditSheet.tsx",
  "backlog/OpenSheet.tsx",
  "backlog/Panel.tsx",
  "decisions/DecisionsPane.tsx",
  "decisions/InboxRow.tsx",
  "fleet/FleetPane.tsx",
  "fleet/LaunchCard.tsx",
  "fleet/LaunchSheet.tsx",
  "notifications/Panel.tsx",
  "partner/Deposit.tsx",
  "partner/FontControl.tsx",
  "partner/Proposal.tsx",
  "partner/Seeing.tsx",
  "partner/StartSitting.tsx",
  "partner/Suggestion.tsx",
  "partner/Transcript.tsx",
  "project/DocumentPane.tsx",
  "project/ProjectPane.tsx",
  "project/Sheet.tsx",
  "review/Answers.tsx",
  "review/Desk.tsx",
  "review/Door.tsx",
  "review/Pill.tsx",
  "review/ReviewRoom.tsx",
  "review/Verdict.tsx",
  "shell/ErrorBoundary.tsx",
  "shell/SignInControl.tsx",
  "shell/SignInSheet.tsx",
  "stickies/Panel.tsx",
];

function walk(directory: string): string[] {
  const found: string[] = [];
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const full = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      found.push(...walk(full));
    } else if (/\.(?:tsx?|css)$/u.test(entry.name)) {
      found.push(path.relative(SRC, full).split(path.sep).join("/"));
    }
  }
  return found;
}

/** The sources the rule reads: every file under src/ but tests, the rule and the component. */
function sources(): string[] {
  return walk(SRC).filter(
    (file) => !/\.test\.tsx?$/u.test(file) && file !== RULE && file !== COMPONENT,
  );
}

describe("the trouble rule", () => {
  it("reads every source under src/", () => {
    const read = sources();
    expect(read.length).toBeGreaterThan(100);
    expect(read).toContain("backlog/Board.tsx");
    expect(read).toContain("shell/shell.css");
  });

  it("refuses a refusal, problem, failed-turn or error line drawn outside Trouble", () => {
    const offenders: string[] = [];
    for (const file of sources()) {
      const text = readFileSync(path.join(SRC, file), "utf8");
      for (const match of text.matchAll(REFUSED)) {
        offenders.push(`${file}: ${match[0]}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("finds every converted site rendering Trouble", () => {
    const missing = CONVERTED.filter((file) => {
      const text = readFileSync(path.join(SRC, file), "utf8");
      return !/<Trouble[\s>]/u.test(text);
    });
    expect(missing).toEqual([]);
  });
});
