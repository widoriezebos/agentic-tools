import { useId, useState } from "react";
import { useNavigate } from "react-router";

import { draftOf } from "./api";
import { usePartner } from "./store";
import { draftMadeLine, PURPOSES, purposeLabel, START, type Purpose } from "./sitting";
import { Help } from "../help/Help";
import { roomPath } from "../routes";
import { Button } from "../shell/controls";
import { Sheet } from "../shell/Sheet";
import "./sitting.css";
import { Trouble } from "../shell/Trouble";

/**
 * Starting a sitting: the sheet, on the record's own page, which opens the
 * sitting's room (g1-s67 D1, D6). What a sitting looks like once it has started
 * is the room's, and the drawer shows none of it.
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
  const navigate = useNavigate();
  const [purpose, setPurpose] = useState<Purpose>("shape a design");
  const [title, setTitle] = useState("");
  // The draft a refused press created before it was refused, or "". It is the
  // page's half of Sol's third finding: the record exists now, so pressing Start
  // again opens the sitting on it rather than writing a second record for the
  // same wish.
  const [made, setMade] = useState("");
  const purposeField = useId();
  const titleField = useId();
  const onThisRecord = subject !== null;

  const asked = () => {
    if (onThisRecord) {
      return { purpose, subject };
    }
    if (made !== "") {
      return { purpose, subject: { kind: "record", id: made, title: title.trim() } };
    }
    return { purpose, title };
  };

  const start = () => {
    void startSitting(asked()).then(
      (opened) => {
        onOpenChange(false);
        // Every sitting opens in its room (g1-s67 D1).
        if (opened !== "") {
          void navigate(roomPath(purpose, opened));
        }
      },
      (error: unknown) => {
        // The refusal is on the sheet, in the server's own words, and the sheet
        // stays open so the human can read it beside what they asked for. Where
        // it left a draft behind, that draft is what the next press is about.
        const created = draftOf(error);
        if (created !== "") {
          setMade(created);
        }
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
        <Trouble text={sittingRefusal} role="status" />
      )}
      {made !== "" && (
        <p className="ms-sitting-said" role="status">
          {draftMadeLine(made)}
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
