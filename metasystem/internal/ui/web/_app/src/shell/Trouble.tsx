import { useId, useRef, type ReactNode } from "react";

import { Button } from "./controls";
import { useTroubles } from "./troubles";
import { ASK_WHAT_HAPPENED, troubleOf, type TroubleAct } from "./troubling";
import { activeSection, reviewIdFromPath, roomIdFromPath } from "../routes";

/**
 * The trouble line (g1-s68 D1): the one shape everything that goes wrong on
 * screen renders through.
 *
 * The sentence is exactly what the site said before, with the role it had. At
 * its end, quietly, one control: Ask what happened. The press is the question
 * — the Partner's store sends one fixed sentence in the human's name and the
 * trouble travels beside it — so there is nothing to type and nothing to copy.
 *
 * What travels is what a reader needs and no more: the sentence, scrubbed of
 * every secret the page holds; the code where there is one; where it happened;
 * the act as words, never the request's arguments; when; and whether the
 * sign-in sheet is the remedy.
 *
 * The control is drawn only where a press can reach a colleague: until the
 * Partner has registered its ask, the line shows no control at all, and where
 * the conversation's own renderer is what broke, the line says so and offers
 * the reload instead of a control whose answer nobody could read.
 */

type Tag = "p" | "span" | "li" | "div";

export function Trouble({
  text,
  children,
  role,
  as: Element = "p",
  variant = "line",
  code,
  act,
  subject,
  at,
  signIn,
  onAsked,
  broken,
  askable = true,
}: {
  /** The sentence exactly as the screen says it: what travels. */
  text: string;
  /** How the sentence is drawn, where that is more than its words. */
  children?: ReactNode;
  role?: "status" | "alert";
  as?: Tag;
  /** A line under a card, a smaller one, one in a box, or a whole pane's. */
  variant?: "line" | "small" | "boxed" | "pane";
  code?: string;
  act?: TroubleAct;
  /** What the page is about, where the site knows better than the page. */
  subject?: { id: string; kind: string };
  /** When it happened, where the site knows: a notification's own time, not when its row was drawn. */
  at?: string;
  signIn?: boolean;
  /** What the press closes first, so the answer can be read: the bell's panel, the sign-in sheet. */
  onAsked?: () => void;
  /** The sentence that stands in for the control where the conversation itself cannot be shown. */
  broken?: string;
  /**
   * False where the line is the conversation's own refusal to take a turn: a
   * press there would meet the same refusal, and nobody could answer it.
   */
  askable?: boolean;
}) {
  const { ask, pending, secrets } = useTroubles();
  const origin = useId();
  // When it happened is when the line first said it, not when it was asked —
  // or, where the site knows the moment itself, that moment.
  const seen = useRef({ text, at: at ?? new Date().toISOString() });
  if (seen.current.text !== text || (at !== undefined && seen.current.at !== at)) {
    seen.current = { text, at: at ?? new Date().toISOString() };
  }
  const waiting = pending.find((entry) => entry.origin === origin);

  const press = () => {
    if (ask === null) {
      return;
    }
    const pathname = globalThis.location.pathname;
    const room = roomIdFromPath(pathname);
    const section =
      room !== "" ? (reviewIdFromPath(pathname) !== "" ? "Review" : "Sitting") : (activeSection(pathname)?.title ?? "");
    const trouble = troubleOf(
      {
        text,
        code,
        where: { section, path: pathname, subject: subject?.id, kind: subject?.kind },
        act,
        at: seen.current.at,
        signIn,
      },
      secrets(),
    );
    onAsked?.();
    ask(trouble, origin);
  };

  const classes = variant === "line" ? "ms-trouble" : `ms-trouble ms-trouble--${variant}`;
  return (
    <Element className={classes} role={role}>
      {variant === "pane" ? (
        <div className="ms-trouble-text">{children ?? text}</div>
      ) : (
        <span className="ms-trouble-text">{children ?? text}</span>
      )}
      {broken !== undefined ? (
        <span className="ms-trouble-broken">
          {broken}{" "}
          <Button
            onClick={() => {
              location.reload();
            }}
          >
            Reload page
          </Button>
        </span>
      ) : (
        ask !== null &&
        askable && (
          <button type="button" className="ms-trouble-ask" onClick={press}>
            {ASK_WHAT_HAPPENED}
          </button>
        )
      )}
      {waiting !== undefined && <span className="ms-trouble-waiting">{waiting.label}</span>}
    </Element>
  );
}
