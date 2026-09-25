import { useMemo, useState } from "react";
import { NavLink } from "react-router";

import type { Need } from "./api";
import {
  ANY_ORIGIN,
  actLabelFor,
  ageLine,
  bandLine,
  blockedLine,
  budgetLine,
  isNarrowed,
  labelsIn,
  noNarrowing,
  queueCount,
  rowLabels,
  SEATS,
  shownQueue,
  YOURS,
  type Narrowing,
} from "./decisions";
import { Help } from "../help/Help";
import { goalPath } from "../routes";
import { Button, Chip } from "../shell/controls";

/**
 * Waiting for your approval: a queue a human works, not a list they read.
 *
 * The rows are the goals nobody has authorized. There are a hundred of them
 * and they are mostly weeks old, so what this block is for is working through
 * them in a sitting: narrowing to a family, opening the one that needs a
 * second look, and saying yes — or not now — to several at once.
 *
 * The silence is said once, in the head, because it is the same sentence for
 * every row here. The derived "Approve X for execution" sentence is not shown
 * at all: it said the title twice.
 *
 * Nothing here is persisted. A narrowing is a sitting's state, and a human
 * who comes back tomorrow to a queue still narrowed to one label would be
 * reading a queue that is quietly missing most of itself.
 */
export function QueueBlock({
  waiting,
  signedIn,
  narrowing,
  onNarrow,
  selected,
  onSelect,
  opened,
  onOpen,
  onAct,
  onBulk,
  now,
}: {
  waiting: readonly Need[];
  signedIn: boolean;
  narrowing: Narrowing;
  onNarrow: (next: Narrowing) => void;
  /** The ids a human has ticked, which may include rows now narrowed away. */
  selected: readonly string[];
  onSelect: (ids: string[]) => void;
  opened: readonly string[];
  onOpen: (ids: string[]) => void;
  onAct: (need: Need, act: "approve" | "park") => void;
  onBulk: (act: "approve" | "park", goals: Need[]) => void;
  now: Date;
}) {
  const shown = useMemo(() => shownQueue(waiting, narrowing), [waiting, narrowing]);
  const labels = useMemo(() => labelsIn(waiting), [waiting]);
  // What a bulk act would act on is what is ticked AND on screen: a human who
  // narrows after ticking is looking at a different set, and acting on rows
  // they can no longer see is the one thing a queue must never do.
  const acting = useMemo(() => shown.filter((need) => selected.includes(need.id)), [shown, selected]);
  const allShown = shown.length > 0 && acting.length === shown.length;

  return (
    <section className="ms-decisions-block" id="decisions-waiting">
      <div className="ms-decisions-head">
        <h2 className="ms-decisions-title">Waiting for your approval</h2>
        <Help id="waiting-for-approval" />
        <Help id="silence" />
        <span className="ms-decisions-count">{queueCount(waiting.length, shown.length)}</span>
      </div>
      <p className="ms-decisions-quiet">
        If you do nothing, these stay in To Do and no seat may claim them.
      </p>
      {waiting.length === 0 ? (
        <p className="ms-decisions-none">Nothing waits for your approval.</p>
      ) : (
        <>
          <Tools
            narrowing={narrowing}
            onNarrow={onNarrow}
            labels={labels}
            onClear={() => {
              onNarrow(noNarrowing);
            }}
          />
          <div className="ms-decisions-bulk">
            <label className="ms-decisions-check">
              <input
                type="checkbox"
                checked={allShown}
                disabled={shown.length === 0}
                onChange={() => {
                  onSelect(allShown ? [] : shown.map((need) => need.id));
                }}
              />
              Select all shown
            </label>
            <Help id="act-selected" />
            <Button
              primary
              disabled={acting.length === 0}
              onClick={() => {
                onBulk("approve", acting);
              }}
            >
              {actLabelFor("approve", acting.length)}
            </Button>
            <Button
              disabled={acting.length === 0 || !signedIn}
              onClick={() => {
                onBulk("park", acting);
              }}
            >
              {actLabelFor("park", acting.length)}
            </Button>
          </div>
          {shown.length === 0 ? (
            <p className="ms-decisions-none">No goal in the queue matches.</p>
          ) : (
            <ul className="ms-decisions-queue">
              {shown.map((need) => (
                <QueueRow
                  key={need.id}
                  need={need}
                  now={now}
                  signedIn={signedIn}
                  ticked={selected.includes(need.id)}
                  onTick={() => {
                    onSelect(
                      selected.includes(need.id)
                        ? selected.filter((id) => id !== need.id)
                        : [...selected, need.id],
                    );
                  }}
                  open={opened.includes(need.id)}
                  onOpen={() => {
                    onOpen(
                      opened.includes(need.id)
                        ? opened.filter((id) => id !== need.id)
                        : [...opened, need.id],
                    );
                  }}
                  onAct={onAct}
                />
              ))}
            </ul>
          )}
        </>
      )}
    </section>
  );
}

/**
 * The three tools, and no more: find, narrow to one family or one origin, and
 * choose between the backlog's order and the newest work.
 *
 * One label at a time. A human triaging is asking "what is there of this
 * kind", and two labels at once answers a question nobody asked.
 */
function Tools({
  narrowing,
  onNarrow,
  labels,
  onClear,
}: {
  narrowing: Narrowing;
  onNarrow: (next: Narrowing) => void;
  labels: { label: string; count: number }[];
  onClear: () => void;
}) {
  return (
    <div className="ms-decisions-tools">
      <div className="ms-decisions-find">
        <label className="ms-visually-hidden" htmlFor="ms-decisions-queue-find">
          Find in the queue
        </label>
        <input
          id="ms-decisions-queue-find"
          type="search"
          className="ms-decisions-find-field"
          placeholder="Find in the id, the intent and the labels"
          value={narrowing.find}
          onChange={(event) => {
            onNarrow({ ...narrowing, find: event.target.value });
          }}
        />
        <label className="ms-visually-hidden" htmlFor="ms-decisions-order">
          Order the queue
        </label>
        <select
          id="ms-decisions-order"
          className="ms-decisions-class"
          value={narrowing.order}
          onChange={(event) => {
            onNarrow({ ...narrowing, order: event.target.value === "newest" ? "newest" : "backlog" });
          }}
        >
          <option value="backlog">Backlog order</option>
          <option value="newest">Newest first</option>
        </select>
        {isNarrowed(narrowing) && <Button onClick={onClear}>Clear</Button>}
      </div>
      <div className="ms-decisions-chips">
        <ChipButton
          on={narrowing.origin === YOURS}
          onPress={() => {
            onNarrow({ ...narrowing, origin: narrowing.origin === YOURS ? ANY_ORIGIN : YOURS });
          }}
        >
          yours
        </ChipButton>
        <ChipButton
          on={narrowing.origin === SEATS}
          onPress={() => {
            onNarrow({ ...narrowing, origin: narrowing.origin === SEATS ? ANY_ORIGIN : SEATS });
          }}
        >
          seats&apos;
        </ChipButton>
        {labels.map((one) => (
          <ChipButton
            key={one.label}
            on={narrowing.label === one.label}
            onPress={() => {
              onNarrow({ ...narrowing, label: narrowing.label === one.label ? "" : one.label });
            }}
          >
            {one.label} {one.count}
          </ChipButton>
        ))}
      </div>
    </div>
  );
}

function ChipButton({
  on,
  onPress,
  children,
}: {
  on: boolean;
  onPress: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      className={on ? "ms-decisions-chip ms-decisions-chip--on" : "ms-decisions-chip"}
      aria-pressed={on}
      onClick={onPress}
    >
      {children}
    </button>
  );
}

/**
 * One queue row: one line that opens.
 *
 * The title is the disclosure, so the thing a human reads is the thing they
 * press to see more of it. What opens is what would let them decide and is
 * not worth a hundred rows of vertical space: the whole intent, the next
 * step, the budget the approval would carry, what is holding it up, where it
 * stands in the backlog, and the way to the goal itself.
 */
function QueueRow({
  need,
  now,
  signedIn,
  ticked,
  onTick,
  open,
  onOpen,
  onAct,
}: {
  need: Need;
  now: Date;
  signedIn: boolean;
  ticked: boolean;
  onTick: () => void;
  open: boolean;
  onOpen: () => void;
  onAct: (need: Need, act: "approve" | "park") => void;
}) {
  const row = need.row;
  const labels = rowLabels(need);
  const age = ageLine(need.since, now);
  return (
    <li className="ms-decisions-queue-row">
      <div className="ms-decisions-queue-line">
        <label className="ms-decisions-check">
          <input type="checkbox" checked={ticked} onChange={onTick} />
          <span className="ms-visually-hidden">Select {need.id}</span>
        </label>
        <button
          type="button"
          className="ms-decisions-queue-open"
          aria-expanded={open}
          onClick={onOpen}
        >
          <span className="ms-mono ms-decisions-queue-id">{need.id}</span>
          <span className="ms-decisions-queue-title">{need.title === "" ? need.id : need.title}</span>
        </button>
        <span className="ms-decisions-queue-facts">
          {age !== "" && <span className="ms-decisions-queue-age">{age}</span>}
          {row !== null && row.tier > 0 && <Chip>tier {row.tier}</Chip>}
          {row !== null && row.origin === YOURS && <Chip>yours</Chip>}
          {labels.shown.map((label) => (
            <Chip key={label}>{label}</Chip>
          ))}
          {labels.more > 0 && <Chip>+{labels.more}</Chip>}
        </span>
        <span className="ms-decisions-queue-acts">
          <Button
            primary
            onClick={() => {
              onAct(need, "approve");
            }}
          >
            Approve
          </Button>
          <Button
            disabled={!signedIn}
            onClick={() => {
              onAct(need, "park");
            }}
          >
            Not now
          </Button>
        </span>
      </div>
      {open && (
        <div className="ms-decisions-queue-more">
          <p className="ms-decisions-queue-intent">{row === null ? need.asked : row.intent}</p>
          {row !== null && row.nextStep !== "" && (
            <p className="ms-decisions-queue-next">Next step: {row.nextStep}</p>
          )}
          <p className="ms-decisions-queue-facts-more">
            <span>{budgetLine(need)}</span>
            <span>{bandLine(need)}</span>
            {blockedLine(need) !== "" && <span>{blockedLine(need)}</span>}
          </p>
          <NavLink className="ms-decisions-way" to={goalPath(need.id)}>
            Open the goal
          </NavLink>
        </div>
      )}
    </li>
  );
}

/** The narrowing the pane starts every visit with. */
export function useNarrowing(): [Narrowing, (next: Narrowing) => void] {
  const [narrowing, setNarrowing] = useState<Narrowing>(noNarrowing);
  return [narrowing, setNarrowing];
}
