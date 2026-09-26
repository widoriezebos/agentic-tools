import { CircleCheck } from "lucide-react";

import type { Need } from "./api";
import { freshLine, type Group, type GroupId } from "./groups";
import { InboxRow, type Acts } from "./InboxRow";
import { QueueBlock } from "./QueueBlock";
import type { Narrowing } from "./decisions";

/**
 * The inbox: one line per group, and one group open.
 *
 * The page at rest is those lines and nothing else. A human arriving asks one
 * question — is anything waiting on me, and how much — and ten lines answer it
 * in two seconds: what kinds, how many of each, how old the newest one is, and
 * how much of it arrived since they were last here. What stays as it is when
 * ignored looks like it: present, quiet, collapsed.
 *
 * One group is open at a time, and opening a second closes the first. That is
 * the whole of the rule: a page where every group could be open is the wall
 * this replaced.
 */
export function Inbox({
  groups,
  open,
  onOpen,
  openRow,
  onOpenRow,
  acts,
  narrowing,
  onNarrow,
  selected,
  onSelect,
  onBulk,
  now,
}: {
  groups: Group[];
  /** The one group open, or null where this inbox has no group at all. */
  open: GroupId | null;
  onOpen: (id: GroupId) => void;
  /** The one row open, across every group: opening a second closes the first. */
  openRow: string;
  onOpenRow: (id: string) => void;
  acts: Acts;
  narrowing: Narrowing;
  onNarrow: (next: Narrowing) => void;
  selected: readonly string[];
  onSelect: (ids: string[]) => void;
  onBulk: (act: "approve" | "park", goals: Need[]) => void;
  now: Date;
}) {
  if (groups.length === 0) {
    return (
      <p className="ms-decisions-calm">
        <CircleCheck className="ms-decisions-calm-icon" size={16} strokeWidth={1.75} aria-hidden="true" />
        Nothing is waiting on you.
      </p>
    );
  }
  return (
    <ul className="ms-decisions-groups">
      {groups.map((group) => (
        <li
          key={group.id}
          className={open === group.id ? "ms-decisions-group ms-decisions-group--open" : "ms-decisions-group"}
        >
          <GroupLine
            group={group}
            open={open === group.id}
            onOpen={() => {
              onOpen(group.id);
            }}
          />
          {open === group.id &&
            (group.id === "queue" ? (
              <QueueBlock
                waiting={group.needs}
                narrowing={narrowing}
                onNarrow={onNarrow}
                selected={selected}
                onSelect={onSelect}
                opened={openRow}
                onOpen={onOpenRow}
                onBulk={onBulk}
                acts={acts}
                now={now}
              />
            ) : (
              <ul className="ms-decisions-rows">
                {group.needs.map((need) => (
                  <InboxRow
                    key={`${need.kind}-${need.id}`}
                    need={need}
                    now={now}
                    acts={acts}
                    open={openRow === need.id}
                    onOpen={() => {
                      onOpenRow(openRow === need.id ? "" : need.id);
                    }}
                  />
                ))}
              </ul>
            ))}
        </li>
      ))}
    </ul>
  );
}

/**
 * One group's line: its name, how many, how old the newest one is, and how
 * many are new.
 *
 * A quiet group says what it does when nobody answers instead of an age. A
 * ruling past its review date stays in force exactly as written until a human
 * says otherwise, and "3 weeks" on that line is a page shouting about
 * something that is not going anywhere.
 */
function GroupLine({ group, open, onOpen }: { group: Group; open: boolean; onOpen: () => void }) {
  const fresh = freshLine(group);
  return (
    <button type="button" className="ms-decisions-group-line" aria-expanded={open} onClick={onOpen}>
      <span className="ms-decisions-group-name">{group.title}</span>
      <span className="ms-decisions-group-count">{group.count}</span>
      <span className="ms-decisions-group-when">{group.standing === "" ? group.newest : group.standing}</span>
      {fresh !== "" && <span className="ms-decisions-group-new">{fresh}</span>}
    </button>
  );
}
