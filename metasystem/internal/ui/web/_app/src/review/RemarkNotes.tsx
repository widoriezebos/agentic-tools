import { useState, type KeyboardEvent } from "react";

import { cardOf, remarkPresses, remarkWhere, type Reading, type Remark } from "./remarks";
import { reviewedOf } from "./room";
import { usePartner } from "../partner/store";
import type { About } from "../stickies/api";
import { savesOn } from "../stickies/stickies";
import { useStickies } from "../stickies/store";
import { Trouble } from "../shell/Trouble";
import "../stickies/stickies.css";

/**
 * A remark on screen (g1-s71 D1): a small sticky, the human's own words, beside
 * the lines or the section it is about, on the desk and on the board. Its two
 * presses open the matching card with its words and its anchor filled — the
 * anchor carrying the remark's commit — and a finding only in a review.
 */

/** What the room reads a remark against, as it stands. */
export function useReading(): Reading {
  const { sitting, table, deskRead } = usePartner();
  return { purpose: sitting?.purpose ?? "review", tip: reviewedOf(table.source).tip, read: deskRead };
}

/** One remark, with where it is, what it says, and the cards it is made into. */
export function RemarkNote({ remark, onOpen }: { remark: Remark; onOpen?: () => void }) {
  const { startCard } = usePartner();
  const reading = useReading();
  return (
    <li className="ms-sticky ms-sticky--small ms-remark">
      <p className="ms-sticky-text">{remark.sticky.text}</p>
      <p className="ms-remark-where">{remarkWhere(remark.about, reading)}</p>
      <div className="ms-sticky-acts">
        {onOpen !== undefined && (
          <button type="button" className="ms-act-link" onClick={onOpen}>
            Open
          </button>
        )}
        {remarkPresses(reading.purpose).map((press) => (
          <button
            key={press.kind}
            type="button"
            className="ms-act-link"
            onClick={() => {
              const card = cardOf(remark, press.kind);
              startCard(card.anchor, card.kind, card.text);
            }}
          >
            {press.label}
          </button>
        ))}
      </div>
    </li>
  );
}

/** What a remark being written is about, in a few words. */
export function aboutWords(about: About): string {
  if (about.kind === "section") {
    return `§ ${about.section ?? ""}`;
  }
  const from = about.from ?? 0;
  const to = about.to ?? from;
  const file = (about.path ?? "").split("/").at(-1) ?? "";
  return to > from ? `${file}:${String(from)}-${String(to)}` : `${file}:${String(from)}`;
}

/**
 * The remarks being written: each a small sticky with its field, Save and
 * Cancel. Its words ride the room's drafts, so Step out and a reload keep them
 * until they are saved.
 */
export function RemarkDrafts() {
  const { remarking } = usePartner();
  const open = Object.entries(remarking);
  if (open.length === 0) {
    return null;
  }
  return (
    <ul className="ms-remark-drafts" aria-label="Remarks you are writing">
      {open.map(([id, draft]) => (
        <RemarkDraft key={id} id={id} text={draft.text} about={draft.about} />
      ))}
    </ul>
  );
}

function RemarkDraft({ id, text, about }: { id: string; text: string; about: About | undefined }) {
  const { noteRemark, dropRemark } = usePartner();
  const { jot } = useStickies();
  const [busy, setBusy] = useState(false);
  const [refusal, setRefusal] = useState("");
  if (about === undefined) {
    return null;
  }
  const save = () => {
    if (busy || text.trim() === "") {
      return;
    }
    setBusy(true);
    void jot(text, [about]).then((refused) => {
      setBusy(false);
      setRefusal(refused);
      if (refused === "") {
        dropRemark(id);
      }
    });
  };
  const keys = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (savesOn(event.key, event.shiftKey)) {
      event.preventDefault();
      save();
    }
  };
  return (
    <li className="ms-sticky ms-remark ms-remark--writing">
      <label className="ms-remark-where" htmlFor={id}>
        Remark on {aboutWords(about)}
      </label>
      <textarea
        id={id}
        className="ms-stickies-field"
        rows={2}
        value={text}
        placeholder="this lock is taken twice"
        onChange={(event) => {
          noteRemark(id, event.target.value);
        }}
        onKeyDown={keys}
      />
      <div className="ms-sticky-acts">
        <button type="button" className="ms-act-link" disabled={busy || text.trim() === ""} onClick={save}>
          Save
        </button>
        <button
          type="button"
          className="ms-act-link"
          disabled={busy}
          onClick={() => {
            dropRemark(id);
          }}
        >
          Cancel
        </button>
      </div>
      {refusal !== "" && <Trouble text={refusal} role="alert" variant="small" />}
    </li>
  );
}
