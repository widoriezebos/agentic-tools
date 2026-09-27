import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Proposal, ProposalState } from "./api";
import { chipName, chipWords, proposedFor, type Proposed } from "./proposed";
import { ProposedChipView } from "./ProposedChip";
import { cardsIn } from "./proposing";

/**
 * What the chip says, and what it puts on the row.
 *
 * Two claims live here. The words are the act's own word where there is one line
 * and a count where there are several, and a line that was refused or whose
 * outcome nobody knows says so in the danger colour — those are functions, and
 * they are exercised as functions. And the chip is a button with a name that
 * carries the goal, because a chip heard out of the row it is in names nothing;
 * that one is read from the markup, which is where the claim is.
 */

function proposal(over: Partial<Proposal> = {}): Proposal {
  return {
    index: 0,
    verb: "park-goal",
    goal: "g1-s44",
    title: "The seat census answers which machines are alive",
    fields: { because: "superseded by the seat inventory (g1-s42)" },
    read: null,
    why: "the inventory covers what these were for",
    offered: true,
    reason: "",
    state: "waiting",
    words: "",
    at: "2026-09-26T09:00:00Z",
    version: 1,
    ...over,
  };
}

/** The lines of one answer, as the hook would answer them for one goal. */
function lines(...entries: readonly Partial<Proposal>[]): readonly Proposed[] {
  const proposals = entries.map((over, index) => proposal({ index, ...over }));
  return proposedFor(cardsIn([{ turn: "t1", proposals }], {}, {}, []), "g1-s44");
}

function rendered(found: readonly Proposed[]): string {
  return renderToStaticMarkup(<ProposedChipView goal="g1-s44" lines={found} onPress={() => undefined} />);
}

describe("what the chip says", () => {
  it("is the act's own button word for one line", () => {
    expect(chipWords(lines({ verb: "park-goal" }))).toEqual({ words: "Pause proposed", danger: false });
    expect(chipWords(lines({ verb: "edit-goal" }))).toEqual({ words: "Edit proposed", danger: false });
    expect(chipWords(lines({ verb: "approve-goal" }))).toEqual({ words: "Approve proposed", danger: false });
  });

  it("is a count for more than one, so the row stays a row", () => {
    expect(chipWords(lines({ verb: "park-goal" }, { verb: "edit-goal" }))).toEqual({
      words: "2 proposed",
      danger: false,
    });
  });

  /**
   * A line a page went away in the middle of is not an alarm and not a record:
   * it is a line the human has still to see through, which the inbox keeps for
   * the same reason. So the chip says what was proposed.
   */
  it("reads a line left in flight as proposed", () => {
    expect(chipWords(lines({ state: "applying" }))).toEqual({ words: "Pause proposed", danger: false });
  });

  it("names a refusal in the danger colour", () => {
    expect(chipWords(lines({ verb: "park-goal", state: "refused", words: "g1-s44 is claimed by m2a" }))).toEqual({
      words: "Pause refused",
      danger: true,
    });
  });

  /**
   * An unresolved line is not "the act was refused" and not "the act landed": it
   * is an act whose outcome nobody knows, and the word for that is the state's
   * own rather than the verb's.
   */
  it("says unresolved for an answer that did not say what happened", () => {
    expect(chipWords(lines({ state: "unresolved", words: "pushed; whether it landed is unresolved" }))).toEqual({
      words: "unresolved",
      danger: true,
    });
  });

  it("carries the danger colour under a count where any of the lines is one of those", () => {
    expect(chipWords(lines({ state: "waiting" }, { state: "refused", words: "no" }))).toEqual({
      words: "2 proposed",
      danger: true,
    });
  });

  it("says nothing at all where no line waits", () => {
    expect(chipWords([])).toBeNull();
    expect(chipWords(lines({ state: "applied" }, { state: "dismissed" }))).toBeNull();
  });

  it("names the goal and the count in the accessible name", () => {
    expect(chipName("g1-s44", lines({ verb: "park-goal" }))).toBe("Pause proposed on g1-s44, 1 action");
    expect(chipName("g1-s44", lines({}, {}))).toBe("2 proposed on g1-s44, 2 actions");
    expect(chipName("g1-s44", [])).toBe("");
  });
});

describe("what the chip puts on the row", () => {
  it("is a button in the marker variant, named for the goal", () => {
    const markup = rendered(lines({ verb: "park-goal" }));

    expect(markup).toContain("<button");
    expect(markup).toContain('type="button"');
    expect(markup).toContain('class="ms-chip ms-chip-proposed"');
    expect(markup).toContain('aria-label="Pause proposed on g1-s44, 1 action"');
    expect(markup).toContain("Pause proposed");
  });

  it("wears the danger class on a refused line", () => {
    const markup = rendered(lines({ state: "refused", words: "g1-s44 is claimed by m2a" }));

    expect(markup).toContain("ms-chip-proposed--wrong");
    expect(markup).toContain("Pause refused");
  });

  it("is absent where the hook answers none", () => {
    expect(rendered([])).toBe("");
    for (const state of ["applied", "dismissed"] as ProposalState[]) {
      expect({ state, markup: rendered(lines({ state })) }).toEqual({ state, markup: "" });
    }
  });
});
