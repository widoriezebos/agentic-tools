import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import type { ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Backlog, Row } from "./backlog/api";
import { LedgerStatement } from "./backlog/BacklogPane";
import { EditSheet } from "./backlog/EditSheet";
import { OpenSheet } from "./backlog/OpenSheet";
import type { Proposal } from "./partner/api";
import { emptyStore, refused } from "./partner/conversation";
import { DepositCard } from "./partner/Deposit";
import { ProposalCard } from "./partner/Proposal";
import { cardsIn } from "./partner/proposing";
import type { Card, Entry } from "./partner/sitting";
import { PartnerAs } from "./partner/store";
import { SuggestionCard } from "./partner/Suggestion";
import type { Offered } from "./partner/suggesting";
import { Transcript } from "./partner/Transcript";
import { Refused } from "./review/Desk";
import { EndWays, SendBackBrief } from "./review/ReviewRoom";
import { CLEAR_REFUSED, NO_FIX, UNANSWERED } from "./review/room";
import { TroublesAs } from "./shell/troubles";
import { ASK_WHAT_HAPPENED } from "./shell/troubling";

/**
 * The refusal lines the first build left drawn on their own (Sol SOL-S68-01):
 * each now renders through `Trouble`, and so carries Ask what happened once the
 * Partner has registered its ask. Read from the markup each site renders, in the
 * state that shows its refusal.
 */

const ASK = `<button type="button" class="ms-trouble-ask">${ASK_WHAT_HAPPENED}</button>`;

function rendered(node: ReactNode, partner: Parameters<typeof PartnerAs>[0]["held"] = {}): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <TroublesAs held={{ ask: () => {} }}>
          <PartnerAs held={partner}>{node}</PartnerAs>
        </TroublesAs>
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

function row(over: Partial<Row> = {}): Row {
  return {
    ref: { kind: "goal", id: "ui-1", revision: 1 }, where: "plans/goals/ui-1.md", lane: "to-do", phase: "",
    state: "open", intent: "The board reads.", nextStep: "Take it to an end state.", concluded: "", origin: "human",
    priority: 2, sequence: 1, tier: 1, labels: [], arc: "", pinned: "", blockedBy: [], holds: [], openBlockers: [],
    sliced: false, decomposed: false, openedAt: "", doneAt: "", lastChangeAt: "", lastVerb: "open", gaps: [], ...over,
  };
}

function backlog(): Backlog {
  return {
    schemaVersion: 1,
    observedAt: "2026-09-24T08:00:00Z",
    ledger: {
      state: "read", tip: "abc1234", committedAt: "2026-09-24T07:00:00Z",
      freshness: { state: "current", since: "2026-09-24T07:59:00Z", detail: "" },
      stale: false, staleAfterSeconds: 900, syncMode: "", stateRoot: "", message: "", problems: [],
      fetch: {
        outcome: "current", startedAt: "", finishedAt: "", tip: "abc1234", detail: "", message: "", failures: 0,
        cadence: "", nextAt: "", succeededAt: "", succeededTip: "abc1234",
      },
    },
    admission: { answered: true, message: "" },
    workingTree: { liveFiles: 1, archivedFiles: 0 },
    authority: { proven: true, human: "Wido", reason: "" },
    budgetDefaults: {},
    counts: {},
    draft: { statement: "" },
    rows: [row()],
    closed: [],
  };
}

function finding(text: string, answer: string, mark: string): Entry {
  return { when: "2026-09-29", who: "Wido", text, clause: "internal/owner.go:60", section: "Findings", mark, answer };
}

const noop = () => undefined;

describe("a refusal the first build drew on its own", () => {
  it("in the New goal sheet, a field's own refusal is a trouble line", () => {
    // The intent suggests the id ui-1, which the backlog already carries.
    const markup = rendered(<OpenSheet backlog={backlog()} intent="ui 1" onClose={noop} onDone={noop} />);
    expect(markup).toContain("The backlog already carries ui-1");
    expect(markup).toContain(ASK);
  });

  it("in the Edit goal sheet, the labels' refusal is a trouble line", () => {
    const markup = rendered(<EditSheet goal={row({ labels: ["UI"] })} backlog={backlog()} onClose={noop} onDone={noop} />);
    expect(markup).toContain("label &quot;UI&quot; must match");
    expect(markup).toContain(ASK);
  });

  it("on the room's desk, a refused read is a trouble line", () => {
    const markup = rendered(<Refused reason="the source could not be read: no such path at a1a1a1a" />);
    expect(markup).toContain("the source could not be read");
    expect(markup).toContain(ASK);
  });

  it("in Send back, no finding answered fix is a trouble line", () => {
    const markup = rendered(
      <SendBackBrief entries={[finding("the log is noisy", UNANSWERED, "deposit:t1#1")]} brief="" busy={false} onEdit={noop} onSend={noop} onBack={noop} />,
    );
    expect(markup).toContain(NO_FIX);
    expect(markup).toContain(ASK);
  });

  it("in End, Clear to land refused while a finding is unanswered is a trouble line, and still lists them", () => {
    const markup = rendered(
      <EndWays entries={[finding("a press that dies holds the lock", UNANSWERED, "deposit:t1#0")]} busy={false} onChoose={noop} />,
    );
    expect(markup).toContain(CLEAR_REFUSED);
    expect(markup).toContain("<li>a press that dies holds the lock</li>");
    expect(markup).toContain(ASK);
  });

  it("a suggestion not offered says why as a trouble line", () => {
    const card: Offered = {
      opening: "", editor: "Edit goal", field: "Intent", text: "The board reads the ledger.", offered: false,
      reason: "no sheet named Intent is open", id: "s1", open: false,
      mark: { previous: null, holds: false, dismissed: false, sent: "", words: "" }, standing: "refused",
    } as Offered;
    const markup = rendered(<SuggestionCard id="s1" />, { offered: [card] });
    expect(markup).toContain("no sheet named Intent is open");
    expect(markup).toContain(ASK);
  });

  it("a deposit not offered says why as a trouble line", () => {
    const card: Card = {
      kind: "finding", text: "the lock is held", anchor: "", offered: false, notOffered: "there is no sitting to offer it to",
      subject: { kind: "record", id: "plans/reviews/review-of-g1-s64.md", title: "" }, id: "local-1",
      mark: { text: "", clause: "", recording: false, recorded: "", refusal: "", dismissed: false },
      standing: "refused",
    } as Card;
    const markup = rendered(<DepositCard id="local-1" />, { deposits: [card] });
    expect(markup).toContain("there is no sitting to offer it to");
    expect(markup).toContain(ASK);
  });

  it("a proposed act not offered says why as a trouble line", () => {
    const proposal: Proposal = {
      index: 0, verb: "park-goal", goal: "fleet-presence", title: "Fleet presence", fields: {}, read: null, why: "",
      offered: false, reason: "fleet-presence is not a goal the ledger carries", state: "waiting", words: "",
      at: "2026-09-26T12:00:00Z", version: 1,
    };
    const markup = rendered(<ProposalCard id="t1" />, { proposals: cardsIn([{ turn: "t1", proposals: [proposal] }], {}, {}, []) });
    expect(markup).toContain("fleet-presence is not a goal the ledger carries");
    expect(markup).toContain(ASK);
  });
});

describe("the ledger that could not be read (Sol's re-read of SOL-S68-01)", () => {
  function failed(over: Partial<Backlog["ledger"]>): Backlog {
    const base = backlog();
    return { ...base, ledger: { ...base.ledger, ...over } };
  }

  it("says each problem a ledger that does not validate carries as a trouble line", () => {
    const markup = rendered(
      <LedgerStatement backlog={failed({ state: "unreadable", problems: ["plans/goals/ui-1.md: no intent", "plans/goals/ui-2.md: no lane"] })} />,
    );
    expect(markup).toContain("plans/goals/ui-1.md: no intent");
    expect(markup).toContain("plans/goals/ui-2.md: no lane");
    expect(markup.split(ASK).length - 1).toBe(2);
  });

  it("says an accepted ref that cannot be read as a trouble line", () => {
    const markup = rendered(<LedgerStatement backlog={failed({ state: "broken", message: "reference is not a commit" })} />);
    expect(markup).toContain("reference is not a commit");
    expect(markup).toContain(ASK);
  });
});

describe("the Partner's own refusal to take a turn", () => {
  it("is askable when the conversation is busy: the press waits as a pending chip", () => {
    const store = refused({ ...emptyStore, state: "ready" }, "a turn is already running in this conversation", "", true);
    const markup = rendered(<Transcript />, { store });
    expect(markup).toContain("a turn is already running in this conversation");
    expect(markup).toContain(ASK);
  });

  it("offers no Ask for any other refusal: a press would meet it again", () => {
    const store = refused({ ...emptyStore, state: "ready" }, "the runtime is not installed", "brew install x");
    const markup = rendered(<Transcript />, { store });
    expect(markup).toContain("the runtime is not installed");
    expect(markup).not.toContain(ASK);
  });
});
