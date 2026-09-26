import { useId, useState } from "react";

import { usePartner } from "./store";
import {
  END,
  PURPOSES,
  purposeLabel,
  SECTIONS,
  sittingChip,
  START,
  TABLE,
  type Purpose,
} from "./sitting";
import { Help } from "../help/Help";
import { Button } from "../shell/controls";
import { Sheet } from "../shell/Sheet";
import "./sitting.css";

/**
 * Starting a sitting, and what a sitting looks like once it has started.
 *
 * Three things, in one file because they are one act seen from three places: the
 * sheet that opens one, the control that opens that sheet — on a record's page
 * and in the Partner's own header, which is where a human is when they decide to
 * sit down — and the four counts that say what the record now holds.
 *
 * What a human chooses is small on purpose: what the sitting is for, and which
 * record it is about. Nothing else is asked, because nothing else is needed: the
 * Partner's first turn is the interface's own and brings what the records hold,
 * so a human sits down with a wish rather than with a form.
 */

/**
 * The sheet: the purpose, and the subject.
 *
 * The subject is this record where the page has one, and a title otherwise —
 * which the server creates as a draft, in the home the purpose's own kind names,
 * before the sitting is opened at all. So "a new draft named now" is one field
 * and not a second act a human has to remember to do first.
 */
export function StartSittingSheet({
  open,
  onOpenChange,
  subject,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The record the page is about, or null where the page is about none. */
  subject: { kind: string; id: string; title: string } | null;
}) {
  const { startSitting, sittingBusy, sittingRefusal } = usePartner();
  const [purpose, setPurpose] = useState<Purpose>("shape a design");
  const [title, setTitle] = useState("");
  const purposeField = useId();
  const titleField = useId();
  const onThisRecord = subject !== null;

  const start = () => {
    void startSitting(
      onThisRecord ? { purpose, subject } : { purpose, title },
    ).then(
      () => {
        onOpenChange(false);
      },
      () => {
        // The refusal is on the sheet, in the server's own words, and the sheet
        // stays open so the human can read it beside what they asked for.
      },
    );
  };

  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      side="right"
      label={START}
      title={START}
      closeLabel="Close without starting a sitting"
      bodyClassName="ms-sitting-sheet"
      sheetName={START}
    >
      <p className="ms-sitting-said">
        A sitting is a working conversation on one record. What you decide in it goes into that
        record as you go, by your own press, and the Partner brings what the records already hold.
        It lays out options with their consequences and never says which to pick.
        <Help id="no-recommendation" />
      </p>
      <label className="ms-sitting-label" htmlFor={purposeField}>
        What is it for
        <Help id="sitting" />
      </label>
      <select
        id={purposeField}
        className="ms-sitting-select"
        value={purpose}
        onChange={(event) => {
          setPurpose(event.target.value as Purpose);
        }}
      >
        {PURPOSES.map((one) => (
          <option key={one} value={one}>
            {purposeLabel(one)}
          </option>
        ))}
      </select>
      {onThisRecord ? (
        <p className="ms-sitting-subject">
          <span className="ms-sitting-label">What it is about</span>
          {subject.title === "" ? subject.id : subject.title}
          <span className="ms-mono ms-sitting-path">{subject.id}</span>
        </p>
      ) : (
        <>
          <label className="ms-sitting-label" htmlFor={titleField}>
            What it is about
          </label>
          <input
            id={titleField}
            className="ms-sitting-field"
            type="text"
            value={title}
            placeholder="a title for a new draft"
            onChange={(event) => {
              setTitle(event.target.value);
            }}
          />
          <p className="ms-sitting-said">
            A record with this title is created as a draft, and the sitting is about it.
          </p>
        </>
      )}
      {sittingRefusal !== "" && (
        <p className="ms-sitting-refusal" role="status">
          {sittingRefusal}
        </p>
      )}
      <div className="ms-sitting-foot">
        <Button primary disabled={sittingBusy || (!onThisRecord && title.trim() === "")} onClick={start}>
          {sittingBusy ? "Starting…" : START}
        </Button>
      </div>
    </Sheet>
  );
}

/**
 * The control in the Partner's own header: Start a sitting, or, while one
 * stands, which record it is on with the four counts beside it.
 *
 * It is one component so the drawer places one element. The counts are the
 * record's own four sections as the sitting's reading holds them, and pressing
 * them opens the table — because the counts are the summary and the table is the
 * thing (D7).
 */
export function SittingControl({ onOpenTable }: { onOpenTable: () => void }) {
  const { sitting, endSitting, sittingBusy } = usePartner();
  const [starting, setStarting] = useState(false);

  if (sitting === null) {
    return (
      <>
        <button
          type="button"
          className="ms-sitting-start"
          onClick={() => {
            setStarting(true);
          }}
        >
          {START}
        </button>
        <StartSittingSheet open={starting} onOpenChange={setStarting} subject={null} />
      </>
    );
  }
  return (
    <span className="ms-sitting-standing">
      <span className="ms-sitting-chip" title={sitting.subject.id}>
        {sittingChip(sitting)}
      </span>
      <Help id="sitting" />
      <SittingCounts onOpen={onOpenTable} />
      <button type="button" className="ms-sitting-end" disabled={sittingBusy} onClick={endSitting}>
        {END}
      </button>
    </span>
  );
}

/**
 * The four counts: what the record's four sections hold, as a strip that opens
 * the table.
 *
 * They are counts of the RECORD and not of the conversation, which is the whole
 * of what makes them honest: a card the Partner offered and nobody recorded is
 * not counted here, because it is not in the record and a fresh worker reading
 * the record would not find it.
 */
export function SittingCounts({ onOpen }: { onOpen: () => void }) {
  const { table } = usePartner();
  return (
    <button type="button" className="ms-sitting-counts" onClick={onOpen} title={`Open ${TABLE}`}>
      {SECTIONS.map((section) => (
        <span key={section} className="ms-sitting-count">
          <span className="ms-sitting-count-name">{section}</span>
          <span className="ms-sitting-count-number">{table.counts[section]}</span>
        </span>
      ))}
    </button>
  );
}
