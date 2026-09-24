import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { firstWords, GoalPicker, matching, pickRefusal, type PickableGoal } from "./GoalPicker";

/**
 * The field that must hold existing goals.
 *
 * Three of its claims are rules and are asserted as rules: which goals a typed
 * line finds, which goals are never among them, and what the field refuses.
 * The fourth is what the chosen goals look like, which can only be read from
 * the markup, so it is read from the markup.
 */

const GOALS: PickableGoal[] = [
  { id: "g1-s13", intent: "The goal page reads the whole record", lane: "To Do", concluded: "" },
  { id: "g1-s14", intent: "The Fleet section reads the seats", lane: "Ready for Work", concluded: "" },
  { id: "refund-queue", intent: "Refunds are issued within a day", lane: "In Progress", concluded: "" },
  { id: "g1-s9", intent: "The application shell, the rail and the header", lane: "Done", concluded: "done" },
  { id: "g1-s7", intent: "A second bundler beside the first", lane: "Abandoned", concluded: "abandoned" },
];

function ids(found: readonly PickableGoal[]): string[] {
  return found.map((goal) => goal.id);
}

describe("what a typed line finds", () => {
  it("offers every goal while nothing is typed, closed ones included", () => {
    expect(ids(matching(GOALS, ""))).toEqual(["g1-s13", "g1-s14", "refund-queue", "g1-s9", "g1-s7"]);
  });

  it("matches on the id", () => {
    expect(ids(matching(GOALS, "g1-s1"))).toEqual(["g1-s13", "g1-s14"]);
  });

  it("matches on the intent's words, in any order and in any case", () => {
    expect(ids(matching(GOALS, "SEATS"))).toEqual(["g1-s14"]);
    expect(ids(matching(GOALS, "reads seats"))).toEqual(["g1-s14"]);
    expect(ids(matching(GOALS, "seats reads"))).toEqual(["g1-s14"]);
  });

  // The id and the intent are one haystack: a human who remembers half of
  // each should not have to remember which half was which.
  it("matches words across the id and the intent together", () => {
    expect(ids(matching(GOALS, "refund day"))).toEqual(["refund-queue"]);
  });

  it("needs every word, not any of them", () => {
    expect(ids(matching(GOALS, "reads nothing"))).toEqual([]);
  });

  it("never offers the goal being opened", () => {
    expect(ids(matching(GOALS, "", "g1-s13"))).toEqual(["g1-s14", "refund-queue", "g1-s9", "g1-s7"]);
    expect(ids(matching(GOALS, "g1-s13", "g1-s13"))).toEqual([]);
  });

  // A goal already chosen is a chip on screen: offering it again would be
  // offering a choice that says nothing.
  it("never offers a goal that has already been chosen", () => {
    expect(ids(matching(GOALS, "", ["g1-s13", "refund-queue"]))).toEqual(["g1-s14", "g1-s9", "g1-s7"]);
  });
});

describe("what the field refuses", () => {
  it("refuses nothing while it is empty, because no dependency is the common case", () => {
    expect(pickRefusal([], "", GOALS)).toBe("");
  });

  it("refuses nothing for live goals that were chosen", () => {
    expect(pickRefusal(["g1-s13", "g1-s14"], "", GOALS)).toBe("");
  });

  it("says a line that names no goal names no goal", () => {
    expect(pickRefusal([], "refnud", GOALS)).toBe("no goal named refnud");
  });

  // A line that finds goals and chose none is not an answer either: sent as
  // it stands it would be no dependency at all, which is a typed intent
  // dropped.
  it("asks for a choice when the line finds goals and none was chosen", () => {
    expect(pickRefusal([], "reads", GOALS)).toBe("Choose one of the goals listed, or clear the field.");
  });

  // With several chips on screen the refusal has to say which one it is
  // about, so it names the goal.
  it("names the goal that has already ended, and what it ended as", () => {
    expect(pickRefusal(["g1-s9"], "", GOALS)).toBe("g1-s9 is already done, nothing to unblock");
    expect(pickRefusal(["g1-s13", "g1-s7"], "", GOALS)).toBe("g1-s7 is abandoned, nothing to unblock");
  });

  // The goal being opened is not in the list, so its own id is a line that
  // names no goal rather than a goal that could be chosen.
  it("counts the excluded goal as no goal at all", () => {
    expect(pickRefusal([], "g1-s13", GOALS, "g1-s13")).toBe("no goal named g1-s13");
  });
});

describe("the intent's first words", () => {
  it("are the whole line when the line is short", () => {
    expect(firstWords("Refunds are issued within a day")).toBe("Refunds are issued within a day");
  });

  it("are cut at a word, never inside one", () => {
    const long = "Refunds are issued within a day, with nobody touching the queue and nothing left behind";
    const cut = firstWords(long);
    expect(cut.length).toBeLessThanOrEqual(81);
    expect(cut.endsWith("…")).toBe(true);
    expect(long.startsWith(cut.slice(0, -1))).toBe(true);
    expect(cut.slice(0, -1).endsWith(" ")).toBe(false);
  });

  it("puts a wrapped intent on one line", () => {
    expect(firstWords("  The board\n  reads.  ")).toBe("The board reads.");
  });
});

describe("the field itself", () => {
  it("is a combobox with a list it can name, and no list until it is asked", () => {
    const markup = renderToStaticMarkup(
      <GoalPicker id="ms-open-blocks" goals={GOALS} chosen={[]} onChoose={() => undefined} />,
    );
    expect(markup).toContain('role="combobox"');
    expect(markup).toContain('aria-expanded="false"');
    expect(markup).toContain('aria-controls="ms-open-blocks-list"');
    expect(markup).toContain('aria-autocomplete="list"');
    expect(markup).not.toContain('role="listbox"');
  });

  // What was chosen is a goal, and a goal is an id and what it is for. An id
  // alone in a box is what this field exists to stop.
  it("shows each choice as a chip: the id, the intent's first words, and a way out", () => {
    const markup = renderToStaticMarkup(
      <GoalPicker
        id="ms-open-blocks"
        goals={GOALS}
        chosen={["refund-queue", "g1-s14"]}
        onChoose={() => undefined}
      />,
    );
    expect(markup).toContain("ms-pick-chips");
    expect(markup).toContain('class="ms-mono">refund-queue<');
    expect(markup).toContain("Refunds are issued within a day");
    expect(markup).toContain('aria-label="Clear refund-queue"');
    expect(markup).toContain('class="ms-mono">g1-s14<');
    expect(markup).toContain('aria-label="Clear g1-s14"');
  });

  // The field stays under the chips, because the usual act after naming a
  // goal is naming another one.
  it("keeps the field open under the chips, so a second goal can be named", () => {
    const markup = renderToStaticMarkup(
      <GoalPicker id="ms-open-blocks" goals={GOALS} chosen={["refund-queue"]} onChoose={() => undefined} />,
    );
    expect(markup).toContain('role="combobox"');
  });
});
