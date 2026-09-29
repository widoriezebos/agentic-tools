import { describe, expect, it } from "vitest";

import type { Source } from "./api";
import { parseAnchor } from "./room";
import {
  cardOf,
  remarkOf,
  remarkPresses,
  EARLIER_READING,
  remarkAbout,
  remarkAnchor,
  remarkDrawn,
  remarksOn,
  remarkWhere,
  sectionRemark,
  type Remark,
} from "./remarks";
import type { About, Notepad, Sticky } from "../stickies/api";

/**
 * Remarks (g1-s71 D1): stickies about lines at a commit or about a record's
 * section, shown on the board under Remarks and on the desk item they belong to
 * as a marker — but never on lines that may be other code.
 */

const RECORD = "plans/reviews/review-of-landing.md";
const TIP = "9c1f0a2e".repeat(5);
const NEW_TIP = "7d3b1e40".repeat(5);
const HEAD_A = "aaaaaaa1".repeat(5);
const HEAD_B = "bbbbbbb2".repeat(5);

function read(over: Partial<Source>): Source {
  const lines = over.lines ?? [
    { number: 40, text: "func held() {" },
    { number: 41, text: "\tmu.Lock()" },
    { number: 42, text: "\tmu.Lock()" },
    { number: 43, text: "}" },
  ];
  return { path: "internal/owner.go", commit: TIP, from: 40, to: 43, total: 90, lines, ...over };
}

function sticky(id: string, text: string, about: About[], doneAt = ""): Sticky {
  return { id, text, about, createdAt: "2026-09-29T09:00:00Z", updatedAt: "2026-09-29T09:00:00Z", doneAt };
}

function notepad(stickies: Sticky[]): Notepad {
  return { schemaVersion: 1, human: "wido", stickies, counts: { open: stickies.length, done: 0 } };
}

describe("a remark made on the desk", () => {
  it("is about the lines at the commit the review's desk read them at, and keeps no text", () => {
    expect(remarkAbout(RECORD, read({}), 41, 42)).toEqual({
      kind: "source", id: RECORD, record: RECORD, path: "internal/owner.go", from: 41, to: 42, commit: TIP,
    });
  });

  it("in a shaping room keeps the checkout's head as provenance and the text of its lines", () => {
    const shaping = read({ commit: "", checkout: true, head: HEAD_A });
    expect(remarkAbout(RECORD, shaping, 41, 42)).toEqual({
      kind: "source", id: RECORD, record: RECORD, path: "internal/owner.go", from: 41, to: 42, commit: HEAD_A,
      lines: "\tmu.Lock()\n\tmu.Lock()",
    });
  });

  it("on a section is about the record and the section", () => {
    expect(sectionRemark(RECORD, "Findings")).toEqual({ kind: "section", id: RECORD, record: RECORD, section: "Findings" });
  });
});

describe("the remarks on one sitting's record", () => {
  it("are this record's open stickies about lines or a section, each once per subject", () => {
    const lines = remarkAbout(RECORD, read({}), 41, 42);
    const other = { ...lines, record: "plans/reviews/other.md", id: "plans/reviews/other.md" };
    const found = remarksOn(
      notepad([
        sticky("s1", "this lock is taken twice", [lines]),
        sticky("s2", "on another record", [other]),
        sticky("s3", "a goal's note", [{ kind: "goal", id: "g1-s9" }]),
        sticky("s4", "struck off", [lines], "2026-09-29T10:00:00Z"),
        sticky("s5", "the piles", [sectionRemark(RECORD, "Findings")]),
      ]),
      RECORD,
    );
    expect(found.map((remark: Remark) => remark.sticky.id)).toEqual(["s1", "s5"]);
  });
});

describe("where a remark in a review room is drawn", () => {
  const lines = remarkAbout(RECORD, read({}), 41, 42);

  it("is on its lines while the desk reads its commit", () => {
    expect(remarkDrawn(lines, read({}), "review")).toBe(true);
    expect(remarkWhere(lines, { purpose: "review", tip: TIP, read: null })).toBe("on lines 41-42 of owner.go");
  });

  it("leaves the lines after Review the new tip, and the board names its commit", () => {
    const after = read({ commit: NEW_TIP });
    expect(remarkDrawn(lines, after, "review")).toBe(false);
    expect(remarkWhere(lines, { purpose: "review", tip: NEW_TIP, read: after })).toBe("on lines 41-42 at 9c1f0a2");
  });

  it("is never drawn on another file's lines", () => {
    expect(remarkDrawn(lines, read({ path: "internal/other.go" }), "review")).toBe(false);
  });

  it("is made into a fact or a finding whose anchor keeps its commit", () => {
    expect(remarkAnchor(lines)).toBe("internal/owner.go:41-42 at 9c1f0a2e9");
    expect(parseAnchor(remarkAnchor(lines))).toEqual({ kind: "source", path: "internal/owner.go", from: 41, to: 42 });
    expect(remarkAnchor(sectionRemark(RECORD, "Findings"))).toBe(`${RECORD} § Findings`);
  });
});

describe("shaping_remark_after_uncommitted_edit", () => {
  // Remark at head A, step out, edit those lines without committing, return: no
  // marker on the current lines, the remark on the board with its words and "at
  // an earlier reading"; the marker back when the lines are restored.
  const atA = read({ commit: "", checkout: true, head: HEAD_A });
  const remark = remarkAbout(RECORD, atA, 41, 42);
  const edited = read({
    commit: "", checkout: true, head: HEAD_A,
    lines: [
      { number: 40, text: "func held() {" },
      { number: 41, text: "\tmu.Lock()" },
      { number: 42, text: "\tdefer mu.Unlock()" },
      { number: 43, text: "}" },
    ],
  });

  it("is drawn on the lines it was made on", () => {
    expect(remarkDrawn(remark, atA, "shape a design")).toBe(true);
    expect(remarkWhere(remark, { purpose: "shape a design", tip: "", read: atA })).toBe("on lines 41-42 of owner.go");
  });

  it("is not drawn once the lines under the same head are other bytes, and the board says an earlier reading", () => {
    expect(remarkDrawn(remark, edited, "shape a design")).toBe(false);
    expect(remarkWhere(remark, { purpose: "shape a design", tip: "", read: edited })).toBe(`on lines 41-42 ${EARLIER_READING}`);
  });

  it("is drawn again when the lines are restored", () => {
    const restored = read({ commit: "", checkout: true, head: HEAD_A });
    expect(remarkDrawn(remark, restored, "shape a design")).toBe(true);
  });

  it("is not drawn once the head has moved, and the board names the head it was made at", () => {
    const moved = read({ commit: "", checkout: true, head: HEAD_B });
    expect(remarkDrawn(remark, moved, "shape a design")).toBe(false);
    expect(remarkWhere(remark, { purpose: "shape a design", tip: "", read: moved })).toBe("on lines 41-42 at aaaaaaa");
  });

  it("is not drawn on lines the read does not show whole", () => {
    expect(remarkDrawn(remark, read({ commit: "", checkout: true, head: HEAD_A, from: 42, to: 43, lines: [{ number: 42, text: "\tmu.Lock()" }, { number: 43, text: "}" }] }), "shape a design")).toBe(false);
  });
});

describe("a remark from a press (D1)", () => {
  it("from a selection of lines is about those lines as the desk read them", () => {
    const shown = read({});
    expect(remarkOf("internal/owner.go:41-42", RECORD, shown)).toEqual(remarkAbout(RECORD, shown, 41, 42));
    expect(remarkOf("internal/owner.go:41", RECORD, shown)).toEqual(remarkAbout(RECORD, shown, 41, 41));
  });

  it("from a selection in a section, or from a pile on the board, is about the section", () => {
    expect(remarkOf(`${RECORD} § Findings`, RECORD, null)).toEqual(sectionRemark(RECORD, "Findings"));
  });

  it("is refused where the lines were not read by this desk", () => {
    expect(remarkOf("internal/other.go:41-42", RECORD, read({}))).toBeNull();
    expect(remarkOf("internal/owner.go:41-42", RECORD, null)).toBeNull();
    expect(remarkOf("some words", RECORD, read({}))).toBeNull();
  });

  it("offers Record as a fact, and in a review Make a finding, each filled with its words and its anchor", () => {
    expect(remarkPresses("review").map((press) => press.label)).toEqual(["Record as a fact", "Make a finding"]);
    expect(remarkPresses("shape a design").map((press) => press.label)).toEqual(["Record as a fact"]);
    const remark: Remark = { sticky: sticky("s1", "this lock is taken twice", []), about: remarkAbout(RECORD, read({}), 41, 42) };
    expect(cardOf(remark, "finding")).toEqual({ anchor: "internal/owner.go:41-42 at 9c1f0a2e9", kind: "finding", text: "this lock is taken twice" });
  });
});
