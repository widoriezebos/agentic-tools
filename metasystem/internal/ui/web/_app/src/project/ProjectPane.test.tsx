import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * Where the goal page puts a save nobody could confirm.
 *
 * The claim is about wiring and cannot be read from a render: the sheet's
 * unresolved outcome is an asynchronous answer from the network, and these
 * tests render to static markup and have no document to press a button in. So
 * the source is the subject, and what is asserted is the one thing that
 * decides whether the human keeps their words — which of this pane's two
 * writers the sheet's reread is handed to.
 */

const SOURCE = readFileSync(path.resolve(fileURLToPath(import.meta.url), "..", "ProjectPane.tsx"), "utf8");

/**
 * The text of one function of this file, from its own `function` line to the
 * next one at the top of the file, so that a claim about what is inside
 * `GoalBlock` cannot be satisfied by something written outside it.
 */
function bodyOf(name: string): string {
  const from = SOURCE.indexOf(`function ${name}(`);
  expect(from, `${name} is a function of ProjectPane.tsx`).toBeGreaterThan(-1);
  const next = SOURCE.indexOf("\nfunction ", from + 1);
  return SOURCE.slice(from, next === -1 ? SOURCE.length : next);
}

describe("the goal page's two writers", () => {
  /**
   * `onEdited` reads the page again from the top, and reading again means a
   * loading state: it is the right thing after a save the ledger confirmed and
   * the wrong thing after one it did not, because the loading state unmounts
   * the block, the sheet inside it, the human's draft and the words saying the
   * save was not confirmed — which for a save that landed without a proof say
   * not to run it again.
   */
  it("reloads through a loading state, which is why the reread cannot go there", () => {
    const briefed = bodyOf("Briefed");

    expect(briefed).toContain("const reload = () => {");
    expect(briefed).toContain('setRead({ state: "loading" });');
  });

  /**
   * So the sheet's reread is handed the pane's other writer: the one that sets
   * the board it already read, in place. Nothing unmounts, nothing loads, and
   * the ledger is read once — by the sheet — rather than twice.
   */
  it("hands the sheet's reread the board setter, not the page reload", () => {
    expect(SOURCE).toContain("<GoalBlock briefing={briefing} ledger={ledger} onEdited={onReload} onReread={onLedger} />");

    const block = bodyOf("GoalBlock");

    expect(block).toContain("onReread: (after: Backlog) => void;");
    expect(block).toContain("onReread={onReread}");
    // The reload belongs to the one outcome that earns it. A second call to it
    // in this block is the reread taking the sheet down with the page.
    expect(block.match(/onEdited\(\)/g)).toHaveLength(1);
    expect(block).toContain("onDone={() => {\n            setEditing(false);\n            onEdited();\n          }}");
    // And the row the sheet is given is found in whatever board this block
    // holds, so once the reread has set it the sheet shows the row as the
    // ledger now has it.
    expect(block).toContain("const mine = ledger?.rows.find((row) => row.ref.id === goal.id);");
  });
});
