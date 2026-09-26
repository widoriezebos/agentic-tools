import { NavLink } from "react-router";

import { usePartner } from "./store";
import { entryPath, SECTIONS, TABLE, TABLE_EMPTY } from "./sitting";
import { Help } from "../help/Help";
import "./sitting.css";

/**
 * The room's table: the four piles, beside the conversation.
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
export function SittingTable() {
  const { sitting, table } = usePartner();
  if (sitting === null) {
    return null;
  }
  return (
    <aside className="ms-table" aria-label={TABLE}>
      <p className="ms-table-head">
        <span>{TABLE}</span>
        <Help id="the-table" />
      </p>
      <p className="ms-table-subject" title={sitting.subject.id}>
        {sitting.subject.title === "" ? sitting.subject.id : sitting.subject.title}
      </p>
      {table.entries.length === 0 && <p className="ms-table-empty">{TABLE_EMPTY}</p>}
      {SECTIONS.map((section) => {
        const entries = table.entries.filter((entry) => entry.section === section);
        if (entries.length === 0) {
          return null;
        }
        return (
          <section key={section} className="ms-table-pile" aria-label={section}>
            <p className="ms-table-pile-head">
              {section}
              <span className="ms-table-pile-count">{entries.length}</span>
            </p>
            <ul className="ms-table-entries">
              {entries.map((entry, at) => (
                <li key={`${section}-${String(at)}`} className="ms-table-entry">
                  <NavLink
                    className="ms-table-entry-link"
                    to={entryPath(sitting.subject.id, section)}
                    title={`Open ${sitting.subject.id} at ${section}`}
                  >
                    <p className="ms-table-entry-text">{entry.text}</p>
                    {entry.clause !== "" && <p className="ms-table-entry-clause">{entry.clause}</p>}
                    <p className="ms-table-entry-who">
                      {entry.who === "" ? entry.when : `${entry.who} · ${entry.when}`}
                    </p>
                  </NavLink>
                </li>
              ))}
            </ul>
          </section>
        );
      })}
    </aside>
  );
}
