import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import type { ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { Anchored } from "./anchors";
import { ChangesShown, Desk, DiffShown, SourceShown } from "./Desk";
import { ReviewItOrDoor } from "./Door";
import { EndWays, Room } from "./ReviewRoom";
import { NOD_LINE, type DeskItem } from "./room";
import type { Backlog, Row } from "../backlog/api";
import { Board } from "../backlog/Board";
import { noFilters } from "../backlog/filters";
import { emptyStore } from "../partner/conversation";
import { DepositCard } from "../partner/Deposit";
import { entriesIn, type Card, type Entry } from "../partner/sitting";
import { PartnerAs } from "../partner/store";
import { reviewable } from "../project/ProjectPane";

/**
 * The review room's screens (g1-s65 §8, frontend), read from their markup over
 * the store's own readings: where Review it stands and where its door does, the
 * room's own header with no rail, the desk's views, the finding card with its
 * anchor and its answers, and End's three ways with Clear to land refused while
 * a finding is unanswered.
 */

const RECORD = "metasystem/plans/reviews/review-of-g1-s64.md";
const TIP = "a1".repeat(20);
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

function row(over: Partial<Row> = {}): Row {
  return {
    ref: { kind: "goal", id: "ui-1", revision: 1 }, where: "plans/goals/ui-1.md", lane: "to-do", phase: "",
    state: "open", intent: "The board reads.", nextStep: "", concluded: "", origin: "human", priority: 2,
    sequence: 1, tier: 1, labels: [], arc: "", pinned: "", blockedBy: [], holds: [], openBlockers: [],
    sliced: false, decomposed: false, openedAt: "2026-09-22T09:00:00Z", doneAt: "",
    lastChangeAt: "2026-09-22T09:00:00Z", lastVerb: "goal open", gaps: [], ...over,
  };
}

function board(rows: Row[], closed: Row[] = [], reviews: Backlog["reviews"] = []): Backlog {
  return {
    schemaVersion: 1, observedAt: "2026-09-28T10:00:00Z",
    ledger: {
      state: "read", tip: "2ef5", committedAt: "2026-09-28T10:00:00Z",
      freshness: { state: "current", since: "", detail: "" }, stale: false, staleAfterSeconds: 3600,
      syncMode: "", stateRoot: "", message: "", problems: [],
      fetch: { outcome: "current", startedAt: "", finishedAt: "", tip: "", detail: "", message: "", failures: 0,
        cadence: "5m", nextAt: "", succeededAt: "", succeededTip: "" },
    },
    admission: { answered: true, message: "" }, workingTree: { liveFiles: 1, archivedFiles: 0 },
    authority: { proven: true, human: "Wido", reason: "" }, budgetDefaults: {}, counts: {},
    draft: { statement: "" }, rows, closed, reviews,
  } as Backlog;
}

function boardMarkup(backlog: Backlog): string {
  return around(
    <Board backlog={backlog} view="board" closedShown={false} onToggleClosed={() => undefined}
      onAct={() => undefined} onMoved={() => undefined} plans={null} filters={noFilters} window={1}
      onWindow={() => undefined} showing="" onFaded={() => undefined} />,
  );
}

describe("Review it", () => {
  it("stands on the Review lane's card and on no other card", () => {
    const shown = boardMarkup(board([
      row({ ref: { kind: "goal", id: "g1-s64", revision: 1 }, lane: "review" }),
      row({ ref: { kind: "goal", id: "g1-s65", revision: 1 }, lane: "to-do" }),
      row({ ref: { kind: "goal", id: "g1-s66", revision: 1 }, lane: "in-progress" }),
    ]));
    expect(shown.match(/>Review it</g)).toHaveLength(1);
  });

  it("is the door once a review stands, read from the record and the room", () => {
    const shown = around(
      <ReviewItOrDoor goal="g1-s64" now={new Date("2026-09-28T12:00:00Z")} doors={[{
        goal: "g1-s64", record: RECORD, title: "Review of g1-s64", findings: 3, unanswered: 1,
        standing: true, steppedOutAt: "2026-09-28T10:00:00Z",
      }]} />,
    );
    expect(shown).toContain("In review · you stepped out 2h ago · 3 findings, 1 unanswered");
    expect(shown).not.toContain(">Review it<");
  });

  it("stands on a goal page waiting to land and a done goal's page, and nowhere else", () => {
    const ledger = board(
      [row({ ref: { kind: "goal", id: "waiting", revision: 1 }, lane: "review" }),
        row({ ref: { kind: "goal", id: "todo", revision: 1 }, lane: "to-do" })],
      [row({ ref: { kind: "goal", id: "landed", revision: 1 }, lane: "done" }),
        row({ ref: { kind: "goal", id: "dropped", revision: 1 }, lane: "abandoned" })],
    );
    expect(["waiting", "landed", "todo", "dropped", "absent"].map((goal) => reviewable(ledger, goal)))
      .toEqual([true, true, false, false, false]);
  });
});

const REVIEW_SOURCE = `# Review of g1-s64\n\n- Kind: review\n- Goals: g1-s64\n- Reviewed: ${TIP} (the tip of goal/g1-s64)\n\n` +
  "## Facts\n\n## Findings\n\n" +
  "- 2026-09-28 · Wido · a press that dies holds the lock [d:deposit:t1#0]\n" +
  "  - Anchor: internal/owner/owner.go:21-24\n  - Answer: unanswered\n\n## Decisions\n\n## Open questions\n";

function roomHeld(source: string, face: "desk" | "board" = "desk") {
  return {
    conversation: RECORD,
    store: { ...emptyStore, conversation: RECORD, state: "ready" as const },
    sitting: { subject: { kind: "record", id: RECORD, title: "Review of g1-s64" }, purpose: "review", startedAt: "" },
    table: { counts: { Facts: 0, Proposals: 0, Decisions: 0, "Open questions": 0, Findings: 1 }, entries: entriesIn(source),
      revision: "r1", source },
    room: { desk: { items: [{ kind: "source" as const, path: "internal/owner/owner.go", from: 14, to: 27 },
      { kind: "changes" as const }], current: 0 }, face, drafts: {} },
  };
}

describe("the room", () => {
  it("is its own screen: the goal, the tip and the record's counts, Board and Step out, and no rail", () => {
    const shown = around(<Room record={RECORD} />, roomHeld(REVIEW_SOURCE));
    expect(shown).toContain("Reviewing");
    expect(shown).toContain("g1-s64");
    expect(shown).toContain(`at tip ${TIP.slice(0, 9)}`);
    expect(shown).toContain("1 finding, 1 unanswered");
    expect(shown).toContain(">Board<");
    expect(shown).toContain(">Step out<");
    expect(shown).toContain(">End<");
    expect(shown).not.toContain("ms-rail");
    // The five walks are presses beside the conversation.
    for (const walk of ["Asked", "Built", "Examined", "Proven", "Behaves"]) {
      expect(shown).toContain(`>${walk}<`);
    }
  });

  it("keeps a strip of what has been on the desk, newest first, the one up marked", () => {
    const shown = around(<Desk record={RECORD} />, roomHeld(REVIEW_SOURCE));
    expect(shown.indexOf("owner.go:14-27")).toBeLessThan(shown.indexOf(">changes<"));
    expect(shown).toMatch(/ms-desk-tab--up"[^>]*aria-current="true"[^>]*>owner.go:14-27</u);
  });

  it("puts the Latest pill on the composer's top edge, outside the conversation's scroller (UX-1)", () => {
    const shown = around(<Room record={RECORD} />, roomHeld(REVIEW_SOURCE));
    expect(shown).toMatch(/<div class="ms-room-talk"><div class="ms-room-transcript/u);
    const css = readFileSync(fileURLToPath(new URL("./room.css", import.meta.url)), "utf8");
    expect(css).toMatch(/\.ms-room-talk \.ms-partner-latest \{[^}]*position: absolute;/u);
    expect(css).toMatch(/\.ms-room-talk \.ms-partner-transcript \{[^}]*position: static;/u);
  });

  it("collapses an empty desk to its one line at phone width, and keeps a desk with an item (UX-2)", () => {
    const held = roomHeld(REVIEW_SOURCE);
    const empty = around(<Room record={RECORD} />, { ...held, room: { ...held.room, desk: { items: [], current: -1 } } });
    expect(empty).toContain("ms-room-desk ms-room-desk--empty");
    expect(around(<Room record={RECORD} />, held)).not.toContain("ms-room-desk--empty");
    const css = readFileSync(fileURLToPath(new URL("./room.css", import.meta.url)), "utf8");
    const phone = css.slice(css.indexOf("@media (max-width: 640px)"));
    expect(phone).toMatch(/\[data-panel\]:has\(> \.ms-room-desk--empty\) \{\s*flex: 0 0 auto !important;/u);
  });

  it("flips to the board, where a finding carries its answers", () => {
    const shown = around(<Room record={RECORD} />, roomHeld(REVIEW_SOURCE, "board"));
    expect(shown).toContain(">Desk<");
    expect(shown).toContain("a press that dies holds the lock");
    expect(shown).toContain("Fix in this goal");
    expect(shown).toContain("Follow-up goal");
    expect(shown).toContain("Accept, with reason");
    expect(shown).toContain("Leave open");
  });
});

describe("the desk's views", () => {
  it("shows a file at a range with the touched lines marked and numbered for a selection", () => {
    const shown = around(<SourceShown put={put} source={{
      path: "internal/owner/owner.go", commit: TIP, from: 13, to: 15, total: 33,
      lines: [{ number: 13, text: "// begin" }, { number: 14, text: "func (o *owner) begin() error {", touched: true },
        { number: 15, text: "\to.mu.Lock()", touched: true }],
    }} />);
    expect(shown.match(/ms-desk-line--touched/g)).toHaveLength(2);
    expect(shown).toContain('data-line="14"');
    expect(shown).toContain('data-ask-surface="desk"');
    expect(shown).toContain(`data-ask-revision="${TIP}"`);
    expect(shown).toContain("lines 13–15 of 33");
    expect(shown).not.toContain("touched lines not marked");
    // Where the touched lines could not be established, the read says so (Sol SOL-A-04).
    const unmarked = around(<SourceShown put={put} source={{
      path: "a.go", commit: TIP, from: 1, to: 1, total: 1, lines: [{ number: 1, text: "package a" }],
      unmarked: "touched lines not marked",
    }} />);
    expect(unmarked).toContain("touched lines not marked");
  });

  it("shows the change index with each file's counts, and a done goal's commits' own changes", () => {
    const shown = around(<ChangesShown put={put} since={false} changes={{
      goal: "g1-s50", comparisons: [{ from: "0".repeat(40), to: "1".repeat(40) }, { from: "9".repeat(40), to: "2".repeat(40) }],
      files: [{ path: "a.go", added: 3, deleted: 1 }, { path: "b.png", added: 0, deleted: 0, binary: true }],
      supplied: 2, total: 2, current: "", moved: false,
    }} />);
    expect(shown).toContain("a.go");
    expect(shown).toContain("+3");
    expect(shown).toContain("−1");
    expect(shown).toContain("binary");
    expect(shown).toContain("2 files");
  });

  it("shows a file's diff as numbered hunks", () => {
    const shown = around(<DiffShown put={put} since={false} diff={{
      path: "a.go", supplied: 2, total: 2,
      parts: [{ from: "b", to: "t", hunks: [{ header: "@@ -1 +1 @@", lines: [
        { kind: "deleted", old: 1, text: "old" }, { kind: "added", new: 1, text: "new" }] }] }],
    }} />);
    expect(shown).toContain("@@ -1 +1 @@");
    expect(shown).toContain("ms-desk-line--added");
    expect(shown).toContain("ms-desk-line--deleted");
  });

  it("makes every anchor in the conversation a press that puts it on the desk", () => {
    const shown = around(<Anchored words="taken in internal/owner/owner.go:14-27 and released" put={put} rest={(words) => words} />);
    expect(shown).toContain('<button type="button" class="ms-anchor-chip"');
    expect(shown).toContain(">internal/owner/owner.go:14-27<");
  });
});

describe("the finding card", () => {
  const card = (over: Partial<Card>): Card => ({
    kind: "finding", text: "", anchor: "internal/owner/owner.go:21-24", offered: true,
    subject: { kind: "record", id: RECORD, title: "" }, id: "local-1",
    mark: { text: "", clause: "internal/owner/owner.go:21-24", recording: false, recorded: "", refusal: "", dismissed: false },
    standing: "blocked", ...over,
  });

  it("opens with the anchor filled and the words left for the human's own", () => {
    const shown = around(<DepositCard id="local-1" />, { deposits: [card({})] });
    expect(shown).toContain("A finding");
    expect(shown).toContain('value="internal/owner/owner.go:21-24"');
    expect(shown).toContain("What you found, in your words");
    expect(shown).toContain(">Record it<");
  });

  it("is recorded unanswered, and then offers the four answers with their consequences", () => {
    const entries: readonly Entry[] = entriesIn(REVIEW_SOURCE);
    const shown = around(<DepositCard id="deposit:t1#0" />, {
      deposits: [card({ id: "deposit:t1#0", standing: "recorded",
        mark: { text: "a press that dies holds the lock", clause: "internal/owner/owner.go:21-24", recording: false,
          recorded: "Findings", refusal: "", dismissed: false } })],
      table: { counts: { Facts: 0, Proposals: 0, Decisions: 0, "Open questions": 0, Findings: 1 }, entries,
        revision: "r1", source: REVIEW_SOURCE },
    });
    expect(shown).toContain("unanswered");
    expect(shown).toContain("the candidate will return to construction and be examined again");
    expect(shown).toContain("Opens a new goal with this finding as its intent, and lets this one land.");
    expect(shown).toContain("Records the risk as accepted, with your reason");
    expect(shown).toContain("Keeps the question open, with its consequence");
  });
});

describe("End", () => {
  const entries = entriesIn(REVIEW_SOURCE);

  it("refuses Clear to land while a recorded finding is unanswered, and lists it", () => {
    const shown = around(<EndWays entries={entries} busy={false} onChoose={() => undefined} />);
    expect(shown).toMatch(/<button[^>]*disabled=""[^>]*>Clear to land</u);
    expect(shown).toContain("Clear to land waits until every finding is answered. These are not:");
    expect(shown).toContain("<li>a press that dies holds the lock</li>");
    expect(shown).toContain(">Send back<");
    expect(shown).toContain(">End without a verdict<");
    expect(shown).not.toContain(NOD_LINE);
  });

  it("says a nod plainly when the piles are empty", () => {
    const shown = around(<EndWays entries={[]} busy={false} onChoose={() => undefined} />);
    expect(shown).toContain(NOD_LINE);
    expect(shown).not.toMatch(/<button[^>]*disabled=""[^>]*>Clear to land</u);
  });
});
