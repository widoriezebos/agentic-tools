import type { About } from "./api";
import { useStickies } from "./store";
import { about as stickiesAbout, chipPath, chipWords } from "./stickies";
import { ageBetween } from "../backlog/format";
import { Help } from "../help/Help";
import { NavLink } from "react-router";

/**
 * The stickies about the thing on this page, under its header.
 *
 * A note comes back where it belongs, which is the whole of D4: a human who
 * wrote "check g1-s13 tomorrow" meets it on g1-s13's page rather than having
 * to remember to open the panel. Only the open ones are here — a page is not
 * the notepad, and what has been struck off belongs behind the panel's own
 * disclosure.
 *
 * "Add a sticky" opens the panel with this page's chip already on, so writing
 * one from here is the same two seconds it is from anywhere else.
 *
 * A page with no stickies about it shows the button and nothing else. A block
 * that said "no stickies" on every goal page would be a sentence nobody needs
 * on every page; a button is the only thing there is to do here.
 */
export function StickiesBlock({ named }: { named: About }) {
  const { notepad, openPanel } = useStickies();
  const shown = stickiesAbout(notepad, named);
  const now = new Date().toISOString();

  return (
    <section className="ms-sticky-block" aria-label="Stickies about this">
      <div className="ms-sticky-block-head">
        <h3 className="ms-sticky-block-title">Stickies</h3>
        <Help id="stickies" />
        <button
          type="button"
          className="ms-act-link"
          onClick={() => {
            openPanel(named);
          }}
        >
          Add a sticky
        </button>
      </div>
      {shown.length > 0 && (
        <ul className="ms-sticky-block-rows">
          {shown.map((sticky) => (
            <li className="ms-sticky ms-sticky--small" key={sticky.id}>
              <p className="ms-sticky-text">{sticky.text}</p>
              <div className="ms-sticky-line">
                {sticky.about
                  .filter((one) => one.kind !== named.kind || one.id !== named.id)
                  .map((one) => (
                    <NavLink className="ms-sticky-chip ms-mono" key={`${one.kind}:${one.id}`} to={chipPath(one)}>
                      {chipWords(one)}
                    </NavLink>
                  ))}
                <time className="ms-sticky-age" dateTime={sticky.createdAt}>
                  {ageBetween(sticky.createdAt, now)}
                </time>
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
