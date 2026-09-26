import { describe, expect, it } from "vitest";

import { BacklogError, type Row } from "./api";
import {
  blockedForEdit,
  changedIn,
  draftOf,
  editable,
  editNote,
  editReason,
  labelRefusal,
  LANDED_NOT_RECORDED,
  nothingChanged,
  outcomeOf,
} from "./editing";
import { offersFor } from "./menu";

/**
 * What the edit sheet sends, and which card offers it.
 *
 * The claim worth a test here is subtraction: a save carries the fields that
 * differ from what the sheet opened with and no others, because a browser
 * that republished all three would overwrite a terminal's edit of a field
 * nobody in this browser touched.
 */

function row(over: Partial<Row> = {}): Row {
  return {
    ref: { kind: "goal", id: "ui-1", revision: 1 },
    where: "plans/goals/ui-1.md",
    lane: "to-do",
    phase: "",
    state: "queued",
    intent: "The board reads.",
    nextStep: "Take it to an end state.",
    concluded: "",
    origin: "human",
    priority: 2,
    sequence: 1,
    tier: 1,
    labels: ["board", "ui"],
    arc: "",
    pinned: "",
    blockedBy: [],
    holds: [],
    openBlockers: [],
    sliced: false,
    decomposed: false,
    openedAt: "",
    doneAt: "",
    lastChangeAt: "",
    lastVerb: "open",
    gaps: [],
    ...over,
  };
}

const approval = {
  by: "human:Wido",
  at: "2026-09-24T08:00:00Z",
  authority: "session",
  reviewBy: "",
  expired: false,
  expiredWhy: "",
};

describe("the draft a sheet opens on", () => {
  it("is the goal as the ledger has it, labels as one line", () => {
    expect(draftOf(row())).toEqual({
      intent: "The board reads.",
      nextStep: "Take it to an end state.",
      labels: "board ui",
    });
  });
});

describe("what a save carries", () => {
  it("is nothing at all when nothing was touched", () => {
    const opened = draftOf(row());
    expect(changedIn(opened, opened)).toEqual({});
    expect(nothingChanged(changedIn(opened, opened))).toBe(true);
  });

  it("is the one field that differs, and never the two that do not", () => {
    const opened = draftOf(row());
    const edit = changedIn(opened, { ...opened, nextStep: "Something else entirely." });
    expect(edit).toEqual({ nextStep: "Something else entirely." });
    expect(Object.hasOwn(edit, "intent")).toBe(false);
    expect(Object.hasOwn(edit, "labels")).toBe(false);
  });

  // The ledger keeps each line on one line, so the sheet folds a break rather
  // than refusing a paragraph — and a break is therefore not a change.
  it("folds the lines it sends, and a break alone changes nothing", () => {
    const opened = draftOf(row());
    expect(changedIn(opened, { ...opened, intent: "The board\nreads." })).toEqual({});
    expect(changedIn(opened, { ...opened, intent: "The board\nreads it all." })).toEqual({
      intent: "The board reads it all.",
    });
  });

  it("sends an emptied label list as an empty list, which clears them", () => {
    const opened = draftOf(row());
    expect(changedIn(opened, { ...opened, labels: "  " })).toEqual({ labels: [] });
  });

  // The engine writes labels sorted and deduplicated, so reordering the words
  // a goal already carries is not a change to send.
  it("says nothing about labels that were only reordered or repeated", () => {
    const opened = draftOf(row());
    expect(changedIn(opened, { ...opened, labels: "ui board" })).toEqual({});
    expect(changedIn(opened, { ...opened, labels: "ui board ui" })).toEqual({});
    expect(changedIn(opened, { ...opened, labels: "ui board refunds" })).toEqual({
      labels: ["ui", "board", "refunds"],
    });
  });
});

describe("why the button is disabled", () => {
  it("names the blank line rather than saying something is missing", () => {
    const opened = draftOf(row());
    expect(blockedForEdit({ ...opened, intent: "  " }, { intent: "" })).toContain("what done looks like");
    expect(blockedForEdit({ ...opened, nextStep: "" }, { nextStep: "" })).toContain("never a script of the how");
  });

  it("refuses a label outside the grammar in the engine's own sentence", () => {
    expect(labelRefusal("board ui")).toBe("");
    expect(labelRefusal("Board")).toBe('label "Board" must match ^[a-z][a-z0-9-]{0,31}$');
    expect(labelRefusal("1st")).toBe('label "1st" must match ^[a-z][a-z0-9-]{0,31}$');
    const opened = draftOf(row());
    expect(blockedForEdit({ ...opened, labels: "Board" }, { labels: ["Board"] })).toBe(
      'label "Board" must match ^[a-z][a-z0-9-]{0,31}$',
    );
  });

  it("refuses to send a sheet in which nothing has changed", () => {
    const opened = draftOf(row());
    expect(blockedForEdit(opened, changedIn(opened, opened))).toContain("Nothing has changed");
    expect(blockedForEdit(opened, changedIn(opened, { ...opened, intent: "New." }))).toBe("");
  });
});

describe("which goal may be edited here", () => {
  it("is a queued one nobody has approved, and no other", () => {
    expect(editable(row())).toBe(true);
    expect(editable(row({ approved: approval }))).toBe(false);
    expect(editable(row({ state: "approved", approved: approval }))).toBe(false);
    expect(editable(row({ state: "claimed" }))).toBe(false);
    expect(editable(row({ state: "parked" }))).toBe(false);
    expect(editable(row({ state: "done" }))).toBe(false);
  });

  it("says which act would let it through, in words from the state", () => {
    expect(editReason(row())).toBe("");
    expect(editReason(row({ state: "approved", approved: approval }))).toBe(
      "approved: withdraw the approval to edit the intent",
    );
    expect(
      editReason(
        row({
          state: "claimed",
          approved: approval,
          claim: { machine: "mac-b", lineage: "lin-1", at: "", landingAt: "" },
        }),
      ),
    ).toBe("claimed by mac-b+lin-1: edit at a terminal");
    // An approval survives a claim and a park, so the state is read first or
    // a goal a seat holds would be told to withdraw an approval instead.
    expect(editReason(row({ state: "parked", approved: approval }))).toBe(
      "parked: return it to the queue to edit",
    );
  });

  // A goal that is over has no act that would let it be edited, so there is
  // no sentence to show: inventing one would offer an edit that is not there.
  it("says nothing about work that is over", () => {
    expect(editReason(row({ state: "done", lane: "done" }))).toBe("");
    expect(editReason(row({ state: "abandoned", lane: "abandoned" }))).toBe("");
  });
});

describe("the card menu", () => {
  it("offers Edit… on a queued unapproved card, between Ask and the lane moves", () => {
    const card = row();
    const offers = offersFor(card, [card]);
    const labels = offers.map((offer) => offer.label);
    expect(labels[0]).toBe("Ask about this");
    expect(labels[1]).toBe("Edit…");
    expect(offers.map((offer) => offer.id)).toContain("edit");
  });

  it("offers it on no other card", () => {
    for (const over of [
      { approved: approval },
      { state: "approved", approved: approval, lane: "ready" as const },
      { state: "claimed" as const, lane: "in-progress" as const },
      { state: "parked" as const, lane: "waiting" as const },
      { state: "done" as const, lane: "done" as const },
    ]) {
      const card = row(over as Partial<Row>);
      expect(offersFor(card, [card]).map((offer) => offer.id)).not.toContain("edit");
    }
  });
});

describe("what the sheet says the act will do", () => {
  it("names the fields it is sending and says the rest is left alone", () => {
    expect(editNote("ui-1", { intent: "x" })).toBe(
      "Publishes goal edit for ui-1, sending the intent and leaving every other field as the ledger has it.",
    );
    expect(editNote("ui-1", { intent: "x", nextStep: "y", labels: [] })).toContain(
      "sending the intent, the next step and the labels",
    );
    expect(editNote("ui-1", {})).toBe("Publishes nothing for ui-1 until something changes.");
  });
});

/**
 * What a failed save means, which is the one judgement this page makes about an
 * act it cannot see.
 *
 * The route has no unresolved outcome: the act layer collapses every publication
 * error into a refusal, and one of its answers says the act LANDED and must not
 * be run again (Astra F1 on g1-s56). So the reading is conservative — a refusal
 * is only what nothing landed behind, and everything else is unresolved, after
 * which this page rereads and never sends the act again by itself.
 */
describe("what came back from a save", () => {
  /** A refusal the server explained, as the one request in this build builds it. */
  function refusal(status: number, code: string, reason: string): BacklogError {
    return new BacklogError("/api/backlog/goals/ui-1/edit", status, reason, code);
  }

  it("is a refusal where the ledger refused the act in the state it is in", () => {
    const said = "goal ui-1 is approved: withdraw the approval, edit it, then approve it again";
    expect(outcomeOf(refusal(409, "refused", said))).toEqual({ kind: "refused", words: said });
    expect(outcomeOf(refusal(409, "rejected", said))).toEqual({ kind: "refused", words: said });
  });

  it("is a refusal where the request itself was wrong, or the seat is nobody's", () => {
    expect(outcomeOf(refusal(400, "no-change", "an edit changes at least one field")).kind).toBe("refused");
    expect(outcomeOf(refusal(403, "unproven", "nobody is signed in here")).kind).toBe("refused");
  });

  /**
   * The answer that says the act landed and its proof did not. It is never a
   * refusal: calling it one would tell a human the opposite of what happened and
   * invite the one press that would publish the edit twice.
   */
  it("keeps the landed-but-unrecorded answer in its own words", () => {
    const said =
      "the act landed at tip 6984cde, but its authority proof did not: no such file; do not run it again";
    const outcome = outcomeOf(refusal(500, LANDED_NOT_RECORDED, said));
    expect(outcome).toEqual({ kind: "unresolved", words: said });
    expect(outcome.words).toContain("do not run it again");
  });

  it("is unresolved where the engine could not answer at all", () => {
    expect(outcomeOf(refusal(500, "failed", "the ledger could not be written")).kind).toBe("unresolved");
  });

  /**
   * And where the ledger answered with an outcome that is neither a confirmation
   * nor a rejection: the journal lost the operation, or confirmed it late, which
   * means it landed.
   */
  it("is unresolved where the ledger neither confirmed nor rejected", () => {
    for (const code of ["confirmed-late", "lost", "abandoned", "expired"]) {
      expect(outcomeOf(refusal(409, code, "the ledger did not confirm goal edit")).kind).toBe("unresolved");
    }
  });

  it("is unresolved where nothing came back that could be read", () => {
    expect(outcomeOf(new TypeError("Load failed"))).toEqual({ kind: "unresolved", words: "Load failed" });
    expect(outcomeOf("something nobody typed")).toEqual({
      kind: "unresolved",
      words: "something nobody typed",
    });
  });
});
