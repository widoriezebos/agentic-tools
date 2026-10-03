import { useState } from "react";
import { NavLink } from "react-router";

import { AnchorPress, FindingDecisions } from "./Answers";
import { remarksOn, sectionRemark, type Remark } from "./remarks";
import { RemarkDrafts, RemarkNote } from "./RemarkNotes";
import { findingOfEntry, type DeskItem } from "./room";
import { drawingsIn } from "../drawing/drawings";
import { useStickies } from "../stickies/store";
import { Help } from "../help/Help";
import { askedFromSitting, ASK_IT, BOARD, BOARD_EMPTY, entryPath, goalsIn, pilesOf, type Entry } from "../partner/sitting";
import { usePartner } from "../partner/store";
import { Sheet as WritingSheet } from "../project/Sheet";
import "../partner/sitting.css";

/**
 * The room's board: the piles of the record's kind (g1-s67 D5), the face of the
 * desk pane that is not the desk. It is the table the drawer's focused view
 * used to stand beside the conversation, moved into the room, which is now
 * where every sitting sits (g1-s53 D7, g1-s65 D8).
 *
 * It is not a second store — it is the four sections of the sitting's record,
 * read from the one reading the sitting holds, each entry with its words, its
 * anchor or its reason, who put it there and when. An empty table on a record
 * nobody has sat on yet is an empty table, and it says so rather than pretending.
 *
 * Every entry is a link into the record it came out of, at its own pile (D7).
 * The table is a reading and the record is the thing: an entry a human wants to
 * check, argue with or edit is checked, argued with and edited where it is
 * written, and the one press that gets there is the entry itself.
 */
export function Board({ changed = [] }: { changed?: readonly string[] }) {
  const { sitting, table, putOnDesk, setFace, startRemark } = usePartner();
  const { notepad } = useStickies();
  // The open question the human pressed Ask it on, while its sheet is open.
  const [asking, setAsking] = useState<Entry | null>(null);
  if (sitting === null) {
    return null;
  }
  return (
    <aside className="ms-table" aria-label={BOARD}>
      <p className="ms-table-head">
        <span>{BOARD}</span>
        <Help id="the-board" />
      </p>
      <p className="ms-table-subject" title={sitting.subject.id}>
        {sitting.subject.title === "" ? sitting.subject.id : sitting.subject.title}
      </p>
      {table.entries.length === 0 && <p className="ms-table-empty">{BOARD_EMPTY}</p>}
      <RemarkDrafts />
      {pilesOf(sitting.purpose).map((section) => {
        const entries = table.entries.filter((entry) => entry.section === section);
        if (entries.length === 0) {
          return null;
        }
        return (
          <section key={section} className="ms-table-pile" aria-label={section}>
            <p className="ms-table-pile-head">
              {section}
              <span className="ms-table-pile-count">{entries.length}</span>
              {/* A remark from the board is about the pile, a section of the
                  record (g1-s71 D1). */}
              <button
                type="button"
                className="ms-table-remark"
                onClick={() => {
                  startRemark(sectionRemark(sitting.subject.id, section));
                }}
              >
                Remark
              </button>
            </p>
            <ul className="ms-table-entries">
              {entries.map((entry, at) => (
                <li key={`${section}-${String(at)}`} className="ms-table-entry">
                  {/* A review's findings carry their answers here as on their
                      cards (g1-s65 D8), and an anchor puts what it is about
                      back on the desk. */}
                  {section === "Findings" ? (
                    <div className="ms-table-finding">
                      <p className="ms-table-entry-text">{entry.text}</p>
                      {entry.clause !== "" && <AnchorPress anchor={entry.clause} />}
                      <p className="ms-table-entry-who">
                        {entry.who === "" ? entry.when : `${entry.who} · ${entry.when}`}
                      </p>
                      <FindingDecisions finding={findingOfEntry(entry)} />
                    </div>
                  ) : (
                  <>
                  <NavLink
                    className="ms-table-entry-link"
                    to={entryPath(sitting.subject.id, section)}
                    title={`Open ${sitting.subject.id} at ${section}`}
                  >
                    <p className="ms-table-entry-text">{entry.text}</p>
                    {entry.clause !== "" && section !== "Facts" && (
                      <p className="ms-table-entry-clause">{entry.clause}</p>
                    )}
                    <p className="ms-table-entry-who">
                      {entry.who === "" ? entry.when : `${entry.who} · ${entry.when}`}
                    </p>
                  </NavLink>
                  {/* A fact's anchor puts what it is about on the desk, as a
                      finding's does (g1-s67 D5): beside the link, because a
                      press inside a link is a press that navigates. */}
                  {section === "Facts" && entry.clause !== "" && <AnchorPress anchor={entry.clause} />}
                  {/* An open question of this record goes to the register with
                      one press (g1-s55 D2). It is beside the entry and not
                      inside its link, because a press inside a link is a press
                      that navigates. */}
                  {section === "Open questions" && (
                    <button
                      type="button"
                      className="ms-table-ask"
                      onClick={() => {
                        setAsking(entry);
                      }}
                    >
                      {ASK_IT}
                    </button>
                  )}
                  </>
                  )}
                </li>
              ))}
            </ul>
          </section>
        );
      })}
      <BoardRemarks
        remarks={remarksOn(notepad, sitting.subject.id)}
        open={(remark) => {
          putOnDesk(remarkItem(remark));
          setFace("desk");
        }}
      />
      <BoardDrawings
        source={table.source}
        open={(item) => {
          putOnDesk(item);
          setFace("desk");
        }}
      />
      {/* The Project pane's own question sheet, over the work area, prefilled
          with this question as one question: its words, what follows from
          leaving it open, and the record it came out of. The scope is the
          record's own goals, because the route scopes by ledger goal id and
          refuses a record path (Astra's F1). */}
      {asking !== null && (
        <WritingSheet
          request={{
            mode: "question",
            scope: null,
            question: askedFromSitting(asking, sitting.subject.id),
            goals: [...goalsIn(table.source)],
          }}
          onClose={() => {
            setAsking(null);
          }}
          onDone={() => {
            setAsking(null);
          }}
        />
      )}
    </aside>
  );
}

/**
 * The human's remarks on this sitting's record (g1-s71 D1), each where it was
 * made — its commit named once the desk reads another — and opening on the
 * desk as the desk reads now.
 */
export function BoardRemarks({ remarks, open }: { remarks: readonly Remark[]; open: (remark: Remark) => void }) {
  if (remarks.length === 0) {
    return null;
  }
  return (
    <section className="ms-table-pile" aria-label="Remarks">
      <p className="ms-table-pile-head">
        Remarks
        <span className="ms-table-pile-count">{remarks.length}</span>
      </p>
      <ul className="ms-remarks">
        {remarks.map((remark) => (
          <RemarkNote
            key={`${remark.sticky.id}-${remark.about.kind}-${String(remark.about.from ?? remark.about.section)}`}
            remark={remark}
            onOpen={() => {
              open(remark);
            }}
          />
        ))}
      </ul>
    </section>
  );
}

/** The drawings the record keeps (g1-s71 D3), each with its question, opening on the desk. */
export function BoardDrawings({
  source,
  open,
}: {
  source: string;
  open: (item: { kind: "drawing"; id: string; source: string; caption: string }) => void;
}) {
  const kept = drawingsIn(source);
  if (kept.length === 0) {
    return null;
  }
  return (
    <section className="ms-table-pile" aria-label="Drawings">
      <p className="ms-table-pile-head">
        Drawings
        <span className="ms-table-pile-count">{kept.length}</span>
      </p>
      <ul className="ms-table-entries">
        {kept.map((one) => (
          <li key={one.id} className="ms-table-entry">
            <button
              type="button"
              className="ms-table-entry-link ms-table-drawing"
              onClick={() => {
                open({ kind: "drawing", id: one.id, source: one.source, caption: one.caption });
              }}
            >
              <p className="ms-table-entry-text">{one.caption}</p>
              <p className="ms-table-entry-who">kept {one.date}</p>
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}

/** What opening a remark puts on the desk: its lines as the desk reads now, or its section. */
export function remarkItem(remark: Remark): DeskItem {
  const about = remark.about;
  if (about.kind === "section") {
    return { kind: "section", record: about.record ?? "", section: about.section ?? "" };
  }
  return { kind: "source", path: about.path ?? "", from: about.from ?? 0, to: about.to ?? about.from ?? 0 };
}
