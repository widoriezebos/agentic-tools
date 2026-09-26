import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Need, Page, Ruling } from "./api";
import { Views } from "./DecisionsPane";
import { noNarrowing, type Narrowing, type TabId } from "./decisions";
import type { Acts } from "./InboxRow";
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

function rendered(payload: Page, shown: Shown = {}): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <Views
          page={payload}
          acts={acts}
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
