import { createContext, useContext, useEffect, useRef, useState } from "react";

import { drawingId, drawWith, type Drawn, type Kept } from "./drawings";
import { Trouble } from "../shell/Trouble";
import "./drawing.css";

/**
 * A drawing (g1-s71 D2, D3): a mermaid fence as a picture, wherever rendered
 * Markdown carries one — the conversation, a record's section on the desk, the
 * board — through one chunk loaded the first time a drawing is on screen. The
 * source is a press away, and is shown instead, with the reason in words, when
 * the chunk does not load or the parse fails.
 *
 * In a room's conversation a drawing offers Put on the desk and Keep it; on the
 * desk it offers Keep it until the record keeps it. What it was drawn for is
 * the answer's turn and the question asked before it, which the transcript
 * says through DrawingOrigin; what the presses do is the room's, through
 * DrawingPresses. Anywhere else a drawing is only a picture.
 */

/** Where an answer's drawings were drawn: its turn, and the question before it. */
export const DrawingOrigin = createContext<{ turn: string; asked: string } | null>(null);

/** A drawing as the desk carries it. */
export type DrawingItem = { kind: "drawing"; id: string; source: string; caption: string };

/** What a room does with a drawing: put it on the desk, keep it, and whether the record keeps it. */
export type Presses = {
  put: (item: DrawingItem) => void;
  keep: (kept: Kept) => Promise<string>;
  kept: (id: string) => boolean;
};

export const DrawingPresses = createContext<Presses | null>(null);

/** Each render is named once on the page; the library draws under that name. */
let drawings = 0;

export function Drawing({ source, item }: { source: string; item?: DrawingItem }) {
  const origin = useContext(DrawingOrigin);
  const presses = useContext(DrawingPresses);
  const holder = useRef<HTMLDivElement | null>(null);
  const [drawn, setDrawn] = useState<Drawn | null>(null);
  const [showing, setShowing] = useState(false);
  const [keeping, setKeeping] = useState(false);
  const [refusal, setRefusal] = useState("");

  useEffect(() => {
    const into = holder.current;
    if (into === null) {
      return;
    }
    let live = true;
    setDrawn(null);
    drawings += 1;
    void drawWith(() => import("./render"), source, into, `ms-drawing-${String(drawings)}`).then((done) => {
      if (live) {
        setDrawn(done);
      }
    });
    return () => {
      live = false;
    };
  }, [source]);

  // What this drawing is, where the room can keep it: the desk's own item, or
  // one drawn in an answer, named by its turn and captioned by its question.
  const named: DrawingItem | null =
    item ?? (origin === null ? null : { kind: "drawing", id: drawingId(origin.turn, source), source, caption: origin.asked });
  const refused = drawn?.state === "refused" ? drawn.said : "";
  const kept = named !== null && presses?.kept(named.id) === true;

  const keep = () => {
    if (named === null || presses === null) {
      return;
    }
    setKeeping(true);
    setRefusal("");
    void presses
      .keep({ id: named.id, caption: named.caption, date: today(), source: named.source })
      .then((said) => {
        setKeeping(false);
        setRefusal(said);
      });
  };

  return (
    <figure className="ms-drawing">
      <div className="ms-drawing-picture" ref={holder} hidden={refused !== ""} />
      {drawn === null && <p className="ms-drawing-arriving">Drawing…</p>}
      {refused !== "" && <Trouble text={refused} role="status" variant="small" />}
      {(showing || refused !== "") && (
        <pre className="ms-md-pre ms-drawing-source">
          <code>{source}</code>
        </pre>
      )}
      <figcaption className="ms-drawing-presses">
        {refused === "" && (
          <button
            type="button"
            className="ms-act-link"
            onClick={() => {
              setShowing(!showing);
            }}
          >
            {showing ? "Hide the source" : "Show the source"}
          </button>
        )}
        {named !== null && presses !== null && item === undefined && (
          <button
            type="button"
            className="ms-act-link"
            onClick={() => {
              presses.put(named);
            }}
          >
            Put on the desk
          </button>
        )}
        {named !== null && presses !== null &&
          (kept ? (
            <span className="ms-drawing-kept">Kept in the record</span>
          ) : (
            <button type="button" className="ms-act-link" disabled={keeping} onClick={keep}>
              {keeping ? "Keeping…" : "Keep it"}
            </button>
          ))}
      </figcaption>
      {refusal !== "" && <Trouble text={refusal} role="status" variant="small" />}
    </figure>
  );
}

/** The day a drawing is kept, as a record writes a date: in this browser's own time zone. */
function today(): string {
  const now = new Date();
  const two = (value: number) => String(value).padStart(2, "0");
  return `${String(now.getFullYear())}-${two(now.getMonth() + 1)}-${two(now.getDate())}`;
}
