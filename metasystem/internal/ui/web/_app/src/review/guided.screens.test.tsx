import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import type { ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Changes } from "./api";
import { ReviewView } from "./Guided";
import { Room } from "./ReviewRoom";
import type { Row } from "../backlog/api";
import { emptyStore } from "../partner/conversation";
import type { Deposit, Message } from "../partner/api";
import { cardsIn, entriesIn, FIX, lineOf, recordedIn } from "../partner/sitting";
import { PartnerAs } from "../partner/store";

/**
 * The guided review on screen (review-findings-read-as-decisions §3, the mocks
 * blocking, clean and older): one column, four numbered steps, the findings
 * as decisions, Try it folded, the verdict with the recommended way first,
 * and no internal word on screen outside the evidence fold.
 */

const RECORD = "plans/reviews/review-of-g1-s21.md";
const TIP = "a1".repeat(20);
const NOW = "c2".repeat(20);
const SOURCE = `# Review of g1-s21\n\n- Kind: review\n- Goals: g1-s21\n- Reviewed: ${TIP} (the tip of goal/g1-s21)\n\n## Facts\n\n## Findings\n\n## Outcome\n`;
const SUBJECT = { kind: "record", id: RECORD, title: "Review of g1-s21" };

/** The words the room never says outside the evidence fold. */
const BANNED = ["tip", "candidate", "anchor", "desk", "Dismiss", "unanswered"];

function finding(over: Partial<Deposit>): Deposit {
  return {
    kind: "finding", text: "nothing tries a press that dies at owner.go:21-24 after a1a1a1a", anchor: "internal/owner/owner.go:21-24",
    consequence: "the lock stays held", severity: "blocks", title: "A press that dies halfway leaves the ledger locked.",
    why: "The next press waits forever.", recommend: "must-fix", reason: "Release the lock on every way out.",
    subject: SUBJECT, offered: true, ...over,
  };
}

const THREE = [
  finding({}),
  finding({ severity: "fix", title: "Only the happy path is tested.", why: "A later change could break it unseen.",
    recommend: "fix-later", reason: "Open a follow-up goal to test each failure.", anchor: "internal/owner/owner_test.go:5-13" }),
  finding({ severity: "note", title: "The design still describes two locks.", why: "Nothing breaks.",
    recommend: "not-a-problem", reason: "The design is history.", anchor: "plans/designs/reading.md" }),
];

function opened(deposits: Deposit[], outcome: Message["outcome"] = "complete"): Message[] {
  return [
    { id: "m1", turn: "t0", role: "human", text: `Open this review of the work recorded in ${RECORD}.`, at: "", interface: true },
    { id: "m2", turn: "t0", role: "partner", at: "", outcome, deposits,
      text: "The goal: one lock across publish and reconcile. This version adds a test. It changes 2 files.\n\n## Asked\n\n..." },
  ];
}

function held(deposits: Deposit[], source = SOURCE, outcome: Message["outcome"] = "complete", live = "") {
  const messages = opened(deposits, outcome);
  return {
    conversation: RECORD,
    store: { ...emptyStore, conversation: RECORD, state: "ready" as const, messages, live: { ...emptyStore.live, turn: live } },
    sitting: { subject: SUBJECT, purpose: "review", startedAt: "" },
    table: { counts: { Facts: 0, Proposals: 0, Decisions: 0, "Open questions": 0, Findings: 0 }, entries: entriesIn(source),
      revision: "r1", source },
    deposits: cardsIn([{ turn: "t0", deposits }], {}, RECORD, recordedIn(source)),
    room: { desk: { items: [], current: -1 }, face: "desk" as const, drafts: {} },
  };
}

const CURRENT: Changes = {
  goal: "g1-s21", comparisons: [], files: [{ path: "internal/owner/owner.go", added: 3, deleted: 1 }], supplied: 1, total: 1,
  current: TIP, moved: false, at: "2026-10-02T12:19:37Z", by: "Wido Riezebos", currentAt: "2026-10-02T12:19:37Z",
};
const MOVED: Changes = { ...CURRENT, current: NOW, moved: true, currentAt: "2026-10-03T07:55:02Z" };
const SINCE: Changes = { ...MOVED, files: [{ path: "a.go", added: 1, deleted: 0 }, { path: "b.go", added: 1, deleted: 0 }] };

function row(): Row {
  return {
    ref: { kind: "goal", id: "g1-s21", revision: 1 }, where: "", lane: "review", phase: "", state: "claimed",
    intent: "What: One lock across publish and reconcile. And the rest.", nextStep: "", concluded: "", origin: "human",
    priority: 1, sequence: 1, tier: 2, labels: [], arc: "", pinned: "", blockedBy: [], holds: [], openBlockers: [],
    claim: { machine: "m1e", lineage: "steward-seat", at: "", landingAt: "" },
    sliced: false, decomposed: false, openedAt: "", doneAt: "", lastChangeAt: "", lastVerb: "", gaps: [],
  } as Row;
}

function around(node: ReactNode, state: Parameters<typeof PartnerAs>[0]["held"]): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <PartnerAs held={state}>{node}</PartnerAs>
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

/** What a person reads: the text outside tags, and outside the evidence fold, where ids and paths live. */
function read(markup: string): string {
  return markup
    .replace(/<details class="ms-layered-evidence">[\s\S]*?<\/details>/gu, " ")
    .replace(/<[^>]*>/gu, " ")
    .replace(/&#x27;/gu, "'")
    .replace(/&quot;/gu, '"')
    .replace(/&amp;/gu, "&")
    .replace(/\s+/gu, " ");
}

function banned(text: string): string[] {
  return BANNED.filter((word) => new RegExp(`\\b${word}\\b`, "iu").test(text));
}

describe("a review with a finding that blocks", () => {
  const shown = around(<ReviewView record={RECORD} changes={CURRENT} since={null} row={row()} />, held(THREE));
  const text = read(shown);

  it("says the goal by its own sentence and the version by its time and builder, the commit behind the (i)", () => {
    expect(text).toContain("Review of One lock across publish and reconcile.");
    expect(text).toMatch(/The version of 2 October, \d\d:19, built by m1e/u);
    expect(text).toContain("This is the version that would land.");
    expect(shown).toContain(`title="commit ${TIP}"`);
    expect(text).toContain("The goal: one lock across publish and reconcile. This version adds a test. It changes 2 files.");
  });

  it("reads as four numbered steps in one column", () => {
    for (const step of ["1 What you are looking at", "2 What the reviewer found", "3 Try it optional", "4 Your verdict"]) {
      expect(text).toContain(step);
    }
    expect(shown).not.toContain("ms-room-panes");
  });

  it("sums up before any card, and lays each card out in layers with its decision", () => {
    expect(text).toContain("3 findings. 1 blocks landing, 1 is worth fixing, 1 is a note.");
    expect(text).toContain("The reviewer recommends: send it back, because of the one that blocks.");
    expect(text).toContain("Blocks landing A press that dies halfway leaves the ledger locked. Why it matters. The next press waits forever.");
    expect(text).toContain("The reviewer recommends: must fix before landing. Release the lock on every way out.");
    expect(text).toContain("Must fix before landing recommended The builder gets this as a correction when you send the goal back; landing over it asks you first.");
    expect(text).toContain("I accept this risk Say why. Your acceptance and your reason are recorded on this review, in your name. " +
      "Accepting it on the goal itself is a separate step, at a terminal: metasystem goal accept-risk.");
    expect(shown).toContain("Evidence, as the reviewer wrote it");
    // A note offers fix after landing and not a problem, and nothing else.
    const note = text.slice(text.indexOf("The design still describes two locks."));
    expect(note.slice(0, note.indexOf("Evidence"))).not.toContain("Must fix before landing");
  });

  it("puts Send it back first, recommended, and says what landing over the block asks", () => {
    expect(text.indexOf("Send it back recommended")).toBeGreaterThan(0);
    expect(text.indexOf("Send it back recommended")).toBeLessThan(text.indexOf("Looks good, land it"));
    expect(text).toContain("One finding must be fixed before landing: you will be asked to accept that risk and say why");
  });

  it("says what each quiet way out does, and that ending does not land the goal (RF-07)", () => {
    expect(text).toContain("Leave for now : everything stays as it is, and the goal waits for you.");
    expect(text).toContain("End without a verdict : the sitting ends and your hold is released; the goal still waits for a decision to land");
    expect(text).not.toContain("lands under its own rules");
  });

  it("says no internal word outside the evidence fold, and no path: the question line names the review", () => {
    expect(banned(text)).toEqual([]);
    expect(text).toContain("Seeing: the review of g1-s21");
    expect(text).not.toContain(RECORD);
  });
});

describe("a clean review", () => {
  it("says nothing was raised once the look completed, and recommends landing in one press", () => {
    const text = read(around(<ReviewView record={RECORD} changes={CURRENT} since={null} row={row()} />, held([])));
    expect(text).toContain("The reviewer found nothing to raise.");
    expect(text).toContain("The reviewer recommends: land it.");
    expect(text.indexOf("Looks good, land it recommended")).toBeLessThan(text.indexOf("Send it back"));
    expect(text).toContain("Nothing else follows: no finding, no follow-up.");
    expect(text).toContain("Say what must change first.");
    expect(banned(text)).toEqual([]);
  });

  it("is not clean while a walk asked after the opening is still running (fix round 2, F-2)", () => {
    const state = held([]);
    const walking = { ...state, store: { ...state.store, messages: [...state.store.messages,
      { id: "w1", turn: "t5", role: "human" as const, text: "Walk me through Built for the review in x: what was built.", at: "", interface: true }],
      live: { ...state.store.live, turn: "t5" } } };
    const text = read(around(<ReviewView record={RECORD} changes={CURRENT} since={null} row={row()} />, walking));
    expect(text).not.toContain("found nothing to raise");
    expect(text).not.toContain("recommended");
    expect(text).toContain("The reviewer is still looking at this version.");
  });

  it("says a look still running, stopped or failed as such, and marks no way (RF-06)", () => {
    const reviewing = read(around(<ReviewView record={RECORD} changes={CURRENT} since={null} row={row()} />, held([], SOURCE, undefined, "t0")));
    expect(reviewing).toContain("The reviewer is still looking at this version.");
    expect(reviewing).not.toContain("found nothing to raise");
    expect(reviewing).not.toContain("recommended");
    expect(reviewing).toContain("The reviewer has not finished looking at this version, so landing now lands it without the reviewer's report.");
    const stopped = read(around(<ReviewView record={RECORD} changes={CURRENT} since={null} row={row()} />, held([], SOURCE, "stopped")));
    expect(stopped).toContain("The reviewer's look at this version was stopped before it finished.");
    expect(stopped).toContain("Ask the reviewer to look again");
  });
});

describe("a look that stopped after it raised findings (fix round 3, F-2)", () => {
  it("offers to ask the reviewer to look again, whatever the number of findings", () => {
    for (const outcome of ["stopped", "failed"] as const) {
      const shown = around(<ReviewView record={RECORD} changes={CURRENT} since={null} row={row()} />, held(THREE, SOURCE, outcome));
      expect(shown).toMatch(/<button[^>]*>Ask the reviewer to look again<\/button>/u);
      expect(read(shown)).toContain("The reviewer's look at this version did not finish, so this list may not be whole.");
    }
    const complete = around(<ReviewView record={RECORD} changes={CURRENT} since={null} row={row()} />, held(THREE));
    expect(complete).not.toContain("Ask the reviewer to look again");
  });
});

describe("a look that has not finished, with findings already raised (fix round 1, R-143-m1e)", () => {
  it("marks no way recommended, says the list may grow, and says what landing on it lets through", () => {
    const text = read(around(<ReviewView record={RECORD} changes={CURRENT} since={null} row={row()} />, held(THREE, SOURCE, undefined, "t0")));
    expect(text).toContain("3 findings. 1 blocks landing, 1 is worth fixing, 1 is a note.");
    expect(text).toContain("The reviewer has not finished looking at this version, so this list may still grow.");
    expect(text).not.toContain("Send it back recommended");
    expect(text).not.toContain("Looks good, land it recommended");
    expect(text).toContain("Landing now lets through whatever it has not looked at yet.");
  });
});

describe("a review of an older version than would land", () => {
  const shown = around(<ReviewView record={RECORD} changes={MOVED} since={SINCE} row={row()} />, held(THREE.slice(1)));
  const text = read(shown);

  it("says a newer version exists, and offers the one press that reviews it", () => {
    expect(text).toMatch(/A newer version exists, from 3 October, \d\d:55\./u);
    expect(text).toContain("This review does not cover the version that would land.");
    expect(text).toContain("2 files changed");
    expect(shown.match(/>Review the current version</gu)).toHaveLength(1);
  });

  it("shows what it found muted, in the older version, with no decision, and offers no verdict", () => {
    expect(text).toContain("2 What the reviewer found in the older version");
    expect(text).toContain("Decide them after the reviewer has looked at the current version.");
    expect(shown).toContain("ms-layered--muted");
    expect(text).not.toContain("Your decision");
    expect(text).toContain("No verdict yet.");
    expect(text).not.toContain("Looks good, land it");
    expect(text).toContain("End without a verdict");
    expect(banned(text)).toEqual([]);
  });

  it("keeps what an earlier version was found to have apart once the record moved on (RF-03)", () => {
    const moved = SOURCE.replace("## Outcome\n", `## Earlier findings\n\n${lineOf({
      when: "2026-10-02", who: "Wido", text: "An earlier finding.", clause: "", section: "Earlier findings", mark: "deposit:t9#0",
      severity: "fix", answer: FIX,
    }, "finding")}\n## Outcome\n`);
    const later = read(around(<ReviewView record={RECORD} changes={CURRENT} since={null} row={row()} />, held([], moved)));
    expect(later).toContain("About the older version: 1 finding, with your decisions then");
    expect(later).toContain("The reviewer found nothing to raise.");
  });
});

describe("a verdict on the goal", () => {
  it("is said in words with the way back, and the steps that led to it give way", () => {
    const text = read(around(<ReviewView record={RECORD} changes={CURRENT} since={null} row={row()} />,
      { ...held(THREE), verdictSaid: "Recorded on g1-s21 as your verdict: send it back." }));
    expect(text).toContain("Recorded on g1-s21 as your verdict: send it back. Back to the board");
    expect(text).not.toContain("What the reviewer found");
    expect(text).not.toContain("Looks good, land it");
  });
});

describe("the room", () => {
  it("is the guided review for a review sitting, and keeps the desk for a sitting that shapes a record", () => {
    expect(around(<Room record={RECORD} />, held([]))).toContain("ms-guided");
  });
});

describe("the review room's width", () => {
  it("is laid out for a desktop only: no phone-width rule shapes the guided review (fix round 2, Wido's correction)", () => {
    const css = readFileSync(fileURLToPath(new URL("./room.css", import.meta.url)), "utf8");
    const media = [...css.matchAll(/@media[^{]*\{([\s\S]*?)\n\}/gu)].map((one) => one[1]);
    expect(media.filter((block) => /\.ms-(guided|layered|decide)/u.test(block))).toEqual([]);
  });
});
