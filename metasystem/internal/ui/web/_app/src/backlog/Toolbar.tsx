import { RefreshCw } from "lucide-react";
import { useId } from "react";

import {
  anySet,
  named,
  nameOf,
  noFilters,
  ANY,
  NONE,
  RANKS,
  type Filters,
  type Rank,
} from "./filters";
import { clockTime } from "./format";
import type { Sync } from "./sync";
import { Help } from "../help/Help";
import type { HelpId } from "../help/terms";
import type { BacklogView } from "../storage";
import { Button, Hint, IconButton, Skeleton } from "../shell/controls";

/**
 * The one row above the lanes.
 *
 * Everything that was stacked over the board is here, in one line, in the
 * order a human uses it: how the backlog is read, what it is narrowed to,
 * and — at the far end, where a control that acts belongs — whether the page
 * is current, the ask that reads it again, and the one act that starts from
 * no card.
 *
 * What used to stand here was a ledger report two lines deep, a row holding
 * the switch, a banner about proof, a filter row, and a line about goals this
 * build cannot place: five bands of furniture between the top of the work
 * area and the first lane title. Four of them said the same thing on every
 * board, every day. They are a chip, a tooltip, and a disclosure under the
 * lanes now, and the lanes start where the work area does.
 *
 * The row wraps rather than scrolls or hides: at a phone's width the filters
 * take a line of their own under the switch, and nothing is past an edge.
 */

/** The switch and the one act, offered only where a ledger was read. */
export type Reading = {
  view: BacklogView;
  onView: (view: BacklogView) => void;
  onNew: () => void;
};

/** What the board is narrowed to, offered only where there is a board. */
export type Narrowing = {
  filters: Filters;
  onFilters: (filters: Filters) => void;
  seats: string[];
  arcs: string[];
};

export function BoardToolbar({
  reading,
  narrowing,
  sync,
  observedAt,
  onRefresh,
}: {
  reading: Reading | null;
  narrowing: Narrowing | null;
  /** What the last read found, or null before there has been one. */
  sync: Sync | null;
  observedAt: string;
  onRefresh: () => void;
}) {
  return (
    <div className="ms-board-toolbar">
      {reading !== null && (
        <div className="ms-board-views" role="group" aria-label="How the backlog is read">
          <Button
            aria-pressed={reading.view === "board"}
            onClick={() => {
              reading.onView("board");
            }}
          >
            Board
          </Button>
          <Button
            aria-pressed={reading.view === "list"}
            onClick={() => {
              reading.onView("list");
            }}
          >
            List
          </Button>
        </div>
      )}
      {narrowing !== null && (
        <FilterBar
          filters={narrowing.filters}
          onChange={narrowing.onFilters}
          seats={narrowing.seats}
          arcs={narrowing.arcs}
        />
      )}
      <div className="ms-board-toolbar-end">
        {sync === null ? <Skeleton /> : <SyncChip sync={sync} />}
        {/* The ask, not a sentence about it: the icon is the control, its
            name is Refresh, and the tooltip says when this page last looked
            so that the answer to "is this current" is in the same place as
            the way to make it current. */}
        <IconButton label="Refresh" hint={`Observed ${clockTime(observedAt)} · Refresh`} onClick={onRefresh}>
          <RefreshCw size={16} strokeWidth={1.75} aria-hidden="true" />
        </IconButton>
        {/* Intake is the one act with no card to start from, so it stands at
            the end of the toolbar rather than on the board. It is never
            disabled for want of proof: the act asks, and a server that finds
            no human behind it answers with the sign-in this page can open. */}
        {reading !== null && (
          <Button primary onClick={reading.onNew}>
            New goal
          </Button>
        )}
      </div>
    </div>
  );
}

/**
 * Whether the page is reading a current ledger, in three or four words.
 *
 * At rest it is muted and says only when this page read and which tip it
 * read: a human who wanted more asks the chip, and the tooltip carries the
 * whole report. Behind wears the marker colour and says since when the
 * server last heard from the canonical branch, which is the one fact the
 * short line cannot leave out. Wrong wears the danger colour, and the pane
 * puts a line under the toolbar saying what to do, because that is the only
 * state anything can be done about.
 *
 * All three are the server's judgement of its own fetch loop, carried here
 * whole. The chip measures nothing: a chip that measured the accepted
 * commit's age would wear the marker over a quiet week and never take it off,
 * however often a human pressed Refresh.
 */
export function SyncChip({ sync }: { sync: Sync }) {
  return (
    <Hint label={sync.report}>
      {/* The chip is the tooltip's trigger and carries no other act, so it
          takes a tab stop of its own: the report has to be reachable without
          a pointer. */}
      <span className={`ms-sync ms-sync--${sync.state}`} tabIndex={0}>
        {sync.line}
      </span>
    </Hint>
  );
}

/* ------------------------------------------------------------ narrowing -- */

/**
 * What the board is narrowed to, in the toolbar where it acts: it narrows the
 * board and nothing above it.
 *
 * The seat and the arc offer what is on this board rather than what the fleet
 * or the plan could hold: a select offering a seat no card carries offers an
 * empty board. A value the browser remembered that nothing on the board
 * carries any more is offered all the same, as itself, so that a lane showing
 * nothing shows why and "clear" is one press away — a filter that widened
 * itself would be the board deciding what a human meant.
 */
function FilterBar({
  filters,
  onChange,
  seats,
  arcs,
}: {
  filters: Filters;
  onChange: (filters: Filters) => void;
  seats: string[];
  arcs: string[];
}) {
  const text = useId();
  return (
    <div className="ms-board-filters" role="group" aria-label="Narrow the board">
      <div className="ms-board-filter">
        <label htmlFor={text}>Find</label>
        <input
          id={text}
          type="search"
          className="ms-board-find"
          value={filters.text}
          placeholder="id or intent"
          onChange={(event) => {
            onChange({ ...filters, text: event.target.value });
          }}
        />
      </div>
      {/* Four of the five filters narrow the board by something this project
          worked out for itself — a band and a position, a tier of proof, the
          seat that claimed it, the arc it belongs to — so each label says
          what it is narrowing by. Find narrows by the words on the card and
          needs no explaining. */}
      <Choose
        label="Priority"
        help="priority"
        value={filters.priority}
        options={RANKS.map((rank) => ({ value: rank, title: rank }))}
        onChange={(value) => {
          onChange({ ...filters, priority: value as Rank });
        }}
      />
      <Choose
        label="Tier"
        help="tier"
        value={filters.tier}
        options={RANKS.map((rank) => ({ value: rank, title: rank }))}
        onChange={(value) => {
          onChange({ ...filters, tier: value as Rank });
        }}
      />
      <Choose
        label="Seat"
        help="seat"
        value={filters.seat}
        options={[{ value: NONE, title: "unassigned" }, ...offered(seats, filters.seat)]}
        onChange={(value) => {
          onChange({ ...filters, seat: value });
        }}
      />
      <Choose
        label="Arc"
        help="arc"
        value={filters.arc}
        options={[{ value: NONE, title: "none" }, ...offered(arcs, filters.arc)]}
        onChange={(value) => {
          onChange({ ...filters, arc: value });
        }}
      />
      {anySet(filters) && (
        <button
          type="button"
          className="ms-project-act"
          onClick={() => {
            onChange(noFilters);
          }}
        >
          clear
        </button>
      )}
    </div>
  );
}

/**
 * The named values a select offers: what is on the board, plus whatever is
 * chosen, so that a choice is never silently dropped from the list that
 * explains it.
 */
function offered(present: string[], chosen: string): { value: string; title: string }[] {
  const name = nameOf(chosen);
  const names = name !== null && !present.includes(name) ? [...present, name] : present;
  return names.map((value) => ({ value: named(value), title: value }));
}

function Choose({
  label,
  help,
  value,
  options,
  onChange,
}: {
  label: string;
  /** The term the label names, where the label is one of this project's own. */
  help?: HelpId;
  value: string;
  options: { value: string; title: string }[];
  onChange: (value: string) => void;
}) {
  const named_ = useId();
  return (
    <div className="ms-board-filter">
      <label htmlFor={named_}>{label}</label>
      {help !== undefined && <Help id={help} />}
      <select
        id={named_}
        className="ms-board-select"
        value={value}
        onChange={(event) => {
          onChange(event.target.value);
        }}
      >
        <option value={ANY}>any</option>
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.title}
          </option>
        ))}
      </select>
    </div>
  );
}
