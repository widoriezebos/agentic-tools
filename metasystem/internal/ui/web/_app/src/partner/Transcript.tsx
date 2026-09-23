import { AlertTriangle, Square } from "lucide-react";
import { useEffect, useRef, useState } from "react";

import type { Message } from "./api";
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
 * What the Partner did on the way is a muted line, including every refusal, so
 * the moment it tried to leave its fence is visible rather than silent.
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
  // The newest words are the ones a human is reading, so the conversation
  // follows them down as they arrive. It is the scroller's own scrollTop and
  // not scrollIntoView, which would move every scrollable ancestor and carry
  // the work area off the screen with it.
  useEffect(() => {
    const scroller = scrollerOf(foot.current);
    if (scroller !== null) {
      scroller.scrollTop = scroller.scrollHeight;
    }
  }, [messages.length, store.live.text, store.live.activity.length, store.refusal]);
  return (
    <div className="ms-partner-transcript">
      {messages.map((message, index) => (
        <Said key={message.id} message={message} rendered={index >= firstRendered} />
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

function Said({ message, rendered }: { message: Message; rendered: boolean }) {
  if (message.role === "human") {
    return (
      <div className="ms-partner-said ms-partner-said--human">
        <p className="ms-partner-who">You</p>
        <Paragraphs text={message.text} />
      </div>
    );
  }
  const failed = message.outcome === "failed" || message.outcome === "refused";
  return (
    <div className="ms-partner-said">
      <p className="ms-partner-who">Project Partner</p>
      {(message.activity ?? []).map((line, index) => (
        <Activity key={index} line={line} />
      ))}
      {message.text !== "" &&
        (rendered ? <Answer text={message.text} /> : <Paragraphs text={message.text} />)}
      {message.outcome === "stopped" && <p className="ms-partner-note">Stopped.</p>}
      {failed && <p className="ms-partner-failed">{message.detail ?? "The turn did not finish."}</p>}
    </div>
  );
}

/** The turn that is running: what it has said, and what it is doing. */
function Running() {
  const { store, stop } = usePartner();
  const live = store.live;
  return (
    <div className="ms-partner-said">
      <p className="ms-partner-who">Project Partner</p>
      {live.activity.map((line, index) => (
        <Activity key={index} line={line} />
      ))}
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

function Answer({ text }: { text: string }) {
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
      <Markdown blocks={blocks} from="" />
    </div>
  );
}
