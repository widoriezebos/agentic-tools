import { useMemo, useState } from "react";

import type { Need } from "./api";
import { InboxRow, type Acts } from "./InboxRow";
import { budgetsFrom, excludedInBulk, notRunIn, NOTHING_SENDABLE, type Applying } from "./proposals";
import type { Backlog } from "../backlog/api";
import { Panel } from "../backlog/Panel";
import { ProposalSubstance } from "../partner/Proposal";
import { APPLY, CONTINUE, DISMISS, type Line } from "../partner/proposing";
import { Help } from "../help/Help";
import { Button } from "../shell/controls";

/**
 * The group the Partner's proposals wait in, worked rather than read one row at
 * a time.
 *
 * It is the queue's own shape, for the queue's own reason: a human who asked the
 * Partner to put six goals away came here to answer six things, and six rows of
 * two buttons each is the wall this page was built to replace. So the rows carry
 * a checkbox and nothing else, and the acts over many are a bar at the foot that
 * is not there at all until something is ticked.
 *
 * What the bar acts on is what is ticked AND on screen, which is every row of
 * this group: acting on a row a human can no longer see is the one thing a list
 * of pending acts must never do.
 */
export function ProposalBlock({
  needs,
  selected,
  onSelect,
  opened,
  onOpen,
  acts,
  now,
}: {
  needs: readonly Need[];
  /** The ids a human has ticked, which may include rows of another group. */
  selected: readonly string[];
  onSelect: (ids: string[]) => void;
  /** The one row open in this group, or "". */
  opened: string;
  onOpen: (id: string) => void;
  acts: Acts;
  now: Date;
}) {
  const ticked = useMemo(() => needs.filter((need) => selected.includes(need.id)), [needs, selected]);
  const all = needs.length > 0 && ticked.length === needs.length;
  // What a stopped run never reached. The run stops at an answer that does not
  // say what happened, because the act may have landed and the next line may
  // depend on it; the lines after it were never sent, and this is the one press
  // that sends them (Sol S60-C-02).
  const stopped = useMemo(
    () => notRunIn(lineFor(needs, acts)),
    [needs, acts],
  );
  return (
    // The group block's own column layout, which the queue's block also takes.
    <div className="ms-decisions-queue">
      <div className="ms-decisions-head">
        <label className="ms-decisions-check ms-decisions-all">
          <input
            type="checkbox"
            checked={all}
            onChange={() => {
              onSelect(all ? [] : needs.map((need) => need.id));
            }}
          />
          Select all shown
        </label>
        {stopped.length > 0 && (
          <Button
            disabled={acts.proposals.running}
            onClick={() => {
              acts.proposals.onContinue(stopped);
            }}
          >
            {CONTINUE}
          </Button>
        )}
      </div>
      <ul className="ms-decisions-rows">
        {needs.map((need) => (
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
      {ticked.length > 0 && (
        <SelectionBar
          ticked={ticked}
          signedIn={acts.signedIn}
          running={acts.proposals.running}
          onApply={acts.proposals.onBulkApply}
          onDismiss={acts.proposals.onBulkDismiss}
          onClear={() => {
            onSelect([]);
          }}
        />
      )}
    </div>
  );
}

/** Every row's line as the page holds it, which is what carries the marks. */
function lineFor(needs: readonly Need[], acts: Acts): Line[] {
  const lines: Line[] = [];
  for (const need of needs) {
    const line = acts.proposals.lineOf(need);
    if (line !== null) {
      lines.push(line);
    }
  }
  return lines;
}

/**
 * The bar at the group's foot: how many are ticked, and the two acts over them.
 *
 * Apply opens a sheet and Dismiss does not, and the difference is the whole of
 * what a sheet is for here. Applying publishes an act per line, so what each one
 * would carry has to be read first — and a collapsed row shows the verb and the
 * subject, not the arguments (Astra S60-02). Dismissing publishes nothing at all.
 */
function SelectionBar({
  ticked,
  signedIn,
  running,
  onApply,
  onDismiss,
  onClear,
}: {
  ticked: readonly Need[];
  signedIn: boolean;
  running: boolean;
  onApply: (needs: readonly Need[]) => void;
  onDismiss: (needs: readonly Need[]) => void;
  onClear: () => void;
}) {
  return (
    <div className="ms-decisions-bar" role="group" aria-label="What you selected">
      <span className="ms-decisions-bar-count">{ticked.length} selected</span>
      <Help id="apply-proposed" />
      <Button
        primary
        disabled={!signedIn || running}
        onClick={() => {
          onApply(ticked);
        }}
      >
        {APPLY}
      </Button>
      <Button
        disabled={!signedIn || running}
        onClick={() => {
          onDismiss(ticked);
        }}
      >
        {DISMISS}
      </Button>
      <Button onClick={onClear}>Clear</Button>
    </div>
  );
}

/**
 * The sheet the bar's Apply opens: every ticked line whole, and one press.
 *
 * It exists because of what a collapsed row does not show. A row says the act and
 * the subject; what the act would CARRY — the reason, the whole intent an
 * approval authorises, the budget it would be admitted with — is in the open row,
 * and a bulk Apply over collapsed rows would publish arguments nobody had read
 * (Astra S60-02). So the confirmation is the list, whole, in the same words the
 * card and the open row use.
 *
 * The budgets are from ONE read, made when this sheet opened, kept in this
 * sheet's own state and sent as displayed: a tuple computed again at the press
 * could differ from the tuple a human confirmed (Astra S58-08). A line whose
 * budget could not be prefilled at all is listed, named and left unsent, exactly
 * as the queue's own bulk sheet lists one — and so is a line somebody has
 * already answered, because trying one again is a press of its own row.
 */
export function ProposalSheet({
  needs,
  backlog,
  holding,
  onClose,
  onApply,
}: {
  needs: readonly Need[];
  /** The backlog as it was read when this sheet opened. */
  backlog: Backlog;
  /** What the page holds about these lines, so the sheet reads them as it does. */
  holding: Applying;
  onClose: () => void;
  /** Apply these lines, with the tuples this sheet displayed. */
  onApply: (lines: readonly Line[]) => void;
}) {
  // Once, when the sheet opened, and kept: what is read is what is sent.
  const [budgets] = useState(() => budgetsFrom(holding.linesOf(needs), backlog));
  const held = holding.linesOf(needs);
  const lines = useMemo(
    () => held.map((line) => ({ ...line, displayed: budgets[line.id] ?? null })),
    [held, budgets],
  );
  const sending = lines.filter((line) => excludedInBulk(line) === "");
  const blocked = sending.length === 0 ? NOTHING_SENDABLE : "";

  return (
    <Panel
      eyebrow="Proposed by the Partner → applied"
      title={`Apply ${actionsWord(sending.length)}`}
      sheetName="Apply proposed"
      unproven=""
      refusal=""
      note={blocked === "" ? noteFor(sending.length) : blocked}
      form
      onClose={onClose}
      act={
        <Button
          primary
          disabled={blocked !== ""}
          onClick={() => {
            onApply(sending);
          }}
        >
          {APPLY}
        </Button>
      }
    >
      <ul className="ms-decisions-plan">
        {lines.map((line) => {
          const excluded = excludedInBulk(line);
          return (
            <li
              key={line.id}
              className={
                excluded === "" ? "ms-decisions-planned" : "ms-decisions-planned ms-decisions-planned--out"
              }
            >
              <ProposalSubstance line={line} />
              {excluded !== "" && <span className="ms-decisions-planned-out">{excluded}</span>}
            </li>
          );
        })}
      </ul>
    </Panel>
  );
}

/** "1 action", "4 actions": what the head counts is what would be sent. */
function actionsWord(sending: number): string {
  return `${String(sending)} ${sending === 1 ? "action" : "actions"}`;
}

function noteFor(sending: number): string {
  return (
    `One act per line, in this order, over ${actionsWord(sending)}. A refusal is passed and the run goes on; ` +
    "an answer that does not say what happened stops it."
  );
}
