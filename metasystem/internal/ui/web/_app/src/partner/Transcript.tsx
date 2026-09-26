import { AlertTriangle, MessageSquare, User } from "lucide-react";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { NavLink, useNavigate } from "react-router";

import type { Message, Page } from "./api";
import { chipOf, sheetNote, whenOf } from "./capture";
import { nameOf } from "./conversation";
import { DepositCard } from "./Deposit";
import { useTypefaceOn } from "./FontControl";
import { Looked } from "./Looked";
import { namesIn, runsIn, type Names } from "./references";
import { ring } from "./ringing";
import { atEnd, scrollerOf } from "./scrolling";
import { usePartner } from "./store";
import { SuggestionCard } from "./Suggestion";
import { depositID, THE_INTERFACES } from "./sitting";
import { idOf } from "./suggesting";
import "./partner.css";
import { previewDocument, type Block } from "../project/api";
import { Markdown } from "../project/Markdown";

/**
 * The conversation, as a human reads it.
 *
 * One column, and every turn attributed the same way, so a reader never has to
 * infer who spoke. Above each turn stands one small row — a mark, the
 * speaker's name, and the time at the row's end — and under it the words: the
 * human's on a tinted block spanning the column, the Partner's as plain prose
 * with a hairline in the accent down its left, from the header to the meta
 * line that says what the answer was given and what it looked at.
 *
 * g1-s31 told the two voices apart by shape alone, a block against the right
 * edge and prose beside it, and once the conversation took the drawer's width
 * the block sat far from the prose it answered; a short question and a short
 * answer looked alike (Wido, 2026-09-24: "I also would like to see a bit more
 * clearly the difference between my message and the partner message"). So
 * attribution is written down rather than implied by position, and nothing
 * here is right-aligned: the eye reads down one edge.
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

/** What the Partner is called in the row above everything it says. */
const PARTNER = "Project Partner";

/** The mark a header row wears. Sixteen pixels beside a twelve pixel row. */
const MARK = 16;

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
  // What every question is signed with. It is the human the server says this
  // conversation is, which is the same answer the identity control reads, so
  // the row above a question and the chip in the header never disagree.
  const asker = nameOf(store.human);
  // The face and the size a human chose for this conversation, on the column
  // they chose it for and on nothing above it.
  useTypefaceOn(column);

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
  }, [
    messages.length,
    store.live.text,
    store.live.doing,
    store.live.looked.length,
    store.live.suggestions.length,
    store.live.deposits.length,
    store.refusal,
  ]);

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
        <Said
          key={message.id}
          message={message}
          rendered={index >= firstRendered}
          names={names}
          asker={asker}
        />
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

function Said({
  message,
  rendered,
  names,
  asker,
}: {
  message: Message;
  rendered: boolean;
  names: Names;
  asker: string;
}) {
  if (message.role === "human") {
    return (
      <div className="ms-turn ms-turn--human">
        {/* The one question nobody typed. A sitting's opening turn is submitted
            by this interface on the human's behalf, and it says so here — from
            the message itself, so a reload cannot turn it into their own words
            (g1-s53 D3). */}
        <TurnHead who="human" name={asker} at={message.at}>
          {message.interface === true && <span className="ms-turn-by">{THE_INTERFACES}</span>}
          {message.page !== undefined && <AskedFrom capture={message.page} />}
        </TurnHead>
        <div className="ms-turn-body">
          <Paragraphs text={message.text} />
        </div>
      </div>
    );
  }
  const failed = message.outcome === "failed" || message.outcome === "refused";
  return (
    <div className="ms-turn ms-turn--partner">
      <TurnHead who="partner" name={PARTNER} at={message.at} />
      <div className="ms-turn-body">
        {message.text !== "" &&
          (rendered ? <Answer text={message.text} names={names} /> : <Paragraphs text={message.text} />)}
        {message.outcome === "stopped" && <p className="ms-partner-note">Stopped.</p>}
        {failed && <p className="ms-partner-failed">{message.detail ?? "The turn did not finish."}</p>}
        {/* What this answer offered, under the words that offered it. The card
            is where the human decides; nothing has been written anywhere. */}
        {(message.suggestions ?? []).map((suggestion, at) => (
          <SuggestionCard key={idOf(message.turn, at)} id={idOf(message.turn, at)} />
        ))}
        {/* What this answer offered the sitting's record. Record it is the
            human's press, and nothing is in the record until they make it. */}
        {(message.deposits ?? []).map((deposit, at) => (
          <DepositCard key={depositID(message.turn, at)} id={depositID(message.turn, at)} />
        ))}
        <Looked looked={message.looked ?? []} />
      </div>
    </div>
  );
}

/**
 * Who spoke, and when: the row that stands above every turn.
 *
 * The mark is inline in the name's own line rather than a column of its own,
 * so that the mark, the name and the time keep one baseline in whatever face
 * the conversation is read in — a terminal face and a proportional one put
 * their letters at different heights inside the same line box, and a row built
 * from three boxes would drift between them. The time goes to the row's end,
 * and the page a question was asked from follows it: it belongs to the
 * question, not under its words.
 *
 * It is the whole of what used to be said by a visually hidden label. A row a
 * reader can see says the same thing to a reader who cannot.
 */
function TurnHead({
  who,
  name,
  at,
  children,
}: {
  who: "human" | "partner";
  name: string;
  at: string;
  children?: ReactNode;
}) {
  const when = whenOf(at);
  return (
    <p className="ms-turn-head">
      <span className="ms-turn-who">
        {who === "human" ? (
          <User className="ms-turn-mark" size={MARK} strokeWidth={1.75} aria-hidden="true" />
        ) : (
          <MessageSquare className="ms-turn-mark" size={MARK} strokeWidth={1.75} aria-hidden="true" />
        )}
        {name}
      </span>
      {when !== "" && <span className="ms-turn-when">{when}</span>}
      {children}
    </p>
  );
}

/**
 * The capture a question was asked from, as a label that goes back to it. It
 * stands in the question's own header row, after the time: it says something
 * about the asking, which is what that row is for.
 *
 * Clicking it opens that page with the view, filters, window and tab the
 * capture carries, through the board's own landing; the page then says what it
 * applied, and names anything it could not.
 */
function AskedFrom({ capture }: { capture: Page }) {
  const navigate = useNavigate();
  const said = chipOf(capture);
  const to = capture.return ?? capture.path;
  // What was open over that page, which is part of where the human was and no
  // part of where the chip goes: the address carries no sheet, so going back
  // reopens nothing.
  const sheet = sheetNote(capture);
  if (said === "" && sheet === "") {
    return null;
  }
  return (
    <>
      {said !== "" && (
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
      )}
      {sheet !== "" && <span className="ms-partner-asked-with">{sheet}</span>}
    </>
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
  return (
    <div className="ms-turn ms-turn--partner">
      {/* A turn that has not finished has no instant to stamp: the time
          arrives with the answer the server writes down. */}
      <TurnHead who="partner" name={PARTNER} at="" />
      <div className="ms-turn-body">
        {live.text !== "" ? (
          <Paragraphs text={live.text} />
        ) : (
          <p className="ms-partner-working">{live.doing === "" ? "Thinking…" : live.doing}</p>
        )}
        {/* A card arrives as the server admits it, which can be before the
            answer's last word. It is under the words either way. */}
        {live.suggestions.map((suggestion, at) => (
          <SuggestionCard key={idOf(live.turn, at)} id={idOf(live.turn, at)} />
        ))}
        {live.deposits.map((deposit, at) => (
          <DepositCard key={depositID(live.turn, at)} id={depositID(live.turn, at)} />
        ))}
      </div>
    </div>
  );
}

/** A refusal, in the server's own words, with the line that installs it. */
function Refusal({ reason, install }: { reason: string; install: string }) {
  return (
    <div className="ms-turn ms-turn--partner">
      <TurnHead who="partner" name={PARTNER} at="" />
      <div className="ms-turn-body">
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
