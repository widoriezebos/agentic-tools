import { AlertTriangle, Square } from "lucide-react";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { NavLink, useNavigate } from "react-router";

import type { Look, Message, Page } from "./api";
import { chipOf } from "./capture";
import { Looked } from "./Looked";
import { namesIn, runsIn, type Names } from "./references";
import { ring } from "./ringing";
import { usePartner } from "./store";
import "./partner.css";
import { previewDocument, type Block } from "../project/api";
import { Markdown } from "../project/Markdown";
import { Button } from "../shell/controls";

/**
 * The conversation, as a human reads it.
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
 * Three things stand around each exchange. A human's message wears the capture
 * it was asked from, so "these" still names something weeks later and clicking
 * it goes back to the board that made it mean that. An answer says what it was
 * read from — the page first, then every tool call, with its outcome — and
 * what it was given, in a stamp that speaks of the past. And the names in an
 * answer become links, where and only where they point at one thing.
 */

/** How far back the transcript is rendered through the reader's route.
 *
 * Every rendering is one request, and a conversation of a hundred messages
 * would be fifty of them on a page load for text nobody has scrolled back to.
 * The recent exchange is what a human is reading; older answers keep their
 * paragraphs, which is what they looked like while they were arriving.
 */
const RENDERED = 25;

export function Transcript() {
  const { store } = usePartner();
  const messages = store.messages;
  const firstRendered = Math.max(0, messages.length - RENDERED);
  const foot = useRef<HTMLDivElement | null>(null);
  // Every name this workspace answers to, built once per index rather than
  // once per answer: the conversation re-renders on every beat.
  const names = useMemo(() => namesIn(store.index), [store.index]);
  // The newest words are the ones a human is reading, so the conversation
  // follows them down as they arrive. It is the scroller's own scrollTop and
  // not scrollIntoView, which would move every scrollable ancestor and carry
  // the work area off the screen with it.
  useEffect(() => {
    const scroller = scrollerOf(foot.current);
    if (scroller !== null) {
      scroller.scrollTop = scroller.scrollHeight;
    }
  }, [messages.length, store.live.text, store.live.doing, store.live.looked.length, store.refusal]);
  return (
    <div className="ms-partner-transcript">
      {messages.map((message, index) => (
        <Said key={message.id} message={message} rendered={index >= firstRendered} names={names} />
      ))}
      {store.live.turn !== "" && <Running />}
      {store.refusal !== "" && <Refusal reason={store.refusal} install={store.install} />}
      <div ref={foot} className="ms-partner-foot" />
    </div>
  );
}

/** The nearest ancestor that scrolls, or null where nothing does. */
function scrollerOf(from: Element | null): HTMLElement | null {
  let at = from?.parentElement ?? null;
  while (at !== null) {
    const overflow = globalThis.getComputedStyle(at).overflowY;
    if ((overflow === "auto" || overflow === "scroll") && at.scrollHeight > at.clientHeight) {
      return at;
    }
    at = at.parentElement;
  }
  return null;
}

function Said({ message, rendered, names }: { message: Message; rendered: boolean; names: Names }) {
  if (message.role === "human") {
    return (
      <div className="ms-partner-said ms-partner-said--human">
        <p className="ms-partner-who">You</p>
        <Paragraphs text={message.text} />
        {message.page !== undefined && <AskedFrom capture={message.page} />}
      </div>
    );
  }
  const failed = message.outcome === "failed" || message.outcome === "refused";
  const looked = message.looked ?? [];
  return (
    <div className="ms-partner-said">
      <p className="ms-partner-who">Project Partner</p>
      <Looked looked={looked} />
      {(message.activity ?? []).map((line, index) => (
        <Activity key={index} line={line} />
      ))}
      {message.text !== "" &&
        (rendered ? <Answer text={message.text} names={names} /> : <Paragraphs text={message.text} />)}
      <Stamp looked={looked} />
      {message.outcome === "stopped" && <p className="ms-partner-note">Stopped.</p>}
      {failed && <p className="ms-partner-failed">{message.detail ?? "The turn did not finish."}</p>}
    </div>
  );
}

/**
 * What the answer was given, in the past tense.
 *
 * "Seeing:" on the chip says what the NEXT question will carry; this says what
 * this one did. They are different words on purpose: an answer already given
 * must not appear to change because the board moved afterwards, and pressing
 * Refresh must not read as if the Partner had been told something.
 */
function Stamp({ looked }: { looked: readonly Look[] }) {
  const page = looked[0];
  if (page === undefined || page.source === undefined || page.source === "") {
    return null;
  }
  return <p className="ms-partner-stamp">Saw: {page.source}</p>;
}

/**
 * The capture a question was asked from, as a chip that goes back to it.
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

/** The turn that is running: what it has read, what it has said, and Stop. */
function Running() {
  const { store, stop } = usePartner();
  const live = store.live;
  const doing = live.doing;
  return (
    <div className="ms-partner-said">
      <p className="ms-partner-who">Project Partner</p>
      {/* While it works, one line says what it is doing now; the list of what
          it read is what stands here when the turn is over. */}
      {doing !== "" && <Activity line={doing} />}
      {live.text === "" ? (
        <p className="ms-partner-note">Thinking…</p>
      ) : (
        <Paragraphs text={live.text} />
      )}
      <div className="ms-partner-stop">
        <Button
          onClick={() => {
            stop();
          }}
        >
          <Square size={14} strokeWidth={1.75} aria-hidden="true" />
          Stop
        </Button>
      </div>
    </div>
  );
}

/** A refusal, in the server's own words, with the line that installs it. */
function Refusal({ reason, install }: { reason: string; install: string }) {
  return (
    <div className="ms-partner-said">
      <p className="ms-partner-who">Project Partner</p>
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

export function Activity({ line }: { line: string }) {
  return <p className="ms-partner-activity">{line}</p>;
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
