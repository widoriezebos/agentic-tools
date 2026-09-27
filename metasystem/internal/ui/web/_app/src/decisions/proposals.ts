import { useCallback, useMemo, useRef, useState } from "react";

import type { Need, Proposed } from "./api";
import type { Proposal } from "../partner/api";
import { lookOnce, sendProposal, writeOutcome } from "../partner/applying";
import { loadBacklog, type Backlog } from "../backlog/api";
import {
  APPLY,
  displayedFor,
  excludedFor,
  lineID,
  lineState,
  markOf,
  noRun,
  releaseRun,
  runProposals,
  takeRun,
  TRY_AGAIN,
  verbWord,
  type Displayed,
  type Displayeds,
  type Line,
  type Mark,
  type Marks,
  type RunPorts,
  type Written,
} from "../partner/proposing";

/**
 * A proposal in the inbox: one row of a Decisions group, applied through the
 * card's own runner.
 *
 * The conversation is where a proposal is made and answered, and the wrong place
 * to keep one that waits. So the row here and the card in the transcript are two
 * readings of ONE record — the entry on the Partner's own message — and applying
 * from either is the same act on the same line, under the same version
 * (g1-s60 D4, D5).
 *
 * Nothing here is a second copy of the run. The rules — what may be sent, the
 * fetch-first read, the freshness guard, the two writes per line, a refusal
 * passed and an unknown answer stopping — are proposing.ts's, and this is what
 * that run needs of an inbox rather than of a card: the row's own state, the
 * budget an approve row read when it opened, and the page's own in-place re-read
 * as what happens after a confirmed act.
 */

/* ------------------------------------------------------------- the lines -- */

/** The one verb whose line carries a budget, and so a read when it opens. */
const APPROVE = "approve-goal";

/**
 * The entries a run has moved, by the line's own id.
 *
 * It is how a row shows what a press did before the page has read its payload
 * again, and how it shows what SOMEBODY ELSE did: the outcome route answers a
 * write it refused with the entry as it stands, and the runner's whole reaction
 * is to show that entry and send nothing for it (g1-s60 D5).
 */
export type Standing = Readonly<Record<string, Proposal>>;

/** The id one proposal row is known by: the card's own spelling of it. */
export function rowID(proposed: Proposed): string {
  return lineID(proposed.turn, proposed.index);
}

/**
 * The answer one line belongs to.
 *
 * It is read back out of the line's own id, which is the conversation coordinate
 * `lineID` mints from the turn and the index — the turn is sixteen random bytes
 * in base64 and carries no `#`. The card's runner closes over one answer because
 * a card IS one answer; the inbox's lines come from every answer that still has
 * something waiting, so each line has to say which.
 */
export function turnOf(line: Line): string {
  const cut = line.id.lastIndexOf("#");
  return cut < 0 ? "" : line.id.slice(0, cut);
}

/**
 * One row's line, as the runner and the card both read a line.
 *
 * Three things stand over the payload, in this order: the entry a run or a
 * conflict left, where it is at least as new as the payload's own; the marks the
 * page holds about the line; and the tuple an approve row read when it opened. A
 * row of any other kind has no line at all.
 *
 * The version decides which entry wins, and not the arrival. A payload read
 * after a write carries that write; an entry held from before it would put the
 * row back where it was and let a press send a version the server has moved past.
 */
export function lineOf(
  need: Need,
  marks: Marks = {},
  displayed: Displayeds = {},
  standing: Standing = {},
): Line | null {
  const proposed = need.proposal;
  if (proposed === null) {
    return null;
  }
  const id = rowID(proposed);
  const held = standing[id];
  const carried = held !== undefined && held.version >= proposed.version ? held : null;
  return {
    index: proposed.index,
    verb: proposed.verb,
    goal: need.where.id,
    title: need.title,
    fields: proposed.fields,
    read: proposed.read,
    why: proposed.explanation,
    offered: true,
    state: carried === null ? proposed.state : carried.state,
    words: carried === null ? proposed.words : (carried.words ?? ""),
    at: need.since,
    version: carried === null ? proposed.version : carried.version,
    id,
    mark: markOf(marks, id),
    displayed: proposed.verb === APPROVE ? (displayed[id] ?? null) : null,
  };
}

/** Every proposal row's line, in the order the rows stand. */
export function linesOf(
  needs: readonly Need[],
  marks: Marks = {},
  displayed: Displayeds = {},
  standing: Standing = {},
): Line[] {
  const lines: Line[] = [];
  for (const need of needs) {
    const line = lineOf(need, marks, displayed, standing);
    if (line !== null) {
      lines.push(line);
    }
  }
  return lines;
}

/**
 * What a proposal row's collapsed line says: the verb's own word, the subject,
 * and where the line stands where that is anything but waiting.
 *
 * The state is on the line because a refused, unresolved or in-flight row is
 * exactly the row a human is looking for: the group is a list of choices, and one
 * that has already been answered once is a different choice from one nobody has
 * touched (g1-s60 D3).
 */
export function proposalLine(need: Need): string {
  const line = lineOf(need);
  if (line === null) {
    return need.title;
  }
  const said = lineState(line);
  const head = `${verbWord(line.verb)} · ${need.title}`;
  return said === "" ? head : `${head} · ${said}`;
}

/**
 * What the row's own first act says.
 *
 * Try again where the line has already been answered once — refused, unresolved,
 * or left in flight by a page that went away — and Apply where nobody has
 * touched it. It is the card's own pair of words, for the card's own reason: a
 * press on a line that has been answered is a retry, and a human has to be able
 * to read that before they make it.
 */
export function applyLabel(line: Line): string {
  return line.state === "waiting" ? APPLY : TRY_AGAIN;
}

/* --------------------------------------------------------------- the run -- */

/**
 * What the inbox hands the runner, beyond the three impure edges the card hands
 * it too.
 *
 * Every one of them is the page's own: where a reconciled entry goes, what the
 * page holds about a line, the in-place re-read, and the sign-in sheet.
 */
export type RunAsk = {
  /** Show one line as the route now holds it, and keep it on the row. */
  reconcile: (id: string, proposal: Proposal) => void;
  mark: (id: string, change: Partial<Mark>) => void;
  /** The page's own read, in place: a re-read that blanked it would take the
   *  rows out from under the run that is reporting on them. */
  reread: () => void;
  /** The sign-in sheet, handed the lines a signed-in human would run on from. */
  signIn: (rest: readonly Line[]) => void;
};

/**
 * The three impure edges, named so a run can be driven without a browser.
 *
 * They are the module both callers share; a test hands its own three and reads
 * what the ports did with them.
 */
export type Through = {
  look: typeof lookOnce;
  record: typeof writeOutcome;
  send: typeof sendProposal;
};

const live: Through = { look: lookOnce, record: writeOutcome, send: sendProposal };

/**
 * The ports one run of the inbox's is made through.
 *
 * `record` names the line's OWN answer rather than one the ports were built
 * with, because a bulk press can carry lines from several answers: the inbox
 * lists every proposal that still waits, and two of them can have been proposed
 * a day apart.
 */
export function portsFor(ask: RunAsk, through: Through = live): RunPorts {
  return {
    look: through.look,
    record: (line, state, words) => through.record(turnOf(line), line, state, words),
    send: through.send,
    mark: (line, change) => {
      ask.mark(line.id, change);
    },
    reconcile: (proposal, line) => {
      ask.reconcile(line.id, proposal);
    },
    reread: ask.reread,
    signIn: ask.signIn,
  };
}

/** Put one line away: the human's own press, and the row leaves the group.
 *
 * It is a write and no act: nothing is published, and what it says is that this
 * human is not going to answer this proposal. Every state a row of this group can
 * be in is one `dismissed` is admitted from — the group carries nothing settled —
 * and it goes at the version the row rendered, so a line somebody else moved
 * first answers with a conflict and stays as they left it (g1-s58 D6).
 */
export async function dismissLine(line: Line, through: Through = live): Promise<Written> {
  return through.record(turnOf(line), line, "dismissed", "");
}

/* ------------------------------------------------- what the page holds -- */

/**
 * What an open proposal row can do, handed down rather than reached for.
 *
 * It is the row's own half of the page's state: the line as everything the page
 * holds about it composes it, the three presses, and whether a run is in flight —
 * because nothing else may be pressed while one is.
 */
export type ProposalActs = {
  lineOf: (need: Need) => Line | null;
  /** Apply, or Try again where the line has been answered once. */
  onApply: (need: Need) => void;
  /** Dismiss, which needs no sheet: it publishes nothing. */
  onDismiss: (need: Need) => void;
  /** Ask the Partner: the drawer opens with the line's words in the composer. */
  onAsk: (need: Need) => void;
  /** The bar's Apply: the sheet opens over every ticked row on screen. */
  onBulkApply: (needs: readonly Need[]) => void;
  /** The bar's Dismiss, which needs no sheet: it publishes nothing. */
  onBulkDismiss: (needs: readonly Need[]) => void;
  running: boolean;
};

/**
 * Why one ticked line is not part of a bulk press, or "".
 *
 * Two reasons, and the second is a boundary rather than a rule about budgets. A
 * line that has been answered once — refused, unresolved, or left in flight by a
 * page that went away — is a RETRY, and a retry is one line at a time: the human
 * reads what happened, checks the goal, and presses Try again on that row. Bulk
 * Try again is a later slice, and a bar that quietly did it would be that slice
 * without the reading it needs (g1-s60, "Not here, later").
 */
export function excludedInBulk(line: Line): string {
  return line.state === "waiting" ? excludedFor(line) : ANSWERED_ONCE;
}

/** What a line that has already been answered says in the sheet. */
export const ANSWERED_ONCE = "answered once already: Try again on the row itself";

/** What the sheet says where nothing it lists can be sent at all. */
export const NOTHING_SENDABLE =
  "Nothing here can be sent: an action that has been answered once is tried again on its own row, " +
  "and an approve with no tuple is approved alone.";

/** What the pane holds about the proposals on it, and how it applies one. */
export type Applying = {
  /** One row's line, over everything the page holds about it. */
  lineOf: (need: Need) => Line | null;
  /** Every proposal row's line, in the order the rows stand. */
  linesOf: (needs: readonly Need[]) => Line[];
  /** Apply these lines, in this order, one act each, never retried. */
  run: (lines: readonly Line[]) => void;
  /** The line a run is in flight for, or "" where none is. */
  running: boolean;
  /**
   * Read the budget an approve row would carry, once, and keep it with the row.
   *
   * It is the card's own moment moved: the card reads it when the answer ends,
   * because that is when its buttons wake, and a row reads it when it opens,
   * because that is when a human can see it. What is sent is what was read
   * (Astra S58-08).
   */
  captureBudgets: (needs: readonly Need[]) => void;
  /** The tuples this page has read, for a sheet that lists several lines. */
  budgets: Displayeds;
  /** Keep the tuples a sheet read when it opened, so the rows say the same. */
  noteBudgets: (read: Displayeds) => void;
  /**
   * Put these lines away, and read the page again when the writes have answered.
   *
   * One press, one guard and one re-read, however many lines: the bar's Dismiss
   * over four rows is one act of the human's — I am not going to answer these —
   * and four re-reads would be four blinks of the page.
   */
  dismiss: (lines: readonly Line[]) => void;
};

/**
 * The state one Decisions page holds about the proposals on it, and the one way
 * it applies them.
 *
 * It is a hook rather than pane state because it is one mechanism: the marks, the
 * entries a run moved, the tuples that were read and the guard that makes a press
 * one run are only meaningful together, and a pane holding four of them would be
 * a pane that can hold three.
 */
export function useProposals(ask: {
  reread: () => void;
  signIn: (rest: readonly Line[]) => void;
}): Applying {
  const [marks, setMarks] = useState<Marks>({});
  const [standing, setStanding] = useState<Standing>({});
  const [budgets, setBudgets] = useState<Displayeds>({});
  const [running, setRunning] = useState("");
  // The run in flight, held synchronously so a second press in the same frame
  // does nothing: a state flag is a render away and a press is not (g1-s58 D5).
  const guard = useRef(noRun());
  // Which lines a budget has already been asked for, held synchronously for the
  // guard's own reason: the answer is a render away, and a row that asked twice
  // before the first read landed would be reading the backlog twice for one
  // opening.
  const asked = useRef(new Set<string>());
  const { reread, signIn } = ask;

  const mark = useCallback((id: string, change: Partial<Mark>) => {
    setMarks((held) => ({ ...held, [id]: { ...markOf(held, id), ...change } }));
  }, []);

  const reconcile = useCallback((id: string, proposal: Proposal) => {
    setStanding((held) => ({ ...held, [id]: proposal }));
  }, []);

  const lineFor = useCallback(
    (need: Need) => lineOf(need, marks, budgets, standing),
    [marks, budgets, standing],
  );

  const linesFor = useCallback(
    (needs: readonly Need[]) => linesOf(needs, marks, budgets, standing),
    [marks, budgets, standing],
  );

  const run = useCallback(
    (lines: readonly Line[]) => {
      if (lines.length === 0 || !takeRun(guard.current, lines[0].id)) {
        return;
      }
      setRunning(lines[0].id);
      void (async () => {
        try {
          await runProposals(lines, portsFor({ reconcile, mark, reread, signIn }));
        } finally {
          releaseRun(guard.current);
          setRunning("");
        }
      })();
    },
    [reconcile, mark, reread, signIn],
  );

  const captureBudgets = useCallback(
    (needs: readonly Need[]) => {
      const lines = linesOf(needs).filter(
        (line) => line.verb === APPROVE && !asked.current.has(line.id),
      );
      if (lines.length === 0) {
        return;
      }
      for (const line of lines) {
        asked.current.add(line.id);
      }
      loadBacklog()
        .then((read) => {
          setBudgets((held) => ({ ...held, ...budgetsFrom(lines, read) }));
        })
        .catch(() => {
          // No budget could be read, so no approve row has one and each says so,
          // which is the truthful answer for a page that could not read the law.
          setBudgets((held) => ({ ...held, ...nothingRead(lines) }));
        });
    },
    [],
  );

  const noteBudgets = useCallback((read: Displayeds) => {
    setBudgets((held) => ({ ...held, ...read }));
  }, []);

  const dismiss = useCallback(
    (lines: readonly Line[]) => {
      if (lines.length === 0 || !takeRun(guard.current, lines[0].id)) {
        return;
      }
      setRunning(lines[0].id);
      void (async () => {
        for (const line of lines) {
          const answered = await dismissLine(line);
          if (answered.kind === "failed") {
            // The conversation could not write it down, so the row says so and
            // stays: what a human pressed did not happen.
            mark(line.id, { refusedUnsent: answered.words });
          } else {
            // A line somebody else moved first comes back as a conflict carrying
            // the entry, and the row then shows what they did.
            setStanding((held) => ({ ...held, [line.id]: answered.proposal }));
          }
        }
        releaseRun(guard.current);
        setRunning("");
        reread();
      })();
    },
    [mark, reread],
  );

  // One object, kept while nothing it answers from has moved: the pane hands it
  // to an effect and to the row's own handles, and a fresh one every render
  // would re-run that effect every render.
  return useMemo(
    () => ({
      lineOf: lineFor,
      linesOf: linesFor,
      run,
      running: running !== "",
      captureBudgets,
      budgets,
      noteBudgets,
      dismiss,
    }),
    [lineFor, linesFor, run, running, captureBudgets, budgets, noteBudgets, dismiss],
  );
}

/** The tuples these lines would carry, from one read of the backlog. */
export function budgetsFrom(lines: readonly Line[], read: Backlog): Displayeds {
  return displayedFor(lines, [...read.rows, ...read.closed], read.budgetDefaults);
}

/** What every approve line says where no budget could be read at all. */
export function nothingRead(lines: readonly Line[]): Displayeds {
  const held: Record<string, Displayed> = {};
  for (const line of lines) {
    held[line.id] = { budget: null, source: "none" };
  }
  return held;
}
