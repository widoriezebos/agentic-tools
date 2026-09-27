import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Need, Page, Proposed, Ruling } from "./api";
import { Views } from "./DecisionsPane";
import { noNarrowing, type Narrowing, type TabId } from "./decisions";
import type { Acts } from "./InboxRow";
import { lineOf } from "./proposals";
import type { Proposal } from "../partner/api";
import { cardsIn, NEEDS_ITS_BUDGET, WAS_IN_FLIGHT } from "../partner/proposing";
import { PartnerAs } from "../partner/store";
import type { Row } from "../backlog/api";

/**
 * What the page puts on the screen, over a payload and nothing else.
 *
 * The reading, the network and the sheets belong to the pane; these are the
 * two views, rendered as the browser would render them. Every assertion is a
 * sentence a human reads: the group's line, the row's one line of substance,
 * what an open row holds and what it offers, the register's own words.
 */

/**
 * The instant these renders read as now: the payload's own readAt, handed to
 * the views rather than taken from the wall, so that an age the page says —
 * "yesterday", "5 days", "review passed 6 days ago" — is asserted against the
 * day the fixtures were written and holds on any day the test runs.
 */
const now = new Date("2026-09-25T11:00:00Z");

/** A row of the ledger, as much of one as an open row reads. */
function row(over: Partial<Row> = {}): Row {
  return {
    ref: { kind: "goal", id: "g1-s40", revision: 3 },
    where: "live",
    lane: "to-do",
    phase: "",
    state: "queued",
    intent: "The pane reads a document as a chapter",
    nextStep: "",
    concluded: "",
    origin: "human",
    priority: 2,
    sequence: 1,
    tier: 3,
    labels: [],
    arc: "",
    pinned: "",
    blockedBy: [],
    openBlockers: [],
    holds: [],
    sliced: false,
    decomposed: false,
    openedAt: "2026-09-20T09:00:00Z",
    doneAt: "",
    lastChangeAt: "",
    lastVerb: "",
    gaps: [],
    ...over,
  } as Row;
}

function need(over: Partial<Need> = {}): Need {
  return {
    kind: "approval",
    id: "g1-s40",
    title: "The pane reads a document as a chapter",
    asked: "Approve The pane reads a document as a chapter for execution",
    by: "the backlog",
    since: "2026-09-20T09:00:00Z",
    deadline: "",
    silence: "it stays in To Do and no seat may claim it",
    recommend: "",
    where: { kind: "goal", id: "g1-s40" },
    act: "approve",
    command: "",
    row: row(),
    new: false,
    words: "",
    context: "",
    owner: "",
    class: "",
    due: "",
    path: "",
    goals: [],
    proposal: null,
    ...over,
  };
}

function ruling(over: Partial<Ruling> = {}): Ruling {
  return {
    id: "R-3",
    date: "2026-09-05",
    words: "The board's two drag moves are the only acts the browser publishes, and g1-s12 is where they land",
    context: "Given with the board design",
    owner: "Wido",
    class: "temporary",
    due: "2026-09-19",
    event: "",
    condition: "class=temporary due=2026-09-19",
    duePassed: true,
    mentions: [
      { id: "g1-s12", where: { kind: "goal", id: "g1-s12" } },
      { id: "dec-1", where: { kind: "record", id: "docs/decisions/roster.md" } },
    ],
    ...over,
  };
}

/** Every kind of thing that can wait on a human, one of each. */
const everyKind: Need[] = [
  need({
    kind: "renewal", id: "g1-s42", title: "The approval on this one expired",
    asked: "Renew the approval of g1-s42: the review date 2026-09-06 has passed",
    silence: "no fresh claim is admitted",
  }),
  need({
    kind: "renewal", id: "g1-s43", title: "Claimed work whose approval expired",
    asked: "Renew the approval of g1-s43: the review date 2026-09-07 has passed",
    silence: "work already claimed continues; renew at the goal", act: "", row: null,
  }),
  need({
    kind: "ruling-review", id: "R-3", title: "R-3",
    asked: "Review R-3, due 2026-09-19: adopt, revise or withdraw",
    by: "Wido", silence: "it stays in force as written", act: "", row: null,
    where: { kind: "register", id: "memory/rulings.md" },
    words: "The board's two drag moves are the only acts the browser publishes. The rest of the ruling.",
    context: "Given with the board design", owner: "Wido", class: "temporary", due: "2026-09-19",
  }),
  need(),
  need({
    kind: "ask", id: "q-1", title: "g1-s21 · budget-above-norm",
    asked: "one more review round and two more hours · the second attempt reached the elapsed limit",
    by: "seat m1e", silence: "no recorded consequence",
    recommend: "raise it once; what is left is small and well understood",
    act: "", row: null, where: { kind: "channel", id: "q-1" },
  }),
  need({
    kind: "question", id: "Q-1", title: "Which census format?", asked: "Which census format?",
    by: "the register", since: "2026-09-24", silence: "it stays open", act: "", row: null,
    new: true, where: { kind: "question", id: "Q-1" },
  }),
  need({
    kind: "parked", id: "g1-s45", title: "The Fleet section reads the census",
    asked: "Unpark The Fleet section reads the census? parked by m2a+implementer 2026-09-24T05:00:00Z: the implementer paused it to finish g1-s48 first",
    by: "m2a+implementer", silence: "it stays parked", act: "unpark", row: null,
    where: { kind: "goal", id: "g1-s45" },
  }),
  need({
    kind: "stopped", id: "g1-s48", title: "Work a breach fence stopped",
    asked: "Resume Work a breach fence stopped, stopped 2026-09-25T07:00:00Z: the attempt limit was reached",
    by: "the engine", silence: "it stays stopped; its claim keeps the goal", act: "", row: null,
    command: "metasystem goal resume --id g1-s48",
    where: { kind: "goal", id: "g1-s48" },
  }),
  need({
    kind: "draft", id: "d-open", title: "A draft nobody accepted",
    asked: "Accept the draft A draft nobody accepted?", by: "design",
    silence: "it stays a draft, shown as one on Project", act: "", row: null,
    where: { kind: "record", id: "plans/designs/draft.md" },
    path: "plans/designs/draft.md",
  }),
  need({
    kind: "landed", id: "d-landed", title: "Every goal of this one landed",
    asked: "Mark Every goal of this one landed done?", by: "every goal landed",
    silence: "it stays marked accepted", act: "", row: null,
    where: { kind: "record", id: "plans/designs/landed.md" },
    path: "plans/designs/landed.md",
    goals: [{ id: "g1-s9", state: "done" }, { id: "g1-s10", state: "done" }],
  }),
  need({
    kind: "alert", id: "n-1", title: "HEALTH unhealthy — the steward runner is stale",
    asked: "HEALTH unhealthy — the steward runner is stale", by: "alert",
    silence: "no recorded consequence", act: "", row: null,
    where: { kind: "notifications", id: "n-1" },
  }),
];

function page(over: Partial<Page> = {}): Page {
  return {
    schemaVersion: 3,
    readAt: "2026-09-25T11:00:00Z",
    signIn: false,
    needsYou: everyKind,
    decided: {
      rulings: [ruling()],
      defects: ["R-9: review condition needs due= or event="],
      decisions: [
        { id: "dec-1", title: "The roster for design work", note: "accepted", at: "2026-09-20T09:00:00Z", where: { kind: "record", id: "docs/decisions/roster.md" } },
      ],
      answered: [
        { id: "Q-2", title: "Who owns the sweep?", note: "R-124", at: "2026-09-17T09:00:00Z", where: { kind: "question", id: "Q-2" } },
      ],
      approved: [
        { id: "g1-s49", title: "Approved and ready to claim", by: "human:Wido", at: "2026-09-25T09:00:00Z", authority: "proven", expired: false, row: row({ ref: { kind: "goal", id: "g1-s49", revision: 4 }, lane: "ready", state: "approved" }) },
        { id: "g1-s9", title: "The shell, the rail and the header", by: "human:Wido", at: "2026-08-20T09:00:00Z", authority: "proven", expired: false, row: row({ ref: { kind: "goal", id: "g1-s9", revision: 9 }, lane: "done", state: "done", where: "archived" }) },
      ],
      notNow: [
        {
          id: "g1-s34", title: "The queue narrows by label and by origin", by: "human:Wido",
          at: "2026-09-13T08:23:22Z", because: "not before the board's own filters settle",
          blocker: "", where: { kind: "goal", id: "g1-s34" },
        },
        {
          id: "g1-s36", title: "The register opens from the review card", by: "human:Wido",
          at: "2026-09-11T08:23:22Z", because: "waiting for g1-s24; the census format decides the path",
          blocker: "g1-s24", where: { kind: "goal", id: "g1-s36" },
        },
      ],
    },
    counts: {
      needsYou: everyKind.length,
      asked: everyKind.filter((one) => one.kind !== "approval").length,
      waiting: everyKind.filter((one) => one.kind === "approval").length,
      rulings: 148,
    },
    visit: { since: "2026-09-24T11:00:00Z", first: false },
    register: "metasystem/memory/rulings.md",
    ...over,
  };
}

/** Nothing acts in a static render; these are the handles the row is given. */
const acts: Acts = {
  signedIn: true,
  onSignIn: () => undefined,
  onApprove: () => undefined,
  onPark: () => undefined,
  onEdit: () => undefined,
  onReturn: () => undefined,
  onWrite: () => undefined,
  writing: "",
  refusedAt: "",
  refusal: "",
  // A proposal row's line is composed from what the page holds about it, so the
  // static render composes one the way the page does, over nothing held.
  proposals: {
    lineOf: (need) => lineOf(need),
    onApply: () => undefined,
    onDismiss: () => undefined,
    onAsk: () => undefined,
    onBulkApply: () => undefined,
    onBulkDismiss: () => undefined,
    onContinue: () => undefined,
    running: false,
  },
};

type Shown = {
  view?: "inbox" | "decided";
  tab?: TabId;
  chosen?: string | null;
  openRow?: string;
  narrowing?: Narrowing;
  selected?: readonly string[];
  signedIn?: boolean;
};

function rendered(payload: Page, shown: Shown = {}, handles: Acts = acts): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <Views
          page={payload}
          acts={handles}
          view={shown.view ?? "inbox"}
          tab={shown.tab ?? "rulings"}
          chosen={shown.chosen === undefined ? "" : shown.chosen}
          openRow={shown.openRow ?? ""}
          narrowing={shown.narrowing ?? noNarrowing}
          selected={shown.selected ?? []}
          signedIn={shown.signedIn}
          now={now}
        />
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

describe("the page at rest", () => {
  // The whole answer to "is anything waiting on me, and how much" in ten
  // lines. Nothing is expanded, nothing is a card, and the queue's hundred
  // rows are one line until a human asks for them.
  it("is the group lines and nothing else", () => {
    const markup = rendered(page());

    for (const title of [
      "Questions", "Drafts to accept", "Designs landed", "Asks from a seat", "Alerts",
      "Approvals to renew", "Stopped goals", "Parked by a seat", "Rulings past review",
      "Goals waiting for approval",
    ]) {
      expect(markup).toContain(`>${title}</span>`);
    }
    // No row is rendered at all: not one line of substance, not one act.
    expect(markup).not.toContain("Which census format?");
    expect(markup).not.toContain("The pane reads a document as a chapter");
    expect(markup).not.toContain(">Approve<");
    expect(markup).not.toContain("Find in the id, the intent and the labels");
    expect(markup).toContain('aria-expanded="false"');
    expect(markup).not.toContain('aria-expanded="true"');
  });

  it("counts the two views in the header, each with what is behind it", () => {
    const markup = rendered(page());

    expect(markup).toContain(`Inbox ${String(everyKind.length)}`);
    // The five tabs of what was decided: one ruling, one decision, one
    // answered question, two approvals and two parks.
    expect(markup).toContain("Decided 7");
  });

  it("says how many of a group are new since the last visit, and nothing where none is", () => {
    const markup = rendered(page());

    expect(markup).toContain(">1 new</span>");
    // Ten groups and one new row between them, so the phrase appears once.
    expect(markup.match(/ new<\/span>/g)).toHaveLength(1);
  });

  it("gives the quiet group its standing sentence where every other says an age", () => {
    const markup = rendered(page());

    expect(markup).toContain("they stay in force");
    expect(markup).toContain(">yesterday</span>");
  });

  it("leaves an empty group off the page and says so when nothing is waiting at all", () => {
    const markup = rendered(page({ needsYou: [], counts: { needsYou: 0, asked: 0, waiting: 0, rulings: 148 } }));

    expect(markup).not.toContain("Questions");
    expect(markup).toContain("Nothing is waiting on you.");
  });

  it("puts signing in at the top when nothing proves a human", () => {
    expect(rendered(page({ signIn: true }))).toContain("Nothing proves a human on this seat yet.");
    expect(rendered(page())).not.toContain("Nothing proves a human on this seat yet.");
  });
});

describe("one group open", () => {
  it("shows that group's rows and no other group's", () => {
    const markup = rendered(page(), { chosen: "questions" });

    expect(markup).toContain('aria-expanded="true"');
    expect(markup).toContain("Which census format?");
    expect(markup).not.toContain("A draft nobody accepted");
    expect(markup.match(/aria-expanded="true"/g)).toHaveLength(1);
  });

  it("opens what this viewer left open, and falls back where that group has gone", () => {
    expect(rendered(page(), { chosen: "drafts" })).toContain("A draft nobody accepted");
    // The draft was accepted between two reads, so the group it was left open
    // on is gone and the first group with something new opens instead.
    const accepted = page({ needsYou: everyKind.filter((one) => one.kind !== "draft") });
    expect(rendered(accepted, { chosen: "drafts" })).toContain("Which census format?");
  });

  it("opens the first group with something new where nothing is remembered", () => {
    expect(rendered(page(), { chosen: null })).toContain("Which census format?");
  });
});

describe("a row's one line", () => {
  it("is the thing itself, with no id and no button on it", () => {
    const markup = rendered(page(), { chosen: "reviews" });

    // The ruling's own first sentence, not "Review R-3, due …".
    expect(markup).toContain("The board&#x27;s two drag moves are the only acts the browser publishes");
    expect(markup).not.toContain("adopt, revise or withdraw");
    expect(markup).not.toContain("<button type=\"button\" class=\"ms-button\"");
  });

  it("carries the goal's facts at its muted end and a dot where it is new", () => {
    const markup = rendered(page(), { chosen: "queue" });

    expect(markup).toContain(">yours<");
    expect(markup).toContain(">tier 3<");
    expect(markup).toContain("5 days");
    // Nothing in the queue is new here, so no dot is drawn in it.
    expect(rendered(page(), { chosen: "questions" })).toContain("ms-decisions-dot");
  });
});

describe("an open row", () => {
  it("names the row's id, small, where a human who needs it looks for it", () => {
    expect(rendered(page(), { chosen: "questions", openRow: "Q-1" })).toContain(">Q-1</p>");
  });

  it("holds a question and the register, which is where it is answered", () => {
    const markup = rendered(page(), { chosen: "questions", openRow: "Q-1" });

    expect(markup).toContain("Which census format?");
    expect(markup).toContain("if you do nothing: it stays open");
    expect(markup).toContain("Open the register");
    // There is no one-click settle: the route takes an answering record's
    // reference or a withdrawal, and a bare "settled" would misstate what
    // happened.
    expect(markup).not.toContain("Settle");
  });

  it("holds a draft, its path, and the act that accepts it", () => {
    const markup = rendered(page(), { chosen: "drafts", openRow: "d-open" });

    expect(markup).toContain("plans/designs/draft.md");
    expect(markup).toContain(">Accept</button>");
    expect(markup).toContain("Open the record");
  });

  it("holds a landed design's goals with their states, and the act that closes it", () => {
    const markup = rendered(page(), { chosen: "landed", openRow: "d-landed" });

    expect(markup).toContain("g1-s9 · done");
    expect(markup).toContain("g1-s10 · done");
    expect(markup).toContain(">Mark done</button>");
    expect(markup).toContain("Open the record");
  });

  it("holds a ruling's words, its context behind a disclosure, and its schedule", () => {
    const markup = rendered(page(), { chosen: "reviews", openRow: "R-3" });

    expect(markup).toContain("The rest of the ruling.");
    expect(markup).toContain("<summary");
    expect(markup).toContain("Given with the board design");
    expect(markup).toContain("owner Wido");
    expect(markup).toContain("review due 2026-09-19");
    expect(markup).toContain("Open the register");
  });

  it("holds a queued goal whole, with its four acts", () => {
    const markup = rendered(page(), { chosen: "queue", openRow: "g1-s40" });

    expect(markup).toContain("The pane reads a document as a chapter");
    expect(markup).toContain("no budget recorded");
    expect(markup).toContain("priority 2, position 1");
    expect(markup).toContain(">Approve</button>");
    expect(markup).toContain(">Not now</button>");
    expect(markup).toContain(">Edit</button>");
    expect(markup).toContain("Open the goal");
  });

  it("holds a seat's park with the way back, and a stopped goal with its command", () => {
    const parked = rendered(page(), { chosen: "parked", openRow: "g1-s45" });
    const stopped = rendered(page(), { chosen: "stopped", openRow: "g1-s48" });

    expect(parked).toContain("the implementer paused it to finish g1-s48 first");
    expect(parked).toContain(">Return to queue</button>");
    expect(stopped).toContain("metasystem goal resume --id g1-s48");
    expect(stopped).not.toContain("metasystem goal unpark");
  });

  it("offers a renewal the sheet only where the engine would take one", () => {
    const unclaimed = rendered(page(), { chosen: "renewals", openRow: "g1-s42" });
    const claimed = rendered(page(), { chosen: "renewals", openRow: "g1-s43" });

    expect(unclaimed).toContain(">Approve</button>");
    expect(claimed).not.toContain(">Approve</button>");
    expect(claimed).toContain("work already claimed continues; renew at the goal");
  });

  it("holds an alert and a seat's ask with the places they are answered", () => {
    const alert = rendered(page(), { chosen: "alerts", openRow: "n-1" });
    const ask = rendered(page(), { chosen: "asks", openRow: "q-1" });

    expect(alert).toContain("Open the message");
    expect(ask).toContain("Answer on the fleet channel with your code");
    expect(ask).toContain("Recommended: raise it once; what is left is small and well understood");
  });

  it("carries the asker's recommendation only where the record has one", () => {
    expect(rendered(page(), { chosen: "questions", openRow: "Q-1" })).not.toContain("Recommended:");
  });

  it("says Sign in to act in place of the acts when nothing proves a human", () => {
    const markup = rendered(page({ signIn: true }), { chosen: "queue", openRow: "g1-s40", signedIn: false });

    expect(markup).toContain(">Sign in to act</button>");
    expect(markup).not.toContain(">Approve</button>");
    // The way through is not an act and stays: reading a goal needs no proof.
    expect(markup).toContain("Open the goal");
  });

  it("shows a refusal under the row it came from, and nowhere else", () => {
    const refused: Acts = { ...acts, refusedAt: "g1-s45", refusal: "the goal is not parked" };
    const markup = renderToStaticMarkup(
      <MemoryRouter>
        <TooltipPrimitive.Provider>
          <Views page={page()} acts={refused} chosen="parked" openRow="g1-s45" now={now} />
        </TooltipPrimitive.Provider>
      </MemoryRouter>,
    );

    expect(markup).toContain("the goal is not parked");
  });
});

describe("the queue", () => {
  const queued = [
    need({ id: "g1-s40", title: "The queue narrows by label", row: row({ ref: { kind: "goal", id: "g1-s40", revision: 3 }, intent: "The queue narrows by label and by origin", labels: ["browser-interface"], origin: "human", tier: 2, openedAt: "2026-09-22T00:00:00Z" }) }),
    need({ id: "g1-s41", title: "The fleet page reads a chain", row: row({ ref: { kind: "goal", id: "g1-s41", revision: 3 }, intent: "The fleet page reads a seat's whole chain", labels: ["headless-fleet"], origin: "main", tier: 0, openedAt: "2026-09-24T00:00:00Z" }) }),
  ];
  const withQueue = (over: Partial<Page> = {}) =>
    page({ needsYou: queued, counts: { needsYou: 2, asked: 0, waiting: 2, rulings: 148 }, ...over });

  it("puts its three tools on one line in the group's head", () => {
    const markup = rendered(withQueue(), { chosen: "queue" });

    expect(markup).toContain("Find in the id, the intent and the labels");
    expect(markup).toContain("Backlog order");
    expect(markup).toContain(">yours</button>");
    expect(markup).toContain("seats&#x27;</button>");
    expect(markup).toContain("browser-interface 1");
    expect(markup).toContain("2 waiting");
  });

  it("shows no selection bar until something is ticked", () => {
    expect(rendered(withQueue(), { chosen: "queue" })).not.toContain("selected</span>");
  });

  it("puts the bar at the group's foot, with the two sheets and a way to clear", () => {
    const markup = rendered(withQueue(), { chosen: "queue", selected: ["g1-s40"] });

    expect(markup).toContain("1 selected");
    expect(markup).toContain(">Approve</button>");
    expect(markup).toContain(">Not now</button>");
    expect(markup).toContain(">Clear</button>");
  });

  // A tick on a row the narrowing then hid is a tick on a row a human can no
  // longer see. Acting on it would be the one thing a queue must never do.
  it("acts only on rows that are ticked AND on screen", () => {
    const markup = rendered(withQueue(), {
      chosen: "queue",
      selected: ["g1-s40", "g1-s41"],
      narrowing: { ...noNarrowing, label: "headless-fleet" },
    });

    expect(markup).toContain("1 selected");
    expect(markup).toContain("2 waiting · 1 shown");
    expect(markup).not.toContain("The queue narrows by label<");
  });

  it("says so when the tools leave nothing on screen", () => {
    const markup = rendered(withQueue(), {
      chosen: "queue",
      narrowing: { ...noNarrowing, find: "nothing says this" },
    });

    expect(markup).toContain("No goal in the queue matches.");
    expect(markup).toContain("2 waiting · 0 shown");
  });
});

describe("what you decided", () => {
  const decided = (tab: TabId = "rulings") => rendered(page(), { view: "decided", tab });

  it("names the five tabs with their counts, the register's own count among them", () => {
    const markup = decided();

    expect(markup).toContain("Rulings 148");
    expect(markup).toContain("Decisions 1");
    expect(markup).toContain("Answered 1");
    expect(markup).toContain("Approved 2");
    expect(markup).toContain("Not now 2");
  });

  it("renders a ruling whole, with its owner, its review chip and its mentions", () => {
    const markup = decided();

    expect(markup).toContain("two drag moves are the only acts the browser publishes, and g1-s12 is where they land");
    expect(markup).not.toContain("…");
    expect(markup).toContain("owner Wido");
    expect(markup).toContain("review passed 6 days ago");
    expect(markup).toContain("/backlog/goal/g1-s12");
    expect(markup).toContain("Given with the board design");
    // A record it names opens the record, at the path the reader opens rather
    // than at the id the words wrote.
    expect(markup).toContain(">dec-1<");
    expect(markup).toContain("/project/doc/docs/decisions/roster.md");
  });

  it("lists the register's broken rows rather than hiding them", () => {
    expect(decided()).toContain("1 row of the register could not be read");
    expect(decided()).toContain("R-9: review condition needs due= or event=");
  });

  it("opens each of the other four tabs on its own rows", () => {
    expect(decided("decisions")).toContain("The roster for design work");
    expect(decided("answered")).toContain("Who owns the sweep?");
    expect(decided("approved")).toContain("Approved and ready to claim");
    expect(decided("not-now")).toContain("not before the board&#x27;s own filters settle");
    expect(decided("not-now")).toContain("waits for g1-s24");
  });

  it("carries Withdraw only where the board's own eligibility allows", () => {
    const markup = decided("approved");

    expect(markup.match(/>Withdraw approval</g)).toHaveLength(1);
    expect(markup).toContain("The shell, the rail and the header");
  });

  it("says what each empty tab is empty of", () => {
    const bare = page({
      decided: { rulings: [], defects: [], decisions: [], answered: [], approved: [], notNow: [] },
      counts: { needsYou: 0, asked: 0, waiting: 0, rulings: 0 },
    });
    const view = (tab: TabId) => rendered(bare, { view: "decided", tab });

    expect(view("decisions")).toContain("This project has recorded no decisions yet.");
    expect(view("answered")).toContain("No question of the register has been answered yet.");
    expect(view("approved")).toContain("No goal carries an approval yet.");
    expect(view("rulings")).toContain("No ruling of the register matches.");
    expect(view("not-now")).toContain("You have paused nothing.");
  });
});

describe("the phone", () => {
  // The page has one column at every width, and the groups stack through the
  // stylesheet rather than through a second markup: nothing here renders
  // differently at 400px, which is what keeps the two widths one page.
  it("renders one markup, which the stylesheet stacks", () => {
    expect(rendered(page())).toBe(rendered(page()));
    expect(rendered(page())).toContain("ms-decisions-group-line");
  });
});

describe("where the two record writes go", () => {
  /**
   * Accepting a draft and marking a landed design done are writes to the
   * record status route, and this page does not open a second way to it: it
   * calls the Project section's own writer, which is the one call site the cut
   * guard counts for that resource. A page that wrote its own request would
   * pass every assertion above and fail the cut.
   */
  it("is the Project section's own status writer, and no request of this page's own", () => {
    const here = path.resolve(fileURLToPath(import.meta.url), "..");
    const pane = readFileSync(path.join(here, "DecisionsPane.tsx"), "utf8");

    expect(pane).toContain('import { setRecordStatus } from "../project/api";');
    expect(pane).toContain("setRecordStatus(need.id, status)");
  });
});

/**
 * The group the Partner's proposals wait in.
 *
 * It is first of all the groups: one press each, the whole of what would happen
 * is on the row, and it is what this human asked the Partner for. What an open
 * row holds is the card's own line — the verb's word, the subject, every argument
 * and the explanation in the Partner's voice — with where the line stands under
 * it and the four acts beside it (g1-s60 D3).
 */
describe("proposed by the Partner", () => {
  const because = "superseded by the seat inventory (g1-s42)";

  function proposed(over: Partial<Need> = {}, action: Partial<Proposed> = {}): Need {
    return need({
      kind: "proposal",
      id: "t7/0",
      title: "The seat census answers which machines are alive",
      asked: `Not now · The seat census answers which machines are alive · Because: ${because}`,
      by: "the Partner",
      since: "2026-09-25T09:00:00Z",
      silence: "it stays proposed; nothing is applied",
      act: "apply",
      row: null,
      new: true,
      where: { kind: "goal", id: "g1-s44" },
      ...over,
      proposal: {
        turn: "t7", index: 0, verb: "park-goal", fields: { because },
        read: null, explanation: "the inventory covers what these were for",
        state: "waiting", words: "", version: 1,
        ...action,
      },
    });
  }

  const waitingRow = proposed();
  const refusedRow = proposed(
    { id: "t7/1", title: "A machine publishes its phase with every tick", where: { kind: "goal", id: "g1-s45" } },
    { index: 1, state: "refused", words: "goal g1-s45 is claimed by m2a", version: 3 },
  );
  const inFlightRow = proposed(
    { id: "t7/2", title: "The fleet page reads a seat's whole chain", where: { kind: "goal", id: "g1-s46" } },
    { index: 2, state: "applying", version: 2 },
  );
  const approveRow = proposed(
    {
      id: "t8/0",
      title: "A stopped seat says why it stopped",
      asked: "Approve · A stopped seat says why it stopped",
      where: { kind: "goal", id: "g1-s47" },
    },
    {
      turn: "t8", verb: "approve-goal", fields: {},
      read: { intent: "A stopped seat says why it stopped.", nextStep: "Read the fence.", tier: 3, labels: [] },
      explanation: "the fence work it names has landed",
    },
  );

  const budget = {
    elapsedLimit: "8h", attemptLimit: 10, reservedJobMinutesLimit: 1200,
    activeJobLimit: 1, reviewRoundLimit: 3,
  };

  /** The page with the four proposal rows first, and every other kind behind. */
  function proposing(): Page {
    const rows = [waitingRow, refusedRow, inFlightRow, approveRow];
    return page({
      needsYou: [...rows, ...everyKind],
      counts: {
        needsYou: rows.length + everyKind.length,
        asked: rows.length + everyKind.filter((one) => one.kind !== "approval").length,
        waiting: everyKind.filter((one) => one.kind === "approval").length,
        rulings: 148,
      },
    });
  }

  /** The handles with the tuple an approve row read when it opened. */
  function withBudget(): Acts {
    return {
      ...acts,
      proposals: {
        ...acts.proposals,
        lineOf: (one: Need) => lineOf(one, {}, { "t8#0": { budget, source: "project" } }),
      },
    };
  }

  it("is the first group, with its count and how many are new", () => {
    const markup = rendered(proposing());

    expect(markup).toContain(">Proposed by the Partner</span>");
    expect(markup.indexOf(">Proposed by the Partner</span>")).toBeLessThan(markup.indexOf(">Questions</span>"));
    expect(markup).toContain('<span class="ms-decisions-group-count">4</span>');
    expect(markup).toContain('<span class="ms-decisions-group-new">4 new</span>');
  });

  it("says the act and the subject on every row, and where a line stands", () => {
    const markup = rendered(proposing(), { chosen: "proposed" });

    expect(markup).toContain("Not now · The seat census answers which machines are alive</span>");
    expect(markup).toContain(
      "Not now · A machine publishes its phase with every tick · refused: goal g1-s45 is claimed by m2a</span>",
    );
    // The apostrophe in the title is escaped in the markup, so the assertion
    // reads from the state's own words back.
    expect(markup).toContain(`whole chain · ${WAS_IN_FLIGHT}</span>`);
    expect(markup).toContain("Approve · A stopped seat says why it stopped</span>");
  });

  it("holds the card's own line whole when a row opens, with the explanation under it", () => {
    const markup = rendered(proposing(), { chosen: "proposed", openRow: "t7/0" });

    expect(markup).toContain('<span class="ms-proposal-word">Not now</span>');
    expect(markup).toContain('<span class="ms-proposal-title">The seat census answers which machines are alive</span>');
    expect(markup).toContain(">g1-s44</a>");
    expect(markup).toContain('<span class="ms-proposal-label">Reason</span>');
    expect(markup).toContain(because);
    expect(markup).toContain("The Partner: the inventory covers what these were for");
    // What silence does, in the row's own muted line.
    expect(markup).toContain("asked by the Partner, today; if you do nothing: it stays proposed; nothing is applied");
  });

  it("shows an approve row the budget it read when the row opened, with its source", () => {
    const markup = rendered(proposing(), { chosen: "proposed", openRow: "t8/0" }, withBudget());

    expect(markup).toContain('<span class="ms-proposal-label">Budget</span>');
    expect(markup).toContain("8h elapsed · 10 attempts · 1200 reserved job-minutes · 1 active jobs · 3 review rounds");
    expect(markup).toContain("budget law for this goal");
    // And the approval's own reviewed substance, as the card shows it.
    expect(markup).toContain("A stopped seat says why it stopped.");
    expect(markup).toContain("Read the fence.");
  });

  it("says needs its budget first where no budget could be read at all", () => {
    const markup = rendered(proposing(), { chosen: "proposed", openRow: "t8/0" });

    expect(markup).toContain(NEEDS_ITS_BUDGET);
  });

  it("offers Apply, Dismiss, Ask the Partner and the way to the goal", () => {
    const markup = rendered(proposing(), { chosen: "proposed", openRow: "t7/0" });

    expect(markup).toContain(">Apply</button>");
    expect(markup).toContain(">Dismiss</button>");
    expect(markup).toContain(">Ask the Partner</button>");
    expect(markup).toContain(">Open the goal</a>");
  });

  it("says Try again on a refused row and on one a page left in flight", () => {
    const refused = rendered(proposing(), { chosen: "proposed", openRow: "t7/1" });
    const inFlight = rendered(proposing(), { chosen: "proposed", openRow: "t7/2" });

    for (const markup of [refused, inFlight]) {
      expect(markup).toContain(">Try again</button>");
      expect(markup).not.toContain(">Apply</button>");
      expect(markup).toContain(">Dismiss</button>");
      expect(markup).toContain(">Open the goal</a>");
    }
    expect(refused).toContain("refused: goal g1-s45 is claimed by m2a");
    expect(inFlight).toContain(WAS_IN_FLIGHT);
  });

  it("offers no press at all on a line whose act landed and could not be recorded", () => {
    // The record still says `applying`, because the write that would have said
    // otherwise failed; the ACT happened. A row that offered Try again on it
    // would be offering to approve the same goal twice (Sol S60-C-01).
    const landed = proposed(
      { id: "t7/3" },
      { index: 3, state: "applying", version: 2 },
    );
    const handles: Acts = {
      ...acts,
      proposals: {
        ...acts.proposals,
        lineOf: (one: Need) =>
          lineOf(one, {
            "t7#3": { ticked: true, notRun: false, refusedUnsent: "", unrecorded: { state: "applied", words: "" } },
          }),
      },
    };

    const markup = rendered(
      page({ needsYou: [landed, ...everyKind] }),
      { chosen: "proposed", openRow: "t7/3" },
      handles,
    );

    expect(markup).toContain("applied; the conversation could not record this");
    expect(markup).not.toContain(">Try again</button>");
    expect(markup).not.toContain(">Apply</button>");
    // Putting it away publishes nothing, so that press stays.
    expect(markup).toContain(">Dismiss</button>");
    expect(markup).toContain(">Ask the Partner</button>");
  });

  it("offers signing in instead, where nothing proves a human", () => {
    const markup = rendered(proposing(), { chosen: "proposed", openRow: "t7/0", signedIn: false });

    expect(markup).toContain(">Sign in to act</button>");
    expect(markup).not.toContain(">Apply</button>");
    expect(markup).not.toContain(">Dismiss</button>");
    // The way through is not an act and stays: reading the goal needs no proof.
    expect(markup).toContain(">Open the goal</a>");
  });
});

/**
 * What the Partner proposed about a goal, on that goal's queue row.
 *
 * The queue is where a goal is triaged before it is opened, so a proposal
 * waiting on one belongs on its line — beside the row's own toggle and not
 * inside it, because it is a second control doing a second thing: the toggle
 * opens the row and the chip opens the conversation at the line (g1-s61 D2, D3).
 */
describe("the chip on a queue row", () => {
  function proposal(over: Partial<Proposal> = {}): Proposal {
    return {
      index: 0,
      verb: "park-goal",
      goal: "g1-s40",
      title: "The pane reads a document as a chapter",
      fields: { because: "superseded by the seat inventory (g1-s42)" },
      read: null,
      why: "the inventory covers what this was for",
      offered: true,
      reason: "",
      state: "waiting",
      words: "",
      at: "2026-09-26T09:00:00Z",
      version: 1,
      ...over,
    };
  }

  /** The inbox as this human's conversation would have it beside them. */
  function queue(proposals: readonly Proposal[], shown: Shown = { chosen: "queue" }): string {
    return renderToStaticMarkup(
      <MemoryRouter>
        <TooltipPrimitive.Provider>
          <PartnerAs held={{ proposals: cardsIn([{ turn: "t1", proposals }], {}, {}, []) }}>
            <Views
              page={page()}
              acts={acts}
              view="inbox"
              tab="rulings"
              chosen={shown.chosen ?? ""}
              openRow={shown.openRow ?? ""}
              narrowing={noNarrowing}
              selected={[]}
              now={now}
            />
          </PartnerAs>
        </TooltipPrimitive.Provider>
      </MemoryRouter>,
    );
  }

  it("stands on the line, outside the toggle the line opens with", () => {
    const markup = queue([proposal()]);

    expect(markup).toContain('aria-label="Not now proposed on g1-s40, 1 action"');
    // Outside: the toggle closes before the chip opens, so one button is not
    // inside the other — which no browser would render and no keyboard reach.
    const chip = markup.indexOf("ms-chip-proposed");
    const line = markup.lastIndexOf("</button>", chip);
    expect(line).toBeGreaterThan(markup.indexOf("ms-decisions-row-open"));
    expect(markup.indexOf("ms-decisions-row-open")).toBeLessThan(chip);
  });

  it("counts every waiting line about that goal, and wears the danger colour for a refusal", () => {
    expect(queue([proposal(), proposal({ index: 1, verb: "edit-goal" })])).toContain(
      'aria-label="2 proposed on g1-s40, 2 actions"',
    );
    const refused = queue([proposal({ state: "refused", words: "g1-s40 is claimed by m2a" })]);
    expect(refused).toContain("ms-chip-proposed--wrong");
    expect(refused).toContain("Not now refused");
  });

  it("is absent where nothing about the goal waits", () => {
    expect(queue([])).not.toContain("ms-chip-proposed");
    expect(queue([proposal({ state: "applied" })])).not.toContain("ms-chip-proposed");
  });

  /**
   * And on the queue's rows only, which is the group the design names: the one
   * where a goal is triaged before it is opened. The other groups that carry a
   * goal — parked, stopped — are a one-line change the day somebody wants it, and
   * the proposal group's own rows never carry it, because a proposal row IS the
   * proposal and a chip saying so would be the row telling a human what they are
   * reading. Both fall out of the one condition: the row's kind.
   */
  it("is on the queue's rows and no other group's, goal or not", () => {
    for (const [group, goal, said] of [
      ["questions", "Q-1", "Which census format?"],
      ["parked", "g1-s45", "The Fleet section reads the census"],
      ["stopped", "g1-s48", "Work a breach fence stopped"],
    ] as const) {
      const markup = queue([proposal({ goal })], { chosen: group });
      // The row is on the screen, so the absence below is the chip's and not the
      // group's.
      expect({ group, shown: markup.includes(said) }).toEqual({ group, shown: true });
      expect({ group, chip: markup.includes("ms-chip-proposed") }).toEqual({ group, chip: false });
    }
    // And one chip on the page where it does stand, not one per group it names.
    expect(queue([proposal()]).match(/ms-chip-proposed/g)).toHaveLength(1);
  });
});
