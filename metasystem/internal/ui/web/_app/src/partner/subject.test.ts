import { describe, expect, it } from "vitest";

import { chipLabel, effectiveSubject, kindWord, registerFor, type Chosen } from "./subject";
import { suggestionsFor, registers, DRAFT_REGISTER, HOW_TO_ASK } from "./suggestions";

/**
 * What "this" means, and for how long.
 *
 * Contract 3 is the rule under test: the subject follows the page until you
 * choose one; a chosen subject stays until it is cleared or replaced;
 * navigating changes neither. So the lifecycle here is a sequence of values
 * rather than a component, which is what makes it testable at all.
 */

const goal: Chosen = {
  kind: "goal",
  id: "g1-s23",
  title: "g1-s23",
  source: "the accepted tip 6984cde, 17:54",
  summary: "Sign in with your code · queued · tier 2",
  to: "/backlog?goal=g1-s23",
};

const design: Chosen = {
  kind: "record",
  id: "plans/designs/user-interface/g1-s26.md",
  title: "g1-s26 When a design has shipped",
  source: "the checkout's records as they stand",
  summary: "design · done",
};

describe("the subject's lifecycle", () => {
  it("follows the page while nothing is chosen", () => {
    const page = { kind: "document", subject: "plans/designs/d1.md", title: "A design", revision: "blob:abc" };
    const subject = effectiveSubject(null, page);
    expect(subject?.kind).toBe("document");
    expect(subject?.id).toBe("plans/designs/d1.md");
    expect(effectiveSubject(null, {})).toBeNull();
  });

  it("stays on the chosen thing however the page changes under it", () => {
    // Chosen on the board, then the human opens a design: "it" is still the
    // goal, which is what makes a follow-up mean what they meant.
    expect(effectiveSubject(goal, {})).toEqual(goal);
    expect(effectiveSubject(goal, { kind: "document", subject: "plans/designs/d1.md" })).toEqual(goal);
  });

  it("is replaced by choosing another, and given back by clearing", () => {
    expect(effectiveSubject(design, { kind: "goal", subject: "g1-s23" })).toEqual(design);
    const page = { kind: "goal", subject: "g1-s23", title: "g1-s23" };
    expect(effectiveSubject(null, page)?.id).toBe("g1-s23");
  });

  it("is named by its kind and what it is, on the chip and in the panel", () => {
    expect(chipLabel(goal)).toBe("Goal g1-s23");
    expect(chipLabel(design)).toBe("Record g1-s26 When a design has shipped");
    expect(kindWord("lane")).toBe("Lane");
    expect(kindWord("passage")).toBe("Passage");
  });
});

describe("which questions are suggested", () => {
  it("follows the subject's kind, and the section where there is no subject", () => {
    expect(registerFor(goal, "Backlog")).toBe("goal");
    expect(registerFor(design, "Project")).toBe("design");
    expect(registerFor({ ...design, id: "plans/decisions/k1.md" }, "Project")).toBe("decision");
    expect(registerFor(null, "Backlog")).toBe("board");
    expect(registerFor(null, "Overview")).toBe("overview");
    expect(registerFor(null, "Fleet")).toBe("page");
  });

  it("offers the design's own three for a goal, and the design's own for a design", () => {
    expect(suggestionsFor("goal").map((one) => one.text)).toEqual([
      "Why is it here?",
      "What would move it?",
      "What does its design say?",
    ]);
    expect(suggestionsFor("design").map((one) => one.text)).toEqual([
      "Has its work shipped?",
      "What does it leave out?",
      "Which goals does it name?",
    ]);
    expect(suggestionsFor("board").map((one) => one.text)).toEqual([
      "What needs me here?",
      "What changed today?",
      "What is next up?",
    ]);
    expect(suggestionsFor("lane").map((one) => one.text)).toEqual(["What is in here, and why?"]);
    expect(suggestionsFor("decision").map((one) => one.text)).toEqual(["What did it decide, and why?"]);
  });

  /**
   * The two a handed-over sheet is worth asking, which are the only two in the
   * register that ask the Partner to write rather than to explain. They are
   * beside the draft's own chip, so they leave when it does.
   */
  it("offers the two that ask the Partner to write, for a sheet handed over", () => {
    expect(suggestionsFor(DRAFT_REGISTER).map((one) => one.text)).toEqual([
      "Suggest a better wording",
      "Suggest the next step",
    ]);
    for (const question of suggestionsFor(DRAFT_REGISTER)) {
      expect(question.scope).toContain("you decide about");
    }
  });

  // A question this page cannot scope is not suggested at all, so a register
  // this build has no questions for answers none rather than something else.
  it("says nothing for a kind it has no questions for", () => {
    expect(suggestionsFor("page")).toEqual([]);
    expect(suggestionsFor("nowhere")).toEqual([]);
  });

  it("gives every question a scope, and every scope words a human can read", () => {
    for (const register of registers()) {
      for (const suggestion of suggestionsFor(register)) {
        expect({ register, text: suggestion.text, scoped: suggestion.scope.length > 10 }).toEqual({
          register,
          text: suggestion.text,
          scoped: true,
        });
      }
    }
    // The two the critique named are scoped in words, not implied.
    expect(suggestionsFor("overview")[0].scope).toContain("Needs-you");
    expect(suggestionsFor("board")[1].scope).toContain("local midnight");
  });

  it("teaches the gesture as a sentence, never as a question that would send", () => {
    expect(HOW_TO_ASK).toContain("Ask about this");
    for (const register of registers()) {
      for (const suggestion of suggestionsFor(register)) {
        expect(suggestion.text).not.toBe(HOW_TO_ASK);
      }
    }
  });
});
