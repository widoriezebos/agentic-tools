import { useMemo, useState } from "react";

import type { Need } from "./api";
import {
  ANY_ORIGIN,
  isNarrowed,
  labelsIn,
  noNarrowing,
  queueCount,
  SEATS,
  shownQueue,
  YOURS,
  type Narrowing,
} from "./decisions";
import { InboxRow, type Acts } from "./InboxRow";
import { Help } from "../help/Help";
import { Button } from "../shell/controls";

/**
 * The queue group: the goals nobody has authorized.
 *
 * It is the one group that is worked rather than read. There are a hundred
 * rows and they are mostly weeks old, so what this is for is a sitting:
 * narrowing to a family, opening the one that needs a second look, saying yes
 * or not now to one — or to twenty — and editing a wording on the way.
 *
 * Three things make it that rather than a wall. The tools are one line in the
 * group's head, so the rows start at the top of the screen. The rows carry no
 * buttons: a hundred rows of two buttons each was two hundred and forty
 * buttons on one page, and the acts belong where a human has decided, which is
 * in the open row. And the bulk acts are a bar at the foot that appears when
 * something is ticked, rather than a row of controls that is disabled most of
 * the time.
 *
 * Nothing here is persisted. A narrowing is a sitting's state, and a human who
 * came back tomorrow to a queue still narrowed to one label would be reading a
 * queue that is quietly missing most of itself.
 */
export function QueueBlock({
  waiting,
  narrowing,
  onNarrow,
  selected,
  onSelect,
  opened,
  onOpen,
  onBulk,
  acts,
  now,
}: {
  waiting: readonly Need[];
  narrowing: Narrowing;
  onNarrow: (next: Narrowing) => void;
  /** The ids a human has ticked, which may include rows now narrowed away. */
  selected: readonly string[];
  onSelect: (ids: string[]) => void;
  /** The one row open in this group, or "". */
  opened: string;
  onOpen: (id: string) => void;
  onBulk: (act: "approve" | "park", goals: Need[]) => void;
  acts: Acts;
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
    <div className="ms-decisions-queue">
      <Tools
        narrowing={narrowing}
        onNarrow={onNarrow}
        labels={labels}
        onClear={() => {
          onNarrow(noNarrowing);
        }}
        count={queueCount(waiting.length, shown.length)}
      />
      {shown.length === 0 ? (
        <p className="ms-decisions-none">No goal in the queue matches.</p>
      ) : (
        <>
          <label className="ms-decisions-check ms-decisions-all">
            <input
              type="checkbox"
              checked={allShown}
              onChange={() => {
                onSelect(allShown ? [] : shown.map((need) => need.id));
              }}
            />
            Select all shown
          </label>
          <ul className="ms-decisions-rows">
            {shown.map((need) => (
              <InboxRow
                key={need.id}
                need={need}
                now={now}
                acts={acts}
                open={opened === need.id}
                onOpen={() => {
                  onOpen(opened === need.id ? "" : need.id);
                }}
                ticked={selected.includes(need.id)}
                onTick={() => {
                  onSelect(
                    selected.includes(need.id)
                      ? selected.filter((id) => id !== need.id)
                      : [...selected, need.id],
                  );
                }}
              />
            ))}
          </ul>
        </>
      )}
      {acting.length > 0 && (
        <SelectionBar
          selected={acting}
          signedIn={acts.signedIn}
          onBulk={onBulk}
          onClear={() => {
            onSelect([]);
          }}
        />
      )}
    </div>
  );
}

/**
 * The bar at the group's foot: how many are ticked, and the two acts over
 * them.
 *
 * It is the only place the bulk acts live, and it is not there at all until
 * something is ticked — a control that spends its life disabled teaches a
 * human to stop reading that part of the screen.
 */
function SelectionBar({
  selected,
  signedIn,
  onBulk,
  onClear,
}: {
  selected: Need[];
  signedIn: boolean;
  onBulk: (act: "approve" | "park", goals: Need[]) => void;
  onClear: () => void;
}) {
  return (
    <div className="ms-decisions-bar" role="group" aria-label="What you selected">
      <span className="ms-decisions-bar-count">{selected.length} selected</span>
      <Help id="act-selected" />
      <Button
        primary
        disabled={!signedIn}
        onClick={() => {
          onBulk("approve", selected);
        }}
      >
        Approve
      </Button>
      <Button
        disabled={!signedIn}
        onClick={() => {
          onBulk("park", selected);
        }}
      >
        Not now
      </Button>
      <Button onClick={onClear}>Clear</Button>
    </div>
  );
}

/**
 * The three tools, on one line in the group's head: find, narrow to one family
 * or one origin, and choose between the backlog's order and the newest work.
 *
 * One label at a time. A human triaging is asking "what is there of this
 * kind", and two labels at once answers a question nobody asked.
 */
function Tools({
  narrowing,
  onNarrow,
  labels,
  onClear,
  count,
}: {
  narrowing: Narrowing;
  onNarrow: (next: Narrowing) => void;
  labels: { label: string; count: number }[];
  onClear: () => void;
  count: string;
}) {
  return (
    <div className="ms-decisions-tools">
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
      <Labels narrowing={narrowing} onNarrow={onNarrow} labels={labels} />
      {isNarrowed(narrowing) && <Button onClick={onClear}>Clear</Button>}
      <span className="ms-decisions-tools-count">{count}</span>
      <Help id="waiting-for-approval" />
    </div>
  );
}

/**
 * The label chips, drawn from the rows on screen, commonest first.
 *
 * A queue with twenty families has twenty chips, which is a second wall on the
 * line that exists to prevent one, so the line carries the four commonest and
 * opens the rest on asking.
 */
const SHOWN_CHIPS = 4;

function Labels({
  narrowing,
  onNarrow,
  labels,
}: {
  narrowing: Narrowing;
  onNarrow: (next: Narrowing) => void;
  labels: { label: string; count: number }[];
}) {
  const [all, setAll] = useState(false);
  const shown = all ? labels : labels.slice(0, SHOWN_CHIPS);
  const more = labels.length - shown.length;
  return (
    <>
      {shown.map((one) => (
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
      {more > 0 && (
        <ChipButton
          on={false}
          onPress={() => {
            setAll(true);
          }}
        >
          +{more}
        </ChipButton>
      )}
    </>
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

/** The narrowing the pane starts every visit with. */
export function useNarrowing(): [Narrowing, (next: Narrowing) => void] {
  const [narrowing, setNarrowing] = useState<Narrowing>(noNarrowing);
  return [narrowing, setNarrowing];
}
