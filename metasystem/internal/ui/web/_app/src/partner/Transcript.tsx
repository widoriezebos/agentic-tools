import { AlertTriangle } from "lucide-react";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { NavLink, useNavigate } from "react-router";

import type { Message, Page } from "./api";
import { chipOf } from "./capture";
import { Looked } from "./Looked";
import { namesIn, runsIn, type Names } from "./references";
import { ring } from "./ringing";
import { usePartner } from "./store";
import "./partner.css";
import { previewDocument, type Block } from "../project/api";
import { Markdown } from "../project/Markdown";

/**
 * The conversation, as a human reads it.
 *
 * One column, one measure, two voices. A question is a block in the surface
 * colour against the right edge, at most two thirds of the measure, with the
 * page it was asked from as a tiny label under it; an answer is plain prose in
 * the reading face, at the size the document pages are read at, with no box
 * around it — an answer is read, not received. Under each answer stands one
 * muted line saying what it was given and what it looked at, and nothing else
 * stands between two turns.
 *
 * An answer arrives in pieces, so while it is arriving it is shown as plain
 * paragraphs: there is no half-parsed Markdown that is worth showing, and a
 * renderer run on every chunk would reflow the page under the human's eyes.
 * When the turn ends the same text goes through the reader's own route — the
 * engine parses it into a typed tree and the reader builds elements from it —
 * so a Partner that writes a list, a table or a code block is read as one, and
 * nothing it writes can become markup: there is no HTML string anywhere on
 * this path.
 *
 * A human's message wears the capture it was asked from, so "these" still names
 * something weeks later and clicking it goes back to the board that made it
 * mean that. And the names in an answer become links, where and only where they
 * point at one thing.
 */

/** How far back the transcript is rendered through the reader's route.
 *
 * Every rendering is one request, and a conversation of a hundred messages
 * would be fifty of them on a page load for text nobody has scrolled back to.
 * The recent exchange is what a human is reading; older answers keep their
 * paragraphs, which is what they looked like while they were arriving.
 */
const RENDERED = 25;

/**
 * How near the end counts as being at it. A line of prose is under this, so a
 * human who has read to the bottom is followed down rather than offered a pill
 * for the pixel they are short of.
 */
const AT_END = 24;

export function Transcript() {
  const { store } = usePartner();
  const messages = store.messages;
  const firstRendered = Math.max(0, messages.length - RENDERED);
  const column = useRef<HTMLDivElement | null>(null);
  // Whether the human is reading the end of the conversation. It is the
  // scroller's own answer, kept in a ref because what matters is where they
  // were before the new words arrived, not where the new words have left them.
  const following = useRef(true);
  const [behind, setBehind] = useState(false);
  // Every name this workspace answers to, built once per index rather than
  // once per answer: the conversation re-renders on every beat.
  const names = useMemo(() => namesIn(store.index), [store.index]);

  // Where the human is in the conversation, from the scroller itself. It is
  // the only thing that clears the pill: no timer takes it away.
  useEffect(() => {
    const scroller = scrollerOf(column.current);
    if (scroller === null) {
      return;
    }
    const scrolled = () => {
      const end = atEnd(scroller);
      following.current = end;
      if (end) {
        setBehind(false);
      }
    };
    scroller.addEventListener("scroll", scrolled, { passive: true });
    return () => {
      scroller.removeEventListener("scroll", scrolled);
    };
  }, []);

  // The newest words are the ones a human at the end is reading, so the
  // conversation follows them down as they arrive. It is the scroller's own
  // scrollTop and not scrollIntoView, which would move every scrollable
  // ancestor and carry the work area off the screen with it. A human who has
  // scrolled up is left where they are, and told there is more below.
  useEffect(() => {
    const scroller = scrollerOf(column.current);
    if (scroller === null) {
      return;
    }
    if (following.current) {
      scroller.scrollTop = scroller.scrollHeight;
      return;
    }
    setBehind(true);
  }, [messages.length, store.live.text, store.live.doing, store.live.looked.length, store.refusal]);

  const toEnd = () => {
    const scroller = scrollerOf(column.current);
    if (scroller !== null) {
      scroller.scrollTop = scroller.scrollHeight;
    }
    following.current = true;
    setBehind(false);
  };

  return (
    <div ref={column} className="ms-conversation ms-partner-transcript">
      {messages.map((message, index) => (
        <Said key={message.id} message={message} rendered={index >= firstRendered} names={names} />
      ))}
      {store.live.turn !== "" && <Running />}
      {store.refusal !== "" && <Refusal reason={store.refusal} install={store.install} />}
      {behind && (
        <button type="button" className="ms-partner-latest" onClick={toEnd}>
          Latest ↓
        </button>
      )}
    </div>
  );
}

/** The nearest ancestor that scrolls, or null where nothing does. */
function scrollerOf(from: Element | null): HTMLElement | null {
  let at = from?.parentElement ?? null;
  while (at !== null) {
    const overflow = globalThis.getComputedStyle(at).overflowY;
    if (overflow === "auto" || overflow === "scroll") {
      return at;
    }
    at = at.parentElement;
  }
  return null;
}

/** True while the scroller is showing the end of what is in it. */
function atEnd(scroller: HTMLElement): boolean {
  return scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight <= AT_END;
}

function Said({ message, rendered, names }: { message: Message; rendered: boolean; names: Names }) {
  if (message.role === "human") {
    return (
      <div className="ms-partner-said ms-partner-said--human">
        <p className="ms-visually-hidden">You</p>
        <div className="ms-partner-bubble">
          <Paragraphs text={message.text} />
        </div>
        {message.page !== undefined && <AskedFrom capture={message.page} />}
      </div>
    );
  }
  const failed = message.outcome === "failed" || message.outcome === "refused";
  return (
    <div className="ms-partner-said ms-partner-said--partner">
      <p className="ms-visually-hidden">Project Partner</p>
      {message.text !== "" &&
        (rendered ? <Answer text={message.text} names={names} /> : <Paragraphs text={message.text} />)}
      {message.outcome === "stopped" && <p className="ms-partner-note">Stopped.</p>}
      {failed && <p className="ms-partner-failed">{message.detail ?? "The turn did not finish."}</p>}
      <Looked looked={message.looked ?? []} />
    </div>
  );
}

/**
 * The capture a question was asked from, as a label that goes back to it.
 *
 * Clicking it opens that page with the view, filters, window and tab the
 * capture carries, through the board's own landing; the page then says what it
 * applied, and names anything it could not.
 */
function AskedFrom({ capture }: { capture: Page }) {
  const navigate = useNavigate();
  const said = chipOf(capture);
  const to = capture.return ?? capture.path;
  if (said === "") {
    return null;
  }
  return (
    <button
      type="button"
      className="ms-partner-asked-from"
      title={`Go back to ${said}`}
      onClick={() => {
        void navigate(to);
      }}
    >
      {said}
    </button>
  );
}

/**
 * The turn that is running.
 *
 * Until the first words arrive, the answer's place holds one italic line saying
 * what the Partner is doing now, under a pulse a reduced-motion preference
 * turns off. What it looked at is an account of a turn that is over, so the
 * list waits for the end of it; Stop is in the composer, where every other
 * thing a human presses is.
 */
function Running() {
  const { store } = usePartner();
  const live = store.live;
  if (live.text !== "") {
    return (
      <div className="ms-partner-said ms-partner-said--partner">
        <p className="ms-visually-hidden">Project Partner</p>
        <Paragraphs text={live.text} />
      </div>
    );
  }
  return (
    <div className="ms-partner-said ms-partner-said--partner">
      <p className="ms-visually-hidden">Project Partner</p>
      <p className="ms-partner-working">{live.doing === "" ? "Thinking…" : live.doing}</p>
    </div>
  );
}

/** A refusal, in the server's own words, with the line that installs it. */
function Refusal({ reason, install }: { reason: string; install: string }) {
  return (
    <div className="ms-partner-said ms-partner-said--partner">
      <p className="ms-visually-hidden">Project Partner</p>
      <p className="ms-partner-failed">
        <AlertTriangle size={14} strokeWidth={1.75} aria-hidden="true" />
        {reason}
      </p>
      {install !== "" && (
        <pre className="ms-partner-install">
          <code>{install}</code>
        </pre>
      )}
    </div>
  );
}

/**
 * Text as paragraphs, which is what an answer looks like while it is
 * arriving. A blank line is a paragraph break and nothing else is read.
 */
export function Paragraphs({ text }: { text: string }) {
  const paragraphs = text.split(/\n{2,}/).filter((part) => part.trim() !== "");
  return (
    <>
      {paragraphs.map((paragraph, index) => (
        <p key={index} className="ms-partner-text">
          {paragraph}
        </p>
      ))}
    </>
  );
}

/**
 * One complete answer, through the engine's own parser and the reader's own
 * elements. It is rendered once and remembered by its text, so a re-render of
 * the transcript does not ask again.
 */
const rendered = new Map<string, Block[]>();

function Answer({ text, names }: { text: string; names: Names }) {
  const [blocks, setBlocks] = useState<Block[] | null>(() => rendered.get(text) ?? null);
  useEffect(() => {
    const held = rendered.get(text);
    if (held !== undefined) {
      setBlocks(held);
      return;
    }
    const aborter = new AbortController();
    previewDocument(text)
      .then((preview) => {
        if (aborter.signal.aborted) {
          return;
        }
        rendered.set(text, preview.blocks);
        setBlocks(preview.blocks);
      })
      .catch(() => {
        // A render that did not come back leaves the paragraphs on screen,
        // which is what the answer looked like while it was arriving.
      });
    return () => {
      aborter.abort();
    };
  }, [text]);
  if (blocks === null) {
    return <Paragraphs text={text} />;
  }
  return (
    <div className="ms-partner-answer">
      <Markdown blocks={blocks} from="" renderText={(words) => <References words={words} names={names} />} />
    </div>
  );
}

/**
 * The names in a run of an answer's text, as links where they point at one
 * thing and as words everywhere else.
 */
function References({ words, names }: { words: string; names: Names }): ReactNode {
  const runs = useMemo(() => runsIn(words, names), [words, names]);
  return (
    <>
      {runs.map((run, at) =>
        "text" in run ? (
          run.text
        ) : (
          <Mention key={at} to={run.reference.to} goal={run.reference.kind === "goal" ? run.reference.id : ""}>
            {run.reference.text}
          </Mention>
        ),
      )}
    </>
  );
}

/**
 * One reference in an answer: a link that navigates when it is pressed and
 * rings the card it names while it is hovered or focused.
 *
 * Hovering never navigates and never clears a filter. Clicking the reference
 * navigates — the reference, not the words around it — and a goal that is
 * hidden by a filter or on another page arrives through the board's own
 * landing, which reveals it and says what it changed.
 */
function Mention({ to, goal, children }: { to: string; goal: string; children: ReactNode }) {
  const marked = useRef<(() => void) | null>(null);
  const mark = () => {
    if (goal === "" || marked.current !== null) {
      return;
    }
    marked.current = ring(goal);
  };
  const unmark = () => {
    marked.current?.();
    marked.current = null;
  };
  useEffect(() => unmark, []);
  return (
    <NavLink
      className="ms-partner-mention"
      to={to}
      onMouseEnter={mark}
      onMouseLeave={unmark}
      onFocus={mark}
      onBlur={unmark}
    >
      {children}
    </NavLink>
  );
}
