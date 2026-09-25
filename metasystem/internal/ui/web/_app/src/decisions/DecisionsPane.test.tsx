import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Need, Page, Ruling } from "./api";
import { Blocks } from "./DecisionsPane";
import type { Row } from "../backlog/api";
import type { TabId } from "./decisions";

/**
 * What the page puts on the screen, over a payload and nothing else.
 *
 * The reading, the network and the sheet belong to the pane; these are the two
 * blocks, rendered as the browser would render them. Every assertion is a
 * sentence a human reads: the silence line, the act, the way through, the
 * command, the register's own words.
 */

/** A row of the ledger, as much of one as a card reads. */
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
    mentions: ["g1-s12"],
    ...over,
  };
}

/** Every kind of thing that can wait on a human, one of each. */
const everyKind: Need[] = [
  need({ kind: "renewal", id: "g1-s42", asked: "Renew the approval of g1-s42: the review date 2026-09-06 has passed", silence: "no fresh claim is admitted" }),
  need({
    kind: "renewal", id: "g1-s43", asked: "Renew the approval of g1-s43: the review date 2026-09-07 has passed",
    silence: "work already claimed continues; renew at the goal", act: "", row: null,
  }),
  need({
    kind: "ruling-review", id: "R-3", title: "R-3",
    asked: "Review R-3, due 2026-09-19: adopt, revise or withdraw",
    by: "Wido", silence: "it stays in force as written", act: "", row: null,
    where: { kind: "register", id: "memory/rulings.md" },
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
    by: "the register", silence: "it stays open", act: "", row: null,
    where: { kind: "question", id: "Q-1" },
  }),
  need({
    kind: "parked", id: "g1-s45", title: "The Fleet section reads the census",
    asked: "Unpark The Fleet section reads the census? parked by human:Wido 2026-09-24T05:00:00Z: the census format is still being decided",
    by: "human:Wido", silence: "it stays parked", act: "", row: null,
    command: "metasystem goal unpark --id g1-s45",
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
  }),
  need({
    kind: "landed", id: "d-landed", title: "Every goal of this one landed",
    asked: "Mark Every goal of this one landed done?", by: "every goal landed",
    silence: "it stays marked accepted", act: "", row: null,
    where: { kind: "record", id: "plans/designs/landed.md" },
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
    schemaVersion: 1,
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
    },
    counts: { needsYou: everyKind.length, rulings: 148 },
    ...over,
  };
}

function rendered(payload: Page, tab: TabId = "rulings"): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <Blocks page={payload} tab={tab} />
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

describe("what needs your choice", () => {
  it("says of every kind what is asked and what silence does", () => {
    const markup = rendered(page());

    for (const one of everyKind) {
      expect(markup).toContain(one.asked);
      expect(markup).toContain(`if you do nothing: ${one.silence}`);
    }
    // The whole list, never a capped group: no row says "and N more".
    expect(markup).not.toContain("more →");
    expect(markup).toContain(String(everyKind.length));
  });

  it("carries the asker's recommendation where the record has one, and no other row invents one", () => {
    const markup = rendered(page());

    expect(markup).toContain("Recommended: raise it once; what is left is small and well understood");
    expect(markup.match(/Recommended:/g)).toHaveLength(1);
  });

  it("offers the act on the rows the server said carry one, and a way through on the rest", () => {
    const markup = rendered(page());

    // Two approvals: the unapproved goal and the unclaimed renewal. The
    // claimed renewal links to its goal instead, because the engine refuses a
    // budget on claimed work.
    expect(markup.match(/>Approve</g)).toHaveLength(2);
    expect(markup).toContain("Answer on the fleet channel with your code");
    expect(markup).toContain("Open the register");
    expect(markup).toContain("Open the record");
    expect(markup).toContain("Open the message");
  });

  it("names the terminal command where the terminal is the way", () => {
    const markup = rendered(page());

    expect(markup).toContain("metasystem goal unpark --id g1-s45");
    expect(markup).toContain("metasystem goal resume --id g1-s48");
  });

  it("says so plainly when nothing is waiting", () => {
    const markup = rendered(page({ needsYou: [], counts: { needsYou: 0, rulings: 148 } }));

    expect(markup).toContain("Nothing is waiting on you.");
    expect(markup).not.toContain("if you do nothing");
  });

  it("puts signing in at the top, as its own row, when nothing proves a human", () => {
    expect(rendered(page({ signIn: true }))).toContain("Sign in to act");
    expect(rendered(page())).not.toContain("Sign in to act");
  });
});

describe("what you decided", () => {
  it("names the four tabs with their counts, the register's own count among them", () => {
    const markup = rendered(page());

    expect(markup).toContain("Rulings 148");
    expect(markup).toContain("Decisions 1");
    expect(markup).toContain("Answered 1");
    expect(markup).toContain("Approved 2");
  });

  it("renders a ruling whole, with its owner, its review chip and its mentions", () => {
    const markup = rendered(page());

    // The words whole, escaped as markup escapes an apostrophe and not
    // shortened: the register is the record, and the card renders all of it.
    expect(markup).toContain("two drag moves are the only acts the browser publishes, and g1-s12 is where they land");
    expect(markup).not.toContain("…");
    expect(markup).toContain("owner Wido");
    expect(markup).toContain("review passed 6 days ago");
    expect(markup).toContain("/backlog/goal/g1-s12");
    // The context is there and behind a disclosure.
    expect(markup).toContain("Given with the board design");
    expect(markup).toContain("<summary");
  });

  it("lists the register's broken rows rather than hiding them", () => {
    const markup = rendered(page());

    expect(markup).toContain("1 row of the register could not be read");
    expect(markup).toContain("R-9: review condition needs due= or event=");
  });

  it("says nothing about defects when the register has none", () => {
    const empty = page();
    const markup = rendered({ ...empty, decided: { ...empty.decided, defects: [] } });

    expect(markup).not.toContain("could not be read");
  });

  it("opens each of the other three tabs on its own rows", () => {
    expect(rendered(page(), "decisions")).toContain("The roster for design work");
    expect(rendered(page(), "answered")).toContain("Who owns the sweep?");
    expect(rendered(page(), "approved")).toContain("Approved and ready to claim");
  });

  it("carries Withdraw only where the board's own eligibility allows", () => {
    const markup = rendered(page(), "approved");

    // Ready for Work carries it; a concluded goal does not.
    expect(markup.match(/>Withdraw approval</g)).toHaveLength(1);
    expect(markup).toContain("The shell, the rail and the header");
  });

  it("says what each empty tab is empty of", () => {
    const empty = page();
    const bare = { ...empty, decided: { rulings: [], defects: [], decisions: [], answered: [], approved: [] }, counts: { needsYou: 0, rulings: 0 } };

    expect(rendered(bare, "decisions")).toContain("This project has recorded no decisions yet.");
    expect(rendered(bare, "answered")).toContain("No question of the register has been answered yet.");
    expect(rendered(bare, "approved")).toContain("No goal carries an approval yet.");
    expect(rendered(bare, "rulings")).toContain("No ruling of the register matches.");
  });
});

describe("the phone", () => {
  // The page has one column at every width, and the card stacks through its
  // own stylesheet rather than through a second markup: nothing here renders
  // differently at 400px, which is what keeps the two widths one page.
  it("renders one markup, which the stylesheet stacks", () => {
    expect(rendered(page())).toBe(rendered(page()));
    expect(rendered(page())).toContain("ms-decisions-need-way");
  });
});
