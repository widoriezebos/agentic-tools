import { useState, type ReactNode } from "react";
import { NavLink } from "react-router";

import type { Need, Where as Reference } from "./api";
import { askedLine, bandLine, blockedLine, budgetLine, destinationFor, wayThrough } from "./decisions";
import { confirmLine, goalLine, rowFacts, rowLine, writeLabel, writeStatus } from "./groups";
import { Help } from "../help/Help";
import { useNotifications } from "../notifications/store";
import { Button, Chip } from "../shell/controls";

/**
 * One row of the inbox: a line, and what is under it when it is open.
 *
 * The line is the thing itself — the question, the ruling's words, the
 * record's title, the goal's intent — and then, muted and at the end, the few
 * facts that tell one row from its neighbours: whose goal it is, its tier, a
 * label or two, how old it is, and a dot where it arrived since the last
 * visit. There is no button on a line and no id on one. A hundred rows of two
 * buttons each is what made this page a wall, and an id fifty characters long
 * is a third of a line a human cannot decide by.
 *
 * One row is open at a time, and what opens is the whole of what that kind of
 * thing is, with the acts that finish it. Every act here runs through a route
 * that already exists: the board's sheets for a goal, the record status route
 * for a draft and a landed design, unpark for a seat's park, and the terminal
 * for a stopped one. Where a decision is made somewhere else, the row says
 * where instead of pretending to an act.
 */

/** What an open row can do, handed down rather than reached for. */
export type Acts = {
  /** Whether a human is signed in NOW, rather than when the payload was composed. */
  signedIn: boolean;
  onSignIn: () => void;
  /** The board's own approve sheet, over the row this need carries. */
  onApprove: (need: Need) => void;
  /** "Not now": the not-now sheet with this one goal in it. */
  onPark: (need: Need) => void;
  /** g1-s47's edit sheet, over the same row. */
  onEdit: (need: Need) => void;
  /** Unpark: one publication, then the page reads its payload again. */
  onReturn: (id: string) => void;
  /** A record's status: accepting a draft, marking a landed design done. */
  onWrite: (need: Need, status: string) => void;
  /** The row a write is in flight for, or "". */
  writing: string;
  /** The row the last refusal belongs to, and what it said. */
  refusedAt: string;
  refusal: string;
};

export function InboxRow({
  need,
  now,
  open,
  onOpen,
  acts,
  ticked,
  onTick,
}: {
  need: Need;
  now: Date;
  open: boolean;
  onOpen: () => void;
  acts: Acts;
  /** The queue's rows carry a checkbox; no other group has a bulk act. */
  ticked?: boolean;
  onTick?: () => void;
}) {
  const facts = rowFacts(need, now);
  return (
    <li className={open ? "ms-decisions-row-item ms-decisions-row-item--open" : "ms-decisions-row-item"}>
      <div className="ms-decisions-row-line">
        {onTick !== undefined && (
          <label className="ms-decisions-check">
            <input type="checkbox" checked={ticked ?? false} onChange={onTick} />
            <span className="ms-visually-hidden">Select {need.id}</span>
          </label>
        )}
        <button type="button" className="ms-decisions-row-open" aria-expanded={open} onClick={onOpen}>
          <span className="ms-decisions-row-said">{rowLine(need)}</span>
          <span className="ms-decisions-row-facts">
            {facts.yours && <Chip>yours</Chip>}
            {facts.tier > 0 && <Chip>tier {facts.tier}</Chip>}
            {facts.labels.map((label) => (
              <Chip key={label}>{label}</Chip>
            ))}
            {facts.age !== "" && <span className="ms-decisions-row-age">{facts.age}</span>}
            {need.new && (
              <>
                <span className="ms-decisions-dot" aria-hidden="true" />
                <span className="ms-visually-hidden">new</span>
              </>
            )}
          </span>
        </button>
      </div>
      {open && <OpenRow need={need} now={now} acts={acts} />}
    </li>
  );
}

/**
 * What one open row holds: the whole substance of that kind of thing, the
 * sentence saying what happens if nobody answers, and the acts.
 */
function OpenRow({ need, now, acts }: { need: Need; now: Date; acts: Acts }) {
  return (
    <div className="ms-decisions-open">
      <p className="ms-mono ms-decisions-open-id">{need.id}</p>
      <Substance need={need} />
      <p className="ms-decisions-open-muted">{askedLine(need, now)}</p>
      {need.recommend !== "" && <p className="ms-decisions-open-recommend">Recommended: {need.recommend}</p>}
      {acts.refusedAt === need.id && acts.refusal !== "" && (
        <p className="ms-decisions-refusal" role="alert">
          {acts.refusal}
        </p>
      )}
      <ActRow need={need} acts={acts} />
    </div>
  );
}

/** The whole of what this kind of thing is, in the record's own words. */
function Substance({ need }: { need: Need }) {
  switch (need.kind) {
    case "question":
      return <p className="ms-decisions-open-words">{need.asked}</p>;
    case "ruling-review":
      return (
        <>
          <p className="ms-decisions-open-words">{need.words === "" ? need.asked : need.words}</p>
          {need.context !== "" && (
            <details className="ms-decisions-context">
              <summary className="ms-decisions-context-line">Context</summary>
              <p className="ms-decisions-context-words">{need.context}</p>
            </details>
          )}
          <p className="ms-decisions-open-facts">
            {need.owner !== "" && <span>owner {need.owner}</span>}
            {need.class !== "" && <span>{need.class}</span>}
            {need.due !== "" && <span>review due {need.due}</span>}
          </p>
        </>
      );
    case "draft":
    case "landed":
      return (
        <>
          <p className="ms-decisions-open-words">{need.title === "" ? need.id : need.title}</p>
          {need.path !== "" && <p className="ms-mono ms-decisions-open-path">{need.path}</p>}
          {need.goals.length > 0 && (
            <ul className="ms-decisions-open-goals">
              {need.goals.map((goal) => (
                <li key={goal.id} className="ms-mono ms-decisions-open-goal">
                  {goalLine(goal)}
                </li>
              ))}
            </ul>
          )}
        </>
      );
    case "approval":
    case "renewal":
      return <GoalSubstance need={need} />;
    default:
      // A seat's park, a stopped goal, an alert and a seat's ask carry no
      // backlog row, so what they are is the sentence the server composed out
      // of the record: the explanation, exactly as today.
      return <p className="ms-decisions-open-words">{need.asked}</p>;
  }
}

/**
 * A goal, from its own row: what is meant to be done, what to do first, the
 * words the board narrows by, where it stands, and what the approval would
 * carry. Nothing here is judged — the row is the projection's.
 */
function GoalSubstance({ need }: { need: Need }) {
  const row = need.row;
  if (row === null) {
    // A renewal on claimed work carries no row: the engine refuses a budget
    // on claimed work, so there is nothing to prefill and nothing to show but
    // what was asked.
    return <p className="ms-decisions-open-words">{need.asked}</p>;
  }
  return (
    <>
      <p className="ms-decisions-open-words">{row.intent}</p>
      {row.nextStep !== "" && <p className="ms-decisions-open-next">Next step: {row.nextStep}</p>}
      <p className="ms-decisions-open-facts">
        <span>{budgetLine(need)}</span>
        <span>{bandLine(need)}</span>
        {row.tier > 0 && <span>tier {row.tier}</span>}
        <span>{row.origin === "human" ? "yours" : "opened by a seat"}</span>
        {blockedLine(need) !== "" && <span>{blockedLine(need)}</span>}
      </p>
      {row.labels.length > 0 && (
        <p className="ms-decisions-open-labels">
          {row.labels.map((label) => (
            <Chip key={label}>{label}</Chip>
          ))}
        </p>
      )}
    </>
  );
}

/**
 * The acts of one open row, and the way through beside them.
 *
 * A seat that nothing proves a human on cannot publish, so the acts are
 * replaced by the one act that is open to it: signing in. The way through is
 * not an act and stays — reading the register does not need a proof.
 */
function ActRow({ need, acts }: { need: Need; acts: Acts }) {
  const offered = <Offered need={need} acts={acts} />;
  return (
    <div className="ms-decisions-open-acts">
      {acts.signedIn ? (
        offered
      ) : (
        <Button
          onClick={() => {
            acts.onSignIn();
          }}
        >
          Sign in to act
        </Button>
      )}
      <Way where={need.where} />
      {need.command !== "" && <code className="ms-mono ms-decisions-command">{need.command}</code>}
    </div>
  );
}

function Offered({ need, acts }: { need: Need; acts: Acts }): ReactNode {
  switch (need.kind) {
    case "draft":
    case "landed":
      return <WriteStatus need={need} acts={acts} />;
    case "approval":
      return (
        <>
          <Button
            primary
            onClick={() => {
              acts.onApprove(need);
            }}
          >
            Approve
          </Button>
          <Button
            onClick={() => {
              acts.onPark(need);
            }}
          >
            Not now
          </Button>
          <Button
            onClick={() => {
              acts.onEdit(need);
            }}
          >
            Edit
          </Button>
        </>
      );
    case "renewal":
      // Only the unclaimed one is offered the sheet: the engine refuses a
      // budget on claimed work, so a sheet prefilled with one would be a form
      // it throws away. The claimed one says so in its silence line and its
      // way through goes to the goal.
      return need.act === "approve" && need.row !== null ? (
        <Button
          primary
          onClick={() => {
            acts.onApprove(need);
          }}
        >
          Approve
        </Button>
      ) : null;
    case "parked":
      return (
        <>
          <Button
            onClick={() => {
              acts.onReturn(need.id);
            }}
          >
            Return to queue
          </Button>
          <Help id="return-to-queue" />
        </>
      );
    default:
      // A question, a ruling review, an alert, a seat's ask and a stopped
      // goal are all decided somewhere this interface does not publish to.
      // The way through beside this is the whole of the act.
      return null;
  }
}

/**
 * Accepting a draft, and marking a landed design done: one write to the
 * record's Status line, through the route the reader's own act writes through.
 *
 * The confirmation is the whole of its safety, so it is one line that names
 * the file and the word that will be written, and it stands where the button
 * was rather than over the page: a human who reads it knows exactly what the
 * file will say.
 */
function WriteStatus({ need, acts }: { need: Need; acts: Acts }) {
  const [asking, setAsking] = useState(false);
  const status = writeStatus(need.kind);
  if (acts.writing === need.id) {
    return <span className="ms-decisions-open-busy">Writing…</span>;
  }
  if (!asking) {
    return (
      <Button
        primary
        onClick={() => {
          setAsking(true);
        }}
      >
        {writeLabel(need.kind)}
      </Button>
    );
  }
  return (
    <span className="ms-decisions-confirm">
      <span className="ms-decisions-confirm-line">{confirmLine(need, status)}</span>
      <Button
        primary
        onClick={() => {
          setAsking(false);
          acts.onWrite(need, status);
        }}
      >
        {writeLabel(need.kind)}
      </Button>
      <Button
        onClick={() => {
          setAsking(false);
        }}
      >
        Cancel
      </Button>
    </span>
  );
}

/**
 * Where a row's decision is made, where it is not an act of this interface: a
 * link where the destination is an address, a button where it is a panel, and
 * plain words where this build has no surface for it — which is the master's
 * rule for an unresolved reference, rather than a link that would refuse.
 */
export function Way({ where }: { where: Reference }) {
  const { openPanel } = useNotifications();
  const destination = destinationFor(where);
  const words = wayThrough(where);
  if (words === "") {
    return null;
  }
  if (destination.kind === "link") {
    return (
      <NavLink className="ms-decisions-way" to={destination.to}>
        {words}
      </NavLink>
    );
  }
  if (destination.kind === "notifications") {
    return (
      <button
        type="button"
        className="ms-decisions-way"
        onClick={() => {
          openPanel(destination.at === "" ? undefined : destination.at);
        }}
      >
        {words}
      </button>
    );
  }
  return <span className="ms-decisions-way ms-decisions-way--words">{words}</span>;
}
