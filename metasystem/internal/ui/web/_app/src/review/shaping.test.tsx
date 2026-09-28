import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import type { ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { Desk, SourceShown } from "./Desk";
import { EndShapingWays, Room } from "./ReviewRoom";
import {
  endOf,
  firstDesk,
  localDeposit,
  openingDesk,
  ownSnapshot,
  roomWord,
  selectionPress,
  sittingDoorLine,
  walksOf,
  type DeskItem,
} from "./room";
import type { DocumentPayload } from "../project/api";
import { FileActions } from "../project/DocumentPane";
import { sittingRowPress } from "../project/pane";
import { Focused } from "../panes/Focused";
import { emptyStore, type Store } from "../partner/conversation";
import { DepositCard } from "../partner/Deposit";
import { cardsIn, countsIn, END, END_WITHOUT, entriesIn, interfaceLine, recordedIn, START } from "../partner/sitting";
import { PartnerAs } from "../partner/store";
import { documentPath, sittingPath } from "../routes";
import { Drawer } from "../shell/Drawer";

/**
 * The room for every sitting (g1-s67 §8, frontend): a sitting that shapes a
 * design or an intent opens the same room a review does, with its purpose's word
 * in the header, the record's sections on the desk, four walks of its own, Fact
 * beside Ask on a selection, the board's piles, and End by purpose; its door is
 * on the record's page and in the Sittings tab; the drawer and /brain show no
 * sitting.
 */

const DESIGN = "metasystem/plans/designs/g1-s66-the-thing.md";
const SOURCE =
  "# g1-s66: the thing\n\n- Kind: design\n- Status: draft\n\n## 1. What exists\n\nThe lock.\n\n## 2. Decisions\n\nD1.\n\n" +
  "## Facts\n\n- 2026-09-28 · Wido · the lock is taken in begin [d:local-1]\n  - Anchor: internal/ui/act/owner.go:41-88\n\n" +
  "## Proposals\n\n- 2026-09-28 · Wido · take it once\n  - Consequence: a second press waits\n\n" +
  "## Decisions\n\n- 2026-09-28 · Wido · one lock per goal\n  - Reason: two would race\n\n" +
  "## Open questions\n\n- 2026-09-28 · Wido · what a sleeping laptop holds\n  - Consequence: it may keep the lock\n";
const put = (item: DeskItem) => item;

function around(node: ReactNode, held: Parameters<typeof PartnerAs>[0]["held"] = {}): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <PartnerAs held={held}>{node}</PartnerAs>
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

function shaping(purpose = "shape a design", face: "desk" | "board" = "desk") {
  return {
    conversation: DESIGN,
    store: { ...emptyStore, conversation: DESIGN, state: "ready" as const },
    sitting: { subject: { kind: "record", id: DESIGN, title: "g1-s66: the thing" }, purpose, startedAt: "" },
    table: { counts: countsIn(SOURCE), entries: entriesIn(SOURCE), revision: "r1", source: SOURCE },
    room: { desk: firstDesk(DESIGN, SOURCE), face, drafts: {} },
  };
}

describe("the room of a sitting that shapes a record", () => {
  it("says the purpose's word in its header, with Board, End and Step out, and no rail", () => {
    const shown = around(<Room record={DESIGN} />, shaping());
    expect(shown).toContain("Shaping the design");
    expect(shown).toContain("g1-s66: the thing");
    expect(shown).not.toContain("Reviewing");
    expect(shown).not.toContain("at tip");
    expect(shown).toContain(">Board<");
    expect(shown).toContain(">End<");
    expect(shown).toContain(">Step out<");
    expect(shown).not.toContain("ms-rail");
    expect(around(<Room record={DESIGN} />, shaping("shape intent"))).toContain("Shaping the intent");
    expect(roomWord("review")).toBe("Reviewing");
  });

  it("stands its four walks under the conversation, and not a review's five", () => {
    const shown = around(<Room record={DESIGN} />, shaping());
    for (const walk of ["Records", "Today", "Cases", "Open"]) {
      expect(shown).toContain(`>${walk}<`);
    }
    for (const walk of ["Asked", "Built", "Examined", "Proven", "Behaves"]) {
      expect(shown).not.toContain(`>${walk}<`);
    }
    expect(walksOf("shape intent").map((one) => one.part)).toEqual(["records", "today", "cases", "open"]);
    expect(walksOf("review").map((one) => one.part)).toEqual(["asked", "built", "examined", "proven", "behaves"]);
  });

  it("opens the desk on the record's first section with its sections in the strip, and offers no change", () => {
    const desk = firstDesk(DESIGN, SOURCE);
    expect(desk.current).toBe(0);
    expect(desk.items.map((item) => (item.kind === "section" ? item.section : ""))).toEqual([
      "1. What exists", "2. Decisions", "Facts", "Proposals", "Decisions", "Open questions",
    ]);
    const shown = around(<Desk record={DESIGN} />, shaping());
    expect(shown).toMatch(/ms-desk-tab--up"[^>]*aria-current="true"[^>]*>g1-s66-the-thing.md § 1. What exists</u);
    expect(shown.indexOf("§ 1. What exists")).toBeLessThan(shown.indexOf("§ 2. Decisions"));
    expect(shown).not.toContain("The change");
  });

  it("opens a sitting whose mark carries no room state on its sections, and a kept desk as it was kept", () => {
    expect(openingDesk(undefined, "shape a design", DESIGN, SOURCE)).toEqual(firstDesk(DESIGN, SOURCE));
    const kept = { items: [{ kind: "source" as const, path: "a.go", from: 1, to: 2 }], current: 0 };
    expect(openingDesk(kept, "shape a design", DESIGN, SOURCE)).toEqual(kept);
    expect(openingDesk(undefined, "review", DESIGN, SOURCE)).toEqual({ items: [], current: -1 });
    expect(openingDesk(undefined, "shape a design", DESIGN, "")).toEqual({ items: [], current: -1 });
  });

  it("shows a file as the checkout has it now, with no commit and no diff", () => {
    const shown = around(<SourceShown put={put} source={{
      path: "internal/ui/act/owner.go", commit: "", checkout: true, from: 41, to: 42, total: 120,
      lines: [{ number: 41, text: "func (o *owner) begin() error {" }, { number: 42, text: "\to.mu.Lock()" }],
    }} />);
    expect(shown).toContain("lines 41–42 of 120 · as the checkout has it now");
    expect(shown).not.toContain("Its diff");
    expect(shown).toContain('data-ask-surface="desk"');
    expect(shown).toContain('data-line="41"');
  });

  it("offers Fact beside Ask on a selection, a card with the anchor filled and the words empty", () => {
    expect(selectionPress("shape a design")).toEqual({ label: "Fact", kind: "fact" });
    expect(selectionPress("shape intent")).toEqual({ label: "Fact", kind: "fact" });
    expect(selectionPress("review")).toEqual({ label: "Finding", kind: "finding" });
    const subject = { kind: "record", id: DESIGN, title: "g1-s66: the thing" };
    const fact = localDeposit({ text: "", clause: "internal/ui/act/owner.go:41-44", kind: "fact" }, subject);
    expect(fact).toEqual({ kind: "fact", text: "", anchor: "internal/ui/act/owner.go:41-44", consequence: "",
      offered: true, subject });
    const finding = localDeposit({ text: "", clause: "a.go:1", kind: "finding", consequence: "it holds" }, subject);
    expect(finding.kind).toBe("finding");
    expect(finding.consequence).toBe("it holds");
    const card = cardsIn([{ turn: "t1", deposits: [{ ...fact, text: "" }] }], {}, DESIGN, recordedIn(""))[0];
    const shown = around(<DepositCard id={card.id} />, { deposits: [card], sitting: shaping().sitting });
    expect(shown).toContain('value="internal/ui/act/owner.go:41-44"');
    expect(shown).toContain(">Record it<");
  });

  it("flips to the board, where the record's piles stand", () => {
    const shown = around(<Room record={DESIGN} />, shaping("shape a design", "board"));
    expect(shown).toContain(">Desk<");
    for (const pile of ["Facts", "Proposals", "Decisions", "Open questions"]) {
      expect(shown).toContain(`aria-label="${pile}"`);
    }
    expect(shown).toContain("the lock is taken in begin");
    expect(shown).toContain(">internal/ui/act/owner.go:41-88<");
    expect(shown).toContain("what a sleeping laptop holds");
  });

  it("ends by purpose: a shaping room drafts the Outcome or ends without recording, and no verdict", () => {
    expect(endOf("shape a design")).toBe("outcome");
    expect(endOf("shape intent")).toBe("outcome");
    expect(endOf("review")).toBe("verdict");
    const shown = around(<EndShapingWays busy={false} onDraft={() => undefined} onWithout={() => undefined} />);
    expect(shown).toContain(`>${END}<`);
    expect(shown).toContain(`>${END_WITHOUT}<`);
    expect(shown).not.toContain("Clear to land");
    expect(shown).not.toContain("Send back");
  });

  it("names the interface's walk in the transcript", () => {
    expect(interfaceLine(`Walk me through Today for the sitting on ${DESIGN}: what the application does today.`))
      .toBe("asked by the interface: the Today walk");
  });
});

/** One design record's page payload, with a sitting standing on it or not. */
function page(sitting?: DocumentPayload["sitting"]): DocumentPayload {
  return {
    kind: "document", id: DESIGN, title: "g1-s66: the thing", revision: "r1", source: SOURCE, owner: "wido",
    path: `/checkout/${DESIGN}`, bytes: 1, modifiedAt: "", readAt: "", state: "readable", reason: "",
    record: { kind: "design" } as DocumentPayload["record"], referencedBy: [], supersededBy: [], headings: [], blocks: [],
    sitting,
  };
}

describe("the room's own snapshot", () => {
  it("is the one read for its record, and never the snapshot the page held before it", () => {
    const sitting = shaping().sitting;
    // The ordinary conversation, with a sitting marked on it before D16, is what
    // the page held when the room was entered: the room must not take its mark
    // for its own and close when the store moves to the room (walkthrough, g1-s67 D6).
    const held = (over: Partial<Store>): Store => ({ ...emptyStore, ...over });
    expect(ownSnapshot(held({ state: "ready", conversation: "", sitting }), DESIGN)).toBe(false);
    expect(ownSnapshot(held({ state: "loading", conversation: DESIGN }), DESIGN)).toBe(false);
    expect(ownSnapshot(held({ state: "ready", conversation: DESIGN, sitting }), DESIGN)).toBe(true);
    expect(ownSnapshot(held({ state: "ready", conversation: DESIGN, sitting: null }), DESIGN)).toBe(true);
  });
});

describe("the door", () => {
  it("stands on the record's page while a sitting stands on it, in place of Start", () => {
    const now = new Date("2026-09-28T12:00:00Z");
    const shown = around(<FileActions path={`/checkout/${DESIGN}`} now={now}
      document={page({ purpose: "shape a design", steppedOutAt: "2026-09-28T10:00:00Z" })}
      onEdit={null} onNewGoal={null} busy="" />);
    expect(shown).toContain("In a sitting · you stepped out 2h ago");
    expect(shown).not.toContain(`>${START}<`);
    const bare = around(<FileActions path={`/checkout/${DESIGN}`} document={page({ purpose: "shape a design" })}
      onEdit={null} onNewGoal={null} busy="" />);
    expect(bare).toContain("In a sitting");
    expect(around(<FileActions path={`/checkout/${DESIGN}`} document={page()} onEdit={null} onNewGoal={null} busy="" />))
      .toContain(`>${START}<`);
    expect(sittingDoorLine("", now)).toBe("In a sitting");
  });

  it("is the Sittings tab's row: a standing sitting opens its room, room state or none, and an ended one starts again", () => {
    const row = {
      record: { kind: "design", id: "01D", path: DESIGN, title: "g1-s66: the thing" },
      counts: { facts: 1, proposals: 0, decisions: 0, questions: 0 }, lastAt: "", standing: true,
    };
    expect(sittingRowPress(row)).toEqual({ go: sittingPath(DESIGN) });
    expect(sittingRowPress({ ...row, steppedOutAt: "2026-09-28T10:00:00Z" })).toEqual({ go: sittingPath(DESIGN) });
    expect(sittingRowPress({ ...row, standing: false })).toEqual({
      start: { purpose: "shape a design", subject: { kind: "record", id: DESIGN, title: "g1-s66: the thing" } },
    });
    expect(sittingRowPress({ ...row, standing: false, record: { ...row.record, kind: "intent" } })).toEqual({
      start: { purpose: "shape intent", subject: { kind: "record", id: DESIGN, title: "g1-s66: the thing" } },
    });
    const review = { ...row, record: { ...row.record, kind: "review", path: "plans/reviews/r.md" } };
    expect(sittingRowPress(review)).toEqual({ go: "/review/plans/reviews/r.md" });
    expect(sittingRowPress({ ...review, standing: false })).toEqual({ go: documentPath("plans/reviews/r.md") });
  });
});

describe("the drawer and the focused view", () => {
  const standing = { subject: { kind: "record", id: DESIGN, title: "g1-s66: the thing" }, purpose: "shape a design",
    startedAt: "" };

  it("show no sitting: no Start, no chip, no counts, no End in the drawer", () => {
    for (const open of [false, true]) {
      const shown = around(
        <Drawer open={open} caret="none" onCompose={() => undefined} onToggle={() => undefined} onEscape={() => undefined} />,
        { sitting: standing },
      );
      expect(shown).not.toContain(START);
      expect(shown).not.toContain("Sitting: ");
      expect(shown).not.toContain("ms-sitting-counts");
      expect(shown).not.toContain(END);
      expect(shown).not.toContain("Your conversation");
    }
  });

  it("show no table at /brain", () => {
    const shown = around(<Focused />, { sitting: standing, table: { counts: countsIn(SOURCE), entries: entriesIn(SOURCE),
      revision: "r1", source: SOURCE } });
    expect(shown).not.toContain("ms-table");
    expect(shown).not.toContain("the lock is taken in begin");
  });
});
