import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { NavLink } from "react-router";

import { editDocument, failureMessage, isStale, loadDocument, type DocumentPayload } from "./api";
import {
  ANSWER,
  answerable,
  answerWords,
  cardsOf,
  CritiqueError,
  decideFinding,
  DEFER,
  FOLD,
  foldAsk,
  foldAskedIn,
  fundingGoal,
  loadCritique,
  NOT_THIS,
  pressRefusal,
  REFUTE,
  rowFor,
  SEND,
  sectionFor,
  sendToCritique,
  stateLine,
  USE,
  type Card,
  type DesignAnswer,
  type DesignLoop,
  type DesignRound,
  type Draft,
  type FoldAsked,
  type Press,
} from "./critiquing";
import {
  comparisonOf,
  foldAt,
  foldConflicted,
  foldRow,
  foldRowFailed,
  foldRowWritten,
  foldUse,
  foldWritten,
  inTheDesign,
  offersFor,
  type Fold,
} from "./folding";
import { headingsOf } from "./sections";
import { Help } from "../help/Help";
import { usePartner } from "../partner/store";
import { documentPath } from "../routes";
import { Button, Chip } from "../shell/controls";
import { useSession } from "../shell/identity";
import { Sheet } from "../shell/Sheet";
import { Trouble } from "../shell/Trouble";

/**
 * A design's critique, on the design's own page (g1-s66 §3).
 *
 * Send to critique is a sheet beside the status; the chain's state is one line;
 * each round's findings stand under a heading of their own as cards with three
 * presses; a fold's section drafted anew stands under its card with old and new
 * side by side; and Answer the round appears when every card has its row. What
 * the engine did after either act is shown in its own words and nowhere
 * computed here.
 *
 * Every request is made because a human pressed something or opened the page.
 * There is no timer: the bell says a round is back, and the page is read again
 * when the human comes to it or presses Refresh.
 */

/** The press open on one card, with what has been typed for it. */
type Pressing = { key: string; press: Press; draft: Draft; heading: string; refusal: string };

function keyOf(card: Card): string {
  return `${String(card.round)}:${card.finding.id}`;
}

/** The critique block's container: the requests, and what they answered. */
export function Critique({
  document,
  goalStates,
  sendAsked,
  onSendClosed,
  onSaved,
}: {
  document: DocumentPayload;
  /** The ledger state of each goal the design names, for the funding goal. */
  goalStates: Readonly<Record<string, string>>;
  /** Whether the head's Send to critique asked for the sheet. */
  sendAsked: boolean;
  onSendClosed: () => void;
  /** The design as read again from disk, after a Use or a close wrote it. */
  onSaved: (payload: DocumentPayload) => void;
}) {
  const [loop, setLoop] = useState<DesignLoop | null>(null);
  const [problem, setProblem] = useState("");
  const [words, setWords] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [pressing, setPressing] = useState<Pressing | null>(null);
  const [folds, setFolds] = useState<Readonly<Record<string, Fold>>>({});
  const { offered, send, dismiss, store } = usePartner();
  const { askToSignIn } = useSession();
  const retried = useRef(false);
  const headings = useMemo(() => headingsOf(document.source), [document.source]);

  const read = useCallback(() => {
    loadCritique(document.id)
      .then((answered) => {
        setLoop(answered);
        setProblem("");
      })
      .catch((error: unknown) => {
        setProblem(failureMessage(error));
      });
  }, [document.id]);

  useEffect(() => {
    read();
  }, [read]);

  /** A signed-out act opens the sign-in sheet and is made once more. */
  const refusedAct = (error: unknown, again: () => void): string => {
    if (error instanceof CritiqueError && error.signIn && !retried.current) {
      retried.current = true;
      askToSignIn(again);
      return "";
    }
    return failureMessage(error);
  };

  /** The design as it now reads from disk, after an act that may have written it. */
  const reread = () => {
    loadDocument(document.id)
      .then(onSaved)
      .catch(() => {
        // The critique still reads; the page keeps the design it has.
      });
  };

  const answer = () => {
    const newest = loop?.rounds[loop.rounds.length - 1];
    if (loop === null || newest === undefined || busy) {
      return;
    }
    setBusy(true);
    sendToCritique(document.id, { goal: loop.goal ?? "", toolCalls: loop.toolCalls, after: newest.round })
      .then((answered) => {
        setBusy(false);
        setWords(answerWords(answered));
        read();
        reread();
      })
      .catch((error: unknown) => {
        setBusy(false);
        const said = refusedAct(error, answer);
        if (said !== "") {
          setWords([said]);
        }
      });
  };

  const decide = (card: Card, press: Press, draft: Draft, then?: () => void) => {
    const refusal = pressRefusal(press, card.finding, draft);
    if (refusal !== "") {
      setPressing((held) => (held === null ? held : { ...held, refusal }));
      return;
    }
    setBusy(true);
    decideFinding(document.id, card.round, rowFor(press, card.finding, draft))
      .then((answered) => {
        setBusy(false);
        setLoop(answered);
        setPressing(null);
        then?.();
      })
      .catch((error: unknown) => {
        setBusy(false);
        const said = refusedAct(error, () => {
          decide(card, press, draft, then);
        });
        setPressing((held) => (held === null ? held : { ...held, refusal: said }));
      });
  };

  const askFold = (card: Card, heading: string, amendment: string) => {
    setPressing(null);
    send(foldAsk(document.id, heading, card.finding, card.round, loop?.chain ?? "", amendment));
  };

  // The sections the Partner drafted for this design, each bound to the finding
  // its request named, the newest per finding.
  const sections = useMemo(() => offersFor(document.id, offered, store.messages), [offered, store.messages, document.id]);
  // The folds asked of the Partner on this chain, which their cards say.
  const asking = useMemo(
    () => store.messages.flatMap((message) => {
      const fold = message.role === "human" ? foldAskedIn(message.text) : null;
      return fold !== null && fold.design === document.id && fold.chain === loop?.chain ? [fold] : [];
    }),
    [store.messages, document.id, loop?.chain],
  );

  const setFold = (id: string, fold: Fold) => {
    setFolds((held) => ({ ...held, [id]: fold }));
  };

  const writeRow = (id: string, fold: Fold, asked: FoldAsked | undefined) => {
    const owed = foldRow(loop, asked);
    if (owed === undefined) {
      setFold(id, foldRowFailed(fold, "the finding is not on this page any more"));
      return;
    }
    decideFinding(document.id, owed.round, owed.row)
      .then((answered) => {
        setLoop(answered);
        setFold(id, foldRowWritten(fold));
      })
      .catch((error: unknown) => {
        setFold(id, foldRowFailed(fold, failureMessage(error)));
      });
  };

  // A Use writes the row only for the finding the draft was asked for.
  const use = (id: string, fold: Fold, asked: FoldAsked | undefined) => {
    const step = foldUse(fold);
    setFold(id, step.fold);
    if (step.save === undefined) {
      return;
    }
    const owes = foldRow(loop, asked) !== undefined;
    editDocument(document.id, step.save.source, step.save.revision)
      .then((payload) => {
        onSaved(payload);
        const written = foldWritten(step.fold, owes);
        setFold(id, written);
        if (owes) {
          writeRow(id, written, asked);
        }
      })
      .catch((error: unknown) => {
        if (!isStale(error)) {
          setFold(id, { ...step.fold, phase: "refused", said: failureMessage(error) });
          return;
        }
        // The writer checks the revision, not the section: read the design
        // again and compare against the words as they are now.
        loadDocument(document.id)
          .then((now) => {
            onSaved(now);
            setFold(id, foldConflicted(step.fold, now.source, now.revision));
          })
          .catch((reading: unknown) => {
            setFold(id, { ...step.fold, phase: "refused", said: failureMessage(reading) });
          });
      });
  };

  if (loop === null) {
    return problem === "" ? null : (
      <p className="ms-project-reason" role="status">
        The critique could not be read: {problem}
      </p>
    );
  }

  return (
    <>
      <CritiqueBlock
        loop={loop}
        words={words}
        busy={busy}
        onAnswer={answer}
        renderRound={(round, live) => (
          <RoundCards
            key={round.round}
            round={round}
            live={live}
            headings={headings}
            pressing={pressing}
            busy={busy}
            asked={asking}
            onOpen={(card, press) => {
              setPressing({ key: keyOf(card), press, draft: { reasoning: "", amendment: "" },
                heading: sectionFor(card.finding, headings), refusal: "" });
            }}
            onChange={(next) => {
              setPressing((held) => (held === null ? held : { ...held, ...next, refusal: "" }));
            }}
            onCancel={() => {
              setPressing(null);
            }}
            onWrite={(card) => {
              if (pressing !== null) {
                decide(card, pressing.press, pressing.draft);
              }
            }}
            onFold={(card) => {
              if (pressing === null) {
                return;
              }
              const refusal = pressing.heading === "" ? "Choose the section this finding is folded into." : pressRefusal("fold", card.finding, pressing.draft);
              if (refusal !== "") {
                setPressing({ ...pressing, refusal });
                return;
              }
              askFold(card, pressing.heading, pressing.draft.amendment);
            }}
          />
        )}
      />
      {sections.map((section) => {
        const fold = folds[section.id] ?? foldAt(section, loop, document.source, document.revision);
        const settled = fold.phase === "comparing" && folds[section.id] === undefined && inTheDesign(document.source, section.heading, section.text);
        return (
          <SectionCard
            key={section.id}
            fold={fold}
            settled={settled}
            asked={section.asked}
            onUse={() => {
              use(section.id, fold, section.asked);
            }}
            onRow={() => {
              writeRow(section.id, fold, section.asked);
            }}
            onDismiss={() => {
              dismiss(section.id);
            }}
          />
        );
      })}
      {sendAsked && (
        <SendSheet
          loop={loop}
          goals={document.record?.goals ?? []}
          states={goalStates}
          onClose={onSendClosed}
          onSent={(answered) => {
            setWords(answerWords(answered));
            onSendClosed();
            read();
          }}
        />
      )}
    </>
  );
}

/**
 * The block under the facts strip: the heading, the chain's line, the engine's
 * words after the last act, Answer the round where it stands, and each round's
 * cards under a heading of its own.
 */
export function CritiqueBlock({
  loop,
  words,
  busy,
  onAnswer,
  renderRound,
}: {
  loop: DesignLoop;
  words: readonly string[];
  busy: boolean;
  onAnswer: () => void;
  renderRound: (round: DesignRound, live: boolean) => React.ReactNode;
}) {
  const line = stateLine(loop);
  if (line === "" && words.length === 0) {
    return null;
  }
  const live = loop.state === "deciding" || loop.state === "answered";
  return (
    <section className="ms-critique" aria-label="Critique">
      <h2 className="ms-facts-work-title">
        Critique
        <Help id="critique" />
      </h2>
      {line !== "" && (
        <p className="ms-critique-line" role="status">
          {line}
        </p>
      )}
      {words.length > 0 && (
        <div className="ms-critique-words" role="status" aria-label="What the engine said">
          {words.map((said, at) => (
            <p key={`${String(at)}-${said}`} className={at === 0 ? "ms-critique-said" : "ms-critique-said-line"}>
              {said}
            </p>
          ))}
        </div>
      )}
      {answerable(loop) && (
        <p className="ms-critique-answer">
          <Button primary disabled={busy} onClick={onAnswer}>
            {ANSWER}
          </Button>
          <Help id="answer-the-round" />
        </p>
      )}
      {loop.rounds.map((round, at) => renderRound(round, live && at === loop.rounds.length - 1))}
    </section>
  );
}

/** One round: its heading and its cards, or its prose where it carries no findings. */
export function RoundCards({
  round,
  live,
  headings,
  pressing,
  busy,
  asked,
  onOpen,
  onChange,
  onCancel,
  onWrite,
  onFold,
}: {
  round: DesignRound;
  /** Whether this is the newest round of an open chain, the only one decided. */
  live: boolean;
  headings: readonly string[];
  pressing: Pressing | null;
  busy: boolean;
  /** The folds asked of the Partner on this chain. */
  asked: readonly FoldAsked[];
  onOpen: (card: Card, press: Press) => void;
  onChange: (next: Partial<Pick<Pressing, "draft" | "heading">>) => void;
  onCancel: () => void;
  onWrite: (card: Card) => void;
  onFold: (card: Card) => void;
}) {
  return (
    <div className="ms-critique-round">
      <h3 className="ms-critique-round-title">Round {round.round}</h3>
      {round.prose !== undefined && round.prose !== "" ? (
        <pre className="ms-critique-prose">{round.prose}</pre>
      ) : round.findings.length === 0 ? (
        <p className="ms-project-note">No findings.</p>
      ) : (
        <ul className="ms-critique-cards">
          {cardsOf(round).map((card) => {
            const open = pressing !== null && pressing.key === keyOf(card) ? pressing : null;
            const folding = asked.some((one) => one.round === card.round && one.finding === card.finding.id);
            return (
              <FindingCard
                key={keyOf(card)}
                card={card}
                live={live}
                headings={headings}
                pressing={open}
                busy={busy}
                folding={folding}
                onOpen={(press) => {
                  onOpen(card, press);
                }}
                onChange={onChange}
                onCancel={onCancel}
                onWrite={() => {
                  onWrite(card);
                }}
                onFold={() => {
                  onFold(card);
                }}
              />
            );
          })}
        </ul>
      )}
    </div>
  );
}

/** What a decided card says about its row, as the file holds it. */
function decidedLine(card: Card): string {
  const row = card.row;
  if (row === undefined) {
    return "";
  }
  const words: Record<string, string> = { accepted: "Accepted", refuted: "Refuted", noted: "Noted", "out-of-scope": "Out of scope" };
  const said = [words[row.disposition] ?? row.disposition, row.reasoning, row.amendment].filter((part) => part !== "");
  return said.join(" · ");
}

/** A piece of evidence, as a chip, and as a link where it names a file of the checkout. */
function EvidenceChips({ evidence }: { evidence: string }) {
  const pieces = evidence.split(/[;,]\s+|\n/).map((piece) => piece.trim()).filter((piece) => piece !== "");
  return (
    <span className="ms-critique-evidence">
      {pieces.map((piece) => {
        const file = /^([\w./-]+\/[\w.-]+\.\w+)(?::[\d-]+)?$/.exec(piece);
        return file === null ? (
          <Chip key={piece}>{piece}</Chip>
        ) : (
          <NavLink key={piece} className="ms-chip ms-critique-anchor" to={documentPath(file[1])}>
            {piece}
          </NavLink>
        );
      })}
    </span>
  );
}

/** One finding of one round, and its three presses. */
export function FindingCard({
  card,
  live,
  headings,
  pressing,
  busy,
  folding,
  onOpen,
  onChange,
  onCancel,
  onWrite,
  onFold,
}: {
  card: Card;
  live: boolean;
  headings: readonly string[];
  pressing: Pressing | null;
  busy: boolean;
  /** Whether a fold of this finding was asked of the Partner. */
  folding: boolean;
  onOpen: (press: Press) => void;
  onChange: (next: Partial<Pick<Pressing, "draft" | "heading">>) => void;
  onCancel: () => void;
  onWrite: () => void;
  onFold: () => void;
}) {
  const { finding } = card;
  const id = `ms-finding-${String(card.round)}-${finding.id}`;
  return (
    <li className="ms-critique-card" data-finding={finding.id} data-decided={card.row === undefined ? "no" : "yes"}>
      <p className="ms-critique-card-head">
        <span className="ms-mono">{finding.id}</span>
        <Chip>{finding.severity}</Chip>
        <Chip marker={finding.material}>{finding.material ? "material" : "not material"}</Chip>
        <Help id="finding-card" />
      </p>
      <p className="ms-critique-claim">{finding.claim.split("\n")[0]}</p>
      {finding.evidence !== "" && <EvidenceChips evidence={finding.evidence} />}
      {finding.change !== undefined && finding.change !== "" && (
        <p className="ms-critique-asks">
          <span className="ms-facts-link-label">It asks</span> {finding.change}
        </p>
      )}
      {finding.tests !== undefined && finding.tests !== "" && (
        <p className="ms-critique-asks">
          <span className="ms-facts-link-label">Tests</span> {finding.tests}
        </p>
      )}
      {card.row !== undefined ? (
        <p className="ms-critique-decided" role="status">
          {decidedLine(card)}
        </p>
      ) : !live ? null : pressing === null ? (
        <p className="ms-critique-presses">
          <Button disabled={busy} onClick={() => { onOpen("fold"); }}>{FOLD}</Button>
          <Button disabled={busy} onClick={() => { onOpen("refute"); }}>{REFUTE}</Button>
          <Button disabled={busy} onClick={() => { onOpen("defer"); }}>{DEFER}</Button>
          {folding && <span className="ms-project-note">Asked your Project Partner for the section; it stands below when it is drafted.</span>}
        </p>
      ) : (
        <div className="ms-critique-press">
          {pressing.press === "fold" && (
            <>
              <label htmlFor={`${id}-section`}>The section it is folded into</label>
              <select
                id={`${id}-section`}
                value={pressing.heading}
                onChange={(event) => {
                  onChange({ heading: event.target.value });
                }}
              >
                <option value="">Choose a section</option>
                {headings.map((heading) => (
                  <option key={heading} value={heading}>
                    {heading}
                  </option>
                ))}
              </select>
              <label htmlFor={`${id}-amendment`}>The amendment, in one line</label>
              <input
                id={`${id}-amendment`}
                type="text"
                value={pressing.draft.amendment}
                onChange={(event) => {
                  onChange({ draft: { ...pressing.draft, amendment: event.target.value } });
                }}
              />
            </>
          )}
          {pressing.press !== "fold" && (
            <>
              <label htmlFor={`${id}-reason`}>
                {pressing.press === "refute"
                  ? "Your reason: the check you made and what it showed"
                  : finding.material
                    ? "The evidence that it is outside the brief's scope"
                    : "A note, if you want one"}
              </label>
              <textarea
                id={`${id}-reason`}
                rows={3}
                value={pressing.draft.reasoning}
                onChange={(event) => {
                  onChange({ draft: { ...pressing.draft, reasoning: event.target.value } });
                }}
              />
            </>
          )}
          {pressing.refusal !== "" && <Trouble text={pressing.refusal} role="status" variant="small" />}
          <p className="ms-critique-presses">
            {pressing.press === "fold" ? (
              <Button primary disabled={busy} onClick={onFold}>
                Ask the Partner for this section
              </Button>
            ) : (
              <Button primary disabled={busy} onClick={onWrite}>
                {pressing.press === "refute" ? "Write refuted" : finding.material ? "Write out of scope" : "Write noted"}
              </Button>
            )}
            <button type="button" className="ms-act-link" disabled={busy} onClick={onCancel}>
              Cancel
            </button>
          </p>
        </div>
      )}
    </li>
  );
}

/**
 * A section drafted anew: old and new side by side with the changed lines
 * marked; Use writes exactly that section; Not this dismisses it. A heading this
 * page cannot tell apart is refused in words with the draft kept; after a
 * revision conflict the comparison is the design as it is now.
 */
export function SectionCard({
  fold,
  settled,
  asked,
  onUse,
  onRow,
  onDismiss,
}: {
  fold: Fold;
  /** Whether the design already carries the section as drafted. */
  settled: boolean;
  /** The fold whose request this draft answers: the one finding whose accepted row a Use writes. */
  asked: FoldAsked | undefined;
  onUse: () => void;
  onRow: () => void;
  onDismiss: () => void;
}) {
  const compared = comparisonOf(fold);
  return (
    <section className="ms-section-card" aria-label={`The section ${fold.heading}, drafted anew`}>
      <p className="ms-critique-card-head">
        <span>
          “{fold.heading}”, drafted anew{asked === undefined ? "" : ` for ${asked.finding} of round ${String(asked.round)}`}
        </span>
        <Help id="section-card" />
      </p>
      {fold.said !== "" && <Trouble text={fold.said} role="status" variant="small" />}
      {compared.state === "refused" && fold.said === "" && <Trouble text={compared.said} role="status" variant="small" />}
      {compared.state === "compared" && (
        <div className="ms-section-sides">
          <SectionSide title="As it stands" lines={compared.old} />
          <SectionSide title="As drafted" lines={compared.new} />
        </div>
      )}
      {compared.state === "refused" && <pre className="ms-critique-prose">{fold.text}</pre>}
      <p className="ms-critique-presses">
        {fold.phase === "done" || settled ? (
          <span className="ms-suggestion-said" role="status">
            Used · the section is in the design
          </span>
        ) : fold.phase === "row" ? (
          <Button primary onClick={onRow}>
            Write the decision
          </Button>
        ) : (
          <>
            <Button primary disabled={fold.phase === "writing" || compared.state === "refused"} onClick={onUse}>
              {USE}
            </Button>
            <Button disabled={fold.phase === "writing"} onClick={onDismiss}>
              {NOT_THIS}
            </Button>
          </>
        )}
      </p>
    </section>
  );
}

function SectionSide({ title, lines }: { title: string; lines: readonly { text: string; changed: boolean }[] }) {
  return (
    <div className="ms-section-side">
      <h4 className="ms-section-side-title">{title}</h4>
      <pre className="ms-section-lines">
        {lines.map((line, at) => (
          <span key={`${String(at)}-${line.text}`} className="ms-section-line" data-changed={line.changed ? "yes" : "no"}>
            {line.changed ? "▍" : " "} {line.text}
            {"\n"}
          </span>
        ))}
      </pre>
    </div>
  );
}

/** The Send to critique sheet: the funding goal and the reader budget, both editable. */
export function SendSheet({
  loop,
  goals,
  states,
  onClose,
  onSent,
}: {
  loop: DesignLoop;
  goals: readonly string[];
  states: Readonly<Record<string, string>>;
  onClose: () => void;
  onSent: (answer: DesignAnswer) => void;
}) {
  const funding = fundingGoal(goals, states);
  const [goal, setGoal] = useState(funding.chosen);
  const [budget, setBudget] = useState(String(loop.toolCalls));
  const [refusal, setRefusal] = useState("");
  const [busy, setBusy] = useState(false);
  const { askToSignIn } = useSession();
  const retried = useRef(false);

  const send = () => {
    setBusy(true);
    setRefusal("");
    sendToCritique(loop.design, { goal, toolCalls: Number(budget) })
      .then((answered) => {
        setBusy(false);
        onSent(answered);
      })
      .catch((error: unknown) => {
        setBusy(false);
        if (error instanceof CritiqueError && error.signIn && !retried.current) {
          retried.current = true;
          askToSignIn(send);
          return;
        }
        setRefusal(failureMessage(error));
      });
  };

  return (
    <Sheet
      open
      onOpenChange={(next) => {
        if (!next) {
          onClose();
        }
      }}
      side="right"
      label={SEND}
      title={SEND}
      closeLabel="Close without sending"
      bodyClassName="ms-sheet-body--launch"
      sheetName={SEND}
    >
      <SendFields
        goals={goals}
        states={states}
        goal={goal}
        budget={budget}
        refusal={refusal}
        busy={busy}
        onGoal={setGoal}
        onBudget={setBudget}
        onSend={send}
      />
    </Sheet>
  );
}

/** The sheet's fields, which the tests read from the markup. */
export function SendFields({
  goals,
  states,
  goal,
  budget,
  refusal,
  busy,
  onGoal,
  onBudget,
  onSend,
}: {
  goals: readonly string[];
  states: Readonly<Record<string, string>>;
  goal: string;
  budget: string;
  refusal: string;
  busy: boolean;
  onGoal: (goal: string) => void;
  onBudget: (budget: string) => void;
  onSend: () => void;
}) {
  const funding = fundingGoal(goals, states);
  return (
    <>
      <p className="ms-launch-hint">
        The configured critique lane reads this design as its brief and returns findings you decide here.
        <Help id="send-to-critique" />
      </p>
      <div className="ms-launch-field">
        <label htmlFor="ms-critique-goal">The goal that funds it</label>
        <select
          id="ms-critique-goal"
          value={goal}
          onChange={(event) => {
            onGoal(event.target.value);
          }}
        >
          {funding.chosen === "" && <option value="">Choose the goal</option>}
          {goals.map((one) => (
            <option key={one} value={one}>
              {one} · {states[one] === undefined || states[one] === "" ? "not in the ledger" : states[one]}
            </option>
          ))}
        </select>
        {funding.choices.length === 0 && (
          <p className="ms-launch-hint">None of the goals this design names is approved; the engine will say so.</p>
        )}
      </div>
      <div className="ms-launch-field">
        <label htmlFor="ms-critique-budget">Reader budget, in tool calls</label>
        <input
          id="ms-critique-budget"
          type="number"
          min={1}
          value={budget}
          onChange={(event) => {
            onBudget(event.target.value);
          }}
        />
      </div>
      {refusal !== "" && <Trouble text={refusal} role="status" act={{ verb: SEND, object: "design" }} />}
      <p className="ms-critique-presses">
        <Button primary disabled={busy} onClick={onSend}>
          {SEND}
        </Button>
      </p>
    </>
  );
}
