import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Need, Page, Ruling } from "./api";
import { Blocks } from "./DecisionsPane";
import { noNarrowing } from "./decisions";
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
    schemaVersion: 2,
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
    register: "metasystem/memory/rulings.md",
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

describe("what is asked of you", () => {
  it("says of every kind what is asked and what silence does", () => {
    const markup = rendered(page());

    // Every kind but the approvals, which are the queue below and say their
    // silence once, in its head, rather than on every row.
    for (const one of everyKind.filter((kind) => kind.kind !== "approval")) {
      expect(markup).toContain(one.asked);
      expect(markup).toContain(`if you do nothing: ${one.silence}`);
    }
    // The whole list, never a capped group: no row says "and N more".
    expect(markup).not.toContain("more →");
  });

  it("does not repeat an approval's derived sentence, which said the title twice", () => {
    const markup = rendered(page());

    expect(markup).not.toContain("for execution");
    // The title is still there, once, as the queue row a human reads.
    expect(markup).toContain("The pane reads a document as a chapter");
  });

  it("counts the two blocks in the header, each a way into its own", () => {
    const markup = rendered(page());

    expect(markup).toContain("10 asked of you");
    expect(markup).toContain("1 waiting for your approval");
    expect(markup).toContain('href="#decisions-asked"');
    expect(markup).toContain('href="#decisions-waiting"');
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

    expect(markup).toContain("metasystem goal resume --id g1-s48");
  });

  // A seat's park is still asked of a human, because a human has not seen it
  // — but the way out of it is a button now, not a command to copy.
  it("offers a seat's park the button that returns it, and no command", () => {
    const markup = rendered(page());

    expect(markup).toContain("Return to queue");
    expect(markup).not.toContain("metasystem goal unpark");
  });

  it("says so plainly when each block is empty", () => {
    const markup = rendered(page({ needsYou: [], counts: { needsYou: 0, asked: 0, waiting: 0, rulings: 148 } }));

    expect(markup).toContain("Nothing is asked of you.");
    expect(markup).toContain("Nothing waits for your approval.");
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

  // A row whose review condition is a date AND an event keeps both on the
  // card. The register carries them — R-29-m2 is due on a date and on a
  // terminal re-arm, whichever comes first — and the date used to win.
  it("keeps the event on a card whose row also carries a due date", () => {
    const both = page();
    const markup = rendered({
      ...both,
      decided: {
        ...both.decided,
        rulings: [
          ruling({
            id: "R-29-m2",
            due: "2026-09-19",
            event: "terminal-re-arm",
            condition: "class=temporary due=2026-09-19 event=terminal-re-arm",
            duePassed: true,
          }),
        ],
      },
    });

    expect(markup).toContain("review passed 6 days ago · or on terminal-re-arm");
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
    const bare = {
      ...empty,
      decided: { rulings: [], defects: [], decisions: [], answered: [], approved: [], notNow: [] },
      counts: { needsYou: 0, asked: 0, waiting: 0, rulings: 0 },
    };

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

describe("the queue", () => {
  const queued = [
    need({ id: "g1-s40", title: "The queue narrows by label", row: row({ ref: { kind: "goal", id: "g1-s40", revision: 3 }, intent: "The queue narrows by label and by origin", labels: ["browser-interface"], origin: "human", tier: 2, openedAt: "2026-09-22T00:00:00Z" }) }),
    need({ id: "g1-s41", title: "The fleet page reads a chain", row: row({ ref: { kind: "goal", id: "g1-s41", revision: 3 }, intent: "The fleet page reads a seat's whole chain", labels: ["headless-fleet"], origin: "main", tier: 0, openedAt: "2026-09-24T00:00:00Z" }) }),
  ];
  const withQueue = (over: Partial<Page> = {}) =>
    page({
      needsYou: queued,
      counts: { needsYou: 2, asked: 0, waiting: 2, rulings: 148 },
      ...over,
    });

  it("says its silence once, in the block's head, rather than on every row", () => {
    const markup = rendered(withQueue());

    expect(markup).toContain("If you do nothing, these stay in To Do and no seat may claim them.");
    expect(markup.match(/stay in To Do and no seat may claim them/g)).toHaveLength(1);
  });

  it("shows the row's age, its tier above zero, whose it is, and its labels", () => {
    const markup = rendered(withQueue());

    expect(markup).toContain(">tier 2<");
    // Tier zero is a record that declares none, so no chip claims one.
    expect(markup).not.toContain(">tier 0<");
    expect(markup).toContain(">yours<");
    expect(markup).toContain(">browser-interface<");
    expect(markup).toContain(">headless-fleet<");
  });

  it("opens a row in place, with the whole intent and what a decision needs", () => {
    const shut = rendered(withQueue());
    const open = renderToStaticMarkup(
      <MemoryRouter>
        <TooltipPrimitive.Provider>
          <Blocks page={withQueue()} tab="rulings" opened={["g1-s40"]} />
        </TooltipPrimitive.Provider>
      </MemoryRouter>,
    );

    expect(shut).toContain('aria-expanded="false"');
    expect(shut).not.toContain("The queue narrows by label and by origin");
    expect(open).toContain('aria-expanded="true"');
    expect(open).toContain("The queue narrows by label and by origin");
    expect(open).toContain("no budget recorded");
    expect(open).toContain("priority 2, position 1");
  });

  it("draws a label chip per label on screen, with its count", () => {
    const markup = rendered(withQueue());

    expect(markup).toContain("browser-interface 1");
    expect(markup).toContain("headless-fleet 1");
    expect(markup).toContain(">yours</button>");
    expect(markup).toContain("seats&#x27;</button>");
  });

  it("counts what a bulk act would act on, and acts on nothing at zero", () => {
    const none = rendered(withQueue());
    const some = renderToStaticMarkup(
      <MemoryRouter>
        <TooltipPrimitive.Provider>
          <Blocks page={withQueue()} tab="rulings" selected={["g1-s40"]} />
        </TooltipPrimitive.Provider>
      </MemoryRouter>,
    );

    expect(none).toContain("Approve 0 selected");
    expect(none).toContain("Not now for 0 selected");
    expect(none.match(/disabled=""/g)?.length).toBeGreaterThan(1);
    expect(some).toContain("Approve 1 selected");
    expect(some).toContain("Not now for 1 selected");
  });

  // A tick on a row the narrowing then hid is a tick on a row a human can no
  // longer see. Acting on it would be the one thing a queue must never do.
  it("acts only on rows that are ticked AND on screen", () => {
    const markup = renderToStaticMarkup(
      <MemoryRouter>
        <TooltipPrimitive.Provider>
          <Blocks
            page={withQueue()}
            tab="rulings"
            selected={["g1-s40", "g1-s41"]}
            narrowing={{ ...noNarrowing, label: "headless-fleet" }}
          />
        </TooltipPrimitive.Provider>
      </MemoryRouter>,
    );

    expect(markup).toContain("Approve 1 selected");
    expect(markup).toContain("2 waiting · 1 shown");
    expect(markup).not.toContain("The queue narrows by label<");
  });

  it("says so when the tools leave nothing on screen", () => {
    const markup = renderToStaticMarkup(
      <MemoryRouter>
        <TooltipPrimitive.Provider>
          <Blocks page={withQueue()} tab="rulings" narrowing={{ ...noNarrowing, find: "nothing says this" }} />
        </TooltipPrimitive.Provider>
      </MemoryRouter>,
    );

    expect(markup).toContain("No goal in the queue matches.");
    expect(markup).toContain("2 waiting · 0 shown");
  });
});

describe("not now", () => {
  it("lists the parks this human made, with the reason and the way back", () => {
    const markup = rendered(page(), "not-now");

    expect(markup).toContain("not before the board&#x27;s own filters settle");
    expect(markup).toContain("The register opens from the review card");
    // A blocker park says what it is waiting for, because it lifts by itself.
    expect(markup).toContain("waits for g1-s24");
    expect(markup.match(/>Return to queue</g)).toHaveLength(3);
  });

  it("says so plainly when this human has paused nothing", () => {
    const empty = page();
    const bare = { ...empty, decided: { ...empty.decided, notNow: [] } };

    expect(rendered(bare, "not-now")).toContain("You have paused nothing.");
  });

  it("offers no way back at all when nothing proves a human", () => {
    const markup = rendered(page({ signIn: true }), "not-now");

    expect(markup).toContain("Return to queue");
    expect(markup).toContain('disabled=""');
  });
});

describe("signing in while the page is open", () => {
  // The payload carries signIn as it stood when the page was read. A human
  // who opens Decisions signed out, signs in through "Sign in to act" and
  // presses Not now must not meet a disabled button: the two acts this page
  // publishes are gated on the LIVE session, not on the stale payload.
  const RETURN_OFF = '<button type="button" class="ms-button" disabled="">Return to queue</button>';
  const RETURN_ON = '<button type="button" class="ms-button">Return to queue</button>';
  const NOT_NOW_OFF = '<button type="button" class="ms-button" disabled="">Not now</button>';
  const NOT_NOW_ON = '<button type="button" class="ms-button">Not now</button>';

  function withSession(payload: Page, live: boolean | undefined, tab: TabId = "not-now"): string {
    return renderToStaticMarkup(
      <MemoryRouter>
        <TooltipPrimitive.Provider>
          <Blocks page={payload} signedIn={live} tab={tab} />
        </TooltipPrimitive.Provider>
      </MemoryRouter>,
    );
  }

  it("takes the payload's answer when nothing else is known", () => {
    expect(withSession(page({ signIn: true }), undefined)).toContain(RETURN_OFF);
    expect(withSession(page(), undefined)).toContain(RETURN_ON);
  });

  it("enables Return to queue the moment the session says a human is signed in", () => {
    const stale = page({ signIn: true });

    expect(withSession(stale, false)).toContain(RETURN_OFF);
    // The same stale payload, and the buttons are live.
    expect(withSession(stale, true)).toContain(RETURN_ON);
    expect(withSession(stale, true)).not.toContain(RETURN_OFF);
  });

  it("enables Not now on the queue rows too, on the same live answer", () => {
    const queued = page({
      signIn: true,
      needsYou: [need({ id: "g1-s40", title: "A goal nobody has authorized" })],
      counts: { needsYou: 1, asked: 0, waiting: 1, rulings: 0 },
    });

    expect(withSession(queued, false, "rulings")).toContain(NOT_NOW_OFF);
    expect(withSession(queued, true, "rulings")).toContain(NOT_NOW_ON);
    expect(withSession(queued, true, "rulings")).not.toContain(NOT_NOW_OFF);
  });

  it("leaves a seat's park in the inbox on the same live answer", () => {
    const seatPark = page({
      signIn: true,
      needsYou: [
        need({ kind: "parked", id: "g1-s37", title: "A goal a seat parked", act: "unpark", row: null }),
      ],
      counts: { needsYou: 1, asked: 1, waiting: 0, rulings: 0 },
    });

    expect(withSession(seatPark, false, "rulings")).toContain(RETURN_OFF);
    expect(withSession(seatPark, true, "rulings")).toContain(RETURN_ON);
  });
});
