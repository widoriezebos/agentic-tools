import { useEffect, useId, useRef, useState } from "react";
import { NavLink, useNavigate } from "react-router";

import { loadChanges, type Changes } from "./api";
import { DeskAnchors } from "./anchors";
import { FindingDecisions, FollowUp } from "./Answers";
import { Desk } from "./Desk";
import { TryIt } from "./Pill";
import { PendingVerdict } from "./Verdict";
import {
  ACCEPTED_LANDING,
  askedFollowing,
  briefOf,
  followUpsFor,
  LANDED_UNFINISHED,
  verdictAnswers,
  decisionOfAnswer,
  earlierTurnsOf,
  END_WITHOUT_SAID,
  examinationOf,
  findingsSummary,
  goalHeading,
  landingWaits,
  LEAVE_SAID,
  leadParagraph,
  lookedAt,
  nodPlan,
  nothingRaised,
  openingTurnOf,
  outcomeBody,
  recommendedWay,
  reviewedOf,
  roomFindingsOf,
  steppingOut,
  versionWhen,
  WALK_WORDS,
  WALKS,
  walksAsked,
  wayConsequence,
  type DeskItem,
  type NodPlan,
  type RoomFinding,
} from "./room";
import { loadBacklog, type Row } from "../backlog/api";
import { Help } from "../help/Help";
import { NotificationsBell } from "../notifications/Bell";
import { FindingLayers, DepositCard } from "../partner/Deposit";
import { FIX, followUp } from "../partner/sitting";
import { usePartner } from "../partner/store";
import { Transcript } from "../partner/Transcript";
import { backlogPath, roomPath } from "../routes";
import { useAbout } from "../shell/about";
import { Composer } from "../shell/Composer";
import { Button } from "../shell/controls";
import { Sheet } from "../shell/Sheet";
import { SignInControl } from "../shell/SignInControl";
import { Trouble } from "../shell/Trouble";
import "./room.css";

/**
 * A review you can decide (review-findings-read-as-decisions §3): one column,
 * four numbered steps, read top to bottom — what you are looking at, what the
 * reviewer found, try it, your verdict — and one question line at the foot.
 * The code and the conversation open as sheets when asked; nothing here is a
 * desk. Every press says what it does before it is pressed, and the nod never
 * refuses: it asks once where it must, and says the impact first (R-142-m1e,
 * R-143-m1e).
 */
export function GuidedReview({ record }: { record: string }) {
  const { store, table, conversation, verdictChanged } = usePartner();
  const reviewed = reviewedOf(table.source);
  const here = conversation === record && store.conversation === record && store.state !== "loading";
  const [changes, setChanges] = useState<Changes | null>(null);
  const [since, setSince] = useState<Changes | null>(null);
  const [row, setRow] = useState<Row | null>(null);
  // The version on screen by its time and its builder, and whether a newer one
  // exists: read once per version, and never polled — and again when a verdict
  // was refused because what it was about moved (F-3, RF-02).
  useEffect(() => {
    if (!here || reviewed.tip === "") {
      return;
    }
    const aborter = new AbortController();
    loadChanges(record, false, aborter.signal)
      .then(async (read) => {
        setChanges(read);
        setSince(read.moved ? await loadChanges(record, true, aborter.signal) : null);
      })
      .catch(() => {
        // A branch that cannot be read says the version without its time.
      });
    return () => {
      aborter.abort();
    };
  }, [here, record, reviewed.tip, verdictChanged]);

  // The goal's own sentence for the title, and the seat that holds it.
  useEffect(() => {
    if (reviewed.goal === "") {
      return;
    }
    const aborter = new AbortController();
    loadBacklog(aborter.signal)
      .then((read) => {
        setRow(read.rows.find((one) => one.ref.id === reviewed.goal) ?? null);
      })
      .catch(() => {
        // The goal is then named by its id.
      });
    return () => {
      aborter.abort();
    };
  }, [reviewed.goal]);

  return <ReviewView record={record} changes={changes} since={since} row={row} />;
}

/**
 * The guided review on screen, over the reads its room made: the change index
 * of the version on screen, what changed since where a newer one exists, and
 * the goal's own row.
 */
export function ReviewView({ record, changes, since, row }: {
  record: string; changes: Changes | null; since: Changes | null; row: Row | null;
}) {
  const partner = usePartner();
  const {
    store, sitting, table, room, deposits, putOnDesk, busy, keepRoomNow, stop, conversation, walk,
    verdictSaid, verdictRefusal, verdictChanged, retryVerdict, stranded, recordStranded, verdictBusy,
    reviewCurrentVersion, askAgain, giveVerdict, decideFinding,
  } = partner;
  const navigate = useNavigate();
  const reviewed = reviewedOf(table.source);
  const here = conversation === record && store.conversation === record && store.state !== "loading";
  const away = backlogPath(reviewed.goal);
  const [seeing, setSeeing] = useState(false);
  const [talking, setTalking] = useState(false);
  const [asking, setAsking] = useState<Asking | null>(null);
  const [said, setSaid] = useState("");
  const [pressing, setPressing] = useState(false);
  const [leaving, setLeaving] = useState(false);
  const title = goalHeading(reviewed.goal === "" ? record : reviewed.goal, row?.intent ?? "");

  useAbout(`Review of ${title}`, {
    kind: "document",
    subject: record,
    title: `Review of ${reviewed.goal}`,
    revision: table.revision,
    tab: "the review",
    returnTo: roomPath("review", record),
  });

  // A walk puts what it explains on the desk, which the code sheet shows.
  const presented = useRef({ turn: "", count: 0 });
  useEffect(() => {
    const live = store.live;
    if (live.turn !== presented.current.turn) {
      presented.current = { turn: live.turn, count: 0 };
    }
    const fresh = live.presents.slice(presented.current.count);
    presented.current.count = live.presents.length;
    if (live.turn === partner.stoppedPresenting) {
      return;
    }
    for (const present of fresh) {
      if (present.kind === "source" && present.path !== undefined) {
        putOnDesk({ kind: "source", path: present.path, from: present.from ?? 0, to: present.to ?? present.from ?? 0 });
      } else if (present.kind === "changes") {
        putOnDesk({ kind: "changes" });
      } else if (present.kind === "diff" && present.path !== undefined) {
        putOnDesk({ kind: "diff", path: present.path });
      }
    }
  }, [store.live, partner.stoppedPresenting, putOnDesk]);

  // A question the person asks opens the conversation, so its answer is seen.
  const asked = store.messages.filter((one) => one.role === "human" && one.interface !== true).length;
  const askedBefore = useRef(-1);
  useEffect(() => {
    if (askedBefore.current >= 0 && asked > askedBefore.current) {
      setTalking(true);
    }
    askedBefore.current = asked;
  }, [asked]);

  // The sitting ended — the verdict was recorded, or the person ended it — so
  // the room closes on the board, unless it has a verdict to say.
  const stood = useRef(false);
  useEffect(() => {
    if (sitting !== null) {
      stood.current = stood.current || here;
      return;
    }
    if (stood.current && here && verdictSaid === "") {
      void navigate(away);
    }
  }, [sitting, here, navigate, away, verdictSaid]);

  const messages = store.messages;
  const examination = examinationOf(messages, store.live.turn);
  const before = earlierTurnsOf(messages);
  const read = roomFindingsOf(deposits, table.entries, before);
  const findings = read.current;
  const moved = changes?.moved === true;
  const summary = findingsSummary(findings);
  // Nothing is marked recommended until the reviewer's look completed (RF-06,
  // fix round 1 R-143-m1e): an unfinished report recommends nothing.
  const way = examination !== "complete" ? "" : recommendedWay(findings);
  // Step 1's paragraph: the newest opening's, while it is being written its
  // words so far, and where it says none the paragraph an earlier opening of
  // this sitting wrote.
  const opening = openingTurnOf(messages);
  const openings = messages.filter((one) => one.role === "partner" && one.text.trim() !== "" && !before.includes(one.turn) &&
    messages.some((asked) => asked.turn === one.turn && asked.interface === true && asked.text.startsWith("Open this review")));
  const lead = leadParagraph(store.live.turn === opening && store.live.text.trim() !== "" ? store.live.text : "") ||
    [...openings].reverse().map((one) => leadParagraph(one.text)).find((said) => said !== "") || "";
  const examined = lookedAt(room.desk.items, walksAsked(messages));
  const builder = row?.claim?.machine ?? changes?.by ?? "";
  const when = versionWhen(changes?.at ?? "");

  const see = (item: DeskItem) => {
    putOnDesk(item);
    setSeeing(true);
  };

  const walkThrough = (part: string) => {
    setTalking(true);
    void walk(part);
  };

  const stepOut = async () => {
    const step = await steppingOut(busy, leaving, stop, () => keepRoomNow());
    if (step.kind === "warn") {
      setLeaving(true);
      return;
    }
    if (step.kind === "stay") {
      setSaid(step.said);
      return;
    }
    void navigate(away);
  };

  // One verdict, given: the answers it records, the person's own must-fix words
  // where they wrote them, and the Outcome composed from the record (§3).
  const give = async (verdict: "clear to land" | "send back" | "no verdict", answers: { id: string; answer: string }[],
    own: string, extra: string, followUps: string[], brief: string) => {
    const after: RoomFinding[] = findings.map((one) => {
      const given = answers.find((each) => each.id === one.id);
      return given === undefined ? one : { ...one, answer: given.answer, recorded: true };
    });
    if (own.trim() !== "") {
      after.push({ id: "", title: own.trim(), why: "", severity: "", recommend: "", reason: "", evidence: "", anchor: "",
        consequence: "", answer: FIX, recorded: true });
    }
    const body = `${outcomeBody(after, examined)}${extra === "" ? "" : `\n\n${extra}`}`;
    setPressing(true);
    setSaid("");
    const refused = await giveVerdict({ verdict, answers, own, outcome: body, tip: reviewed.tip, followUps, brief });
    setPressing(false);
    setSaid(refused);
    if (refused === "") {
      setAsking(null);
    }
  };

  const openedGoals = (list: readonly RoomFinding[]) =>
    list.filter((one) => decisionOfAnswer(one.answer) === "follow-up")
      .map((one) => one.answer.split(" — ").slice(1).join(" — ").replace(/^goal\s+/u, "").trim());

  // The verdict, once its ask is answered and its follow-up goals are open:
  // every answer it records in its one write (F-2), the impact a nod over
  // corrections or over an unfinished look carries into the Outcome
  // (RF-01, R-143-m1e), and a send-back's brief.
  const finish = (way: Way, plan: NodPlan, reason: string, own: string, opened: Readonly<Record<string, string>>) => {
    const answers = verdictAnswers(way, plan, reason, opened);
    if (way === "land") {
      const extra = [
        plan.corrections.length + plan.risks.length === 0 ? "" : `${ACCEPTED_LANDING} ${plan.impact} The reason given: ${reason.trim()}`,
        examination === "complete" ? "" : LANDED_UNFINISHED,
      ].filter((one) => one !== "").join("\n\n");
      void give("clear to land", answers, "", extra, [...openedGoals(findings), ...Object.values(opened)], "");
      return;
    }
    const listed = own.trim() === "" ? plan.corrections : [...plan.corrections, {
      id: "", title: own.trim(), why: "", severity: "", recommend: "", reason: "", evidence: "", anchor: "",
      consequence: "", answer: FIX, recorded: true,
    }];
    void give("send back", answers, own, "", [], briefOf(listed, record, reviewed.tip));
  };

  // One press of a way: at once where there is nothing to ask, else one ask —
  // the corrections and their impact on a nod, what must change on a
  // send-back with nothing to fix, and what the undecided findings will record.
  const press = (way: Way) => {
    const plan = nodPlan(findings);
    const asks = way === "land" ? plan.asks : plan.corrections.length === 0 || askedFollowing(plan, way).length > 0;
    if (!asks) {
      finish(way, plan, "", "", {});
      return;
    }
    setAsking({ way, plan, reason: "", own: "", queue: [], opened: {} });
  };

  const reviewCurrent = async () => {
    if (changes === null || changes.current === "") {
      return;
    }
    setPressing(true);
    setSaid(await reviewCurrentVersion(changes.current));
    setPressing(false);
  };

  const ways = (["land", "send back"] as const).slice().sort((one, two) => (one === way ? -1 : two === way ? 1 : 0));

  return (
    <div className="ms-guided">
      <header className="ms-guided-top">
        <NavLink to={away}>Back to the board</NavLink>
        <span className="ms-guided-top-right">
          <NotificationsBell />
          <SignInControl />
        </span>
      </header>
      <main className="ms-guided-main">
        <h1 className="ms-guided-title">
          Review of {title}
          <Help id="review-room" />
        </h1>
        <p className="ms-guided-facts">
          <span>
            {when === "" ? "The version under review" : `The version of ${when}`}
            {builder === "" ? "" : `, built by ${builder}`}
          </span>
          {reviewed.tip !== "" && (
            <span className="ms-guided-hint" title={`commit ${reviewed.tip}`} aria-label={`commit ${reviewed.tip}`}>
              i
            </span>
          )}
          {changes !== null && (
            moved ? (
              <span className="ms-guided-state ms-guided-state--newer">
                A newer version exists{changes.currentAt === undefined ? "" : `, from ${versionWhen(changes.currentAt)}`}.
              </span>
            ) : (
              <span className="ms-guided-state ms-guided-state--current">This is the version that would land.</span>
            )
          )}
        </p>
        {verdictSaid !== "" && (
          <p className="ms-guided-banner ms-guided-banner--done" role="status">
            {verdictSaid}{" "}
            <Button primary onClick={() => { void navigate(away); }}>
              Back to the board
            </Button>
          </p>
        )}
        {verdictChanged !== "" && <p className="ms-guided-banner" role="status">{verdictChanged}</p>}
        {stranded !== null && verdictRefusal === "" && !moved && (
          <PendingVerdict pending={stranded} busy={verdictBusy} onPress={recordStranded} />
        )}
        {verdictRefusal !== "" && (
          <p className="ms-guided-banner" role="status">
            Your verdict is written in the review, and not yet on the goal: {verdictRefusal}{" "}
            <Button onClick={retryVerdict}>Record the verdict again</Button>
          </p>
        )}
        {leaving && busy && (
          <p className="ms-guided-banner" role="status">
            The reviewer is answering. Leaving now stops the answer, and it is kept as stopped.{" "}
            <Button onClick={() => void stepOut()}>Leave anyway</Button>
            <Button onClick={() => { setLeaving(false); }}>Stay</Button>
          </p>
        )}
        {said !== "" && <Trouble text={said} role="status" variant="small" />}
        {store.state === "ready" && sitting === null && !stood.current && (
          <p className="ms-guided-banner" role="status">
            No review stands on this record. What it recorded stays in it; press Review it on the goal to begin another.
          </p>
        )}

        {verdictSaid === "" && (<>
        <section className="ms-guided-step" aria-labelledby="step-1">
          <h2 id="step-1"><span className="ms-guided-n">1</span> What you are looking at</h2>
          <p>{lead === "" ? "The reviewer is reading this version; what it adds appears here once it has looked." : lead}</p>
          {moved && (
            <div className="ms-guided-notice">
              <p>
                <strong>This review does not cover the version that would land.</strong> Since the reviewer looked, the
                builder made a newer version
                {changes?.currentAt === undefined ? "" : ` (${versionWhen(changes.currentAt)}`}
                {since === null ? (changes?.currentAt === undefined ? "" : ")") : `${changes?.currentAt === undefined ? " (" : "; "}${String(since.files.length)} ${since.files.length === 1 ? "file" : "files"} changed)`}
                . A verdict given here would be about the older version, and you would be asked for your word again.
              </p>
              <p>
                <Button primary disabled={pressing || busy} onClick={() => void reviewCurrent()}>
                  Review the current version
                </Button>{" "}
                <span className="ms-guided-quiet">
                  The reviewer looks at the current version afresh. What it found before stays marked as about the
                  older version; it raises again what still applies, with your earlier decision as its recommendation.
                </span>
              </p>
            </div>
          )}
        </section>

        <section className="ms-guided-step" aria-labelledby="step-2">
          <h2 id="step-2">
            <span className="ms-guided-n">2</span> What the reviewer found
            {moved && <small>in the older version</small>}
          </h2>
          {findings.length === 0 ? (
            <NothingRaised examination={examination} moved={moved} onAgain={() => { void askAgain(); }} />
          ) : moved ? (
            <p className="ms-guided-summary ms-guided-summary--older">
              <strong>{summary.counts.split(".")[0]}</strong> in the older version.{summary.counts.split(".").slice(1).join(".")} Decide
              them after the reviewer has looked at the current version.
            </p>
          ) : (
            <p className="ms-guided-summary">
              <strong>{summary.counts.split(".")[0]}.</strong>{summary.counts.split(".").slice(1).join(".")}
              <br />
              {examination === "complete" ? (
                <>
                  <strong>The reviewer recommends:</strong> {summary.recommends} If you decide nothing on a finding, it
                  follows the recommendation.
                </>
              ) : (
                <>
                  <strong>{examination === "reviewing"
                    ? "The reviewer has not finished looking at this version, so this list may still grow."
                    : "The reviewer's look at this version did not finish, so this list may not be whole."}</strong>{" "}
                  If you decide nothing on a finding, it follows the reviewer&apos;s recommendation.
                  {examination !== "reviewing" && (
                    <>
                      <br />
                      <Button onClick={() => { void askAgain(); }}>Ask the reviewer to look again</Button>
                    </>
                  )}
                </>
              )}
            </p>
          )}
          {findings.map((finding) => (
            <FindingLayers
              key={finding.id === "" ? finding.title : finding.id}
              finding={finding}
              muted={moved}
              onSee={see}
              decision={moved ? undefined : <FindingDecisions finding={finding} />}
            />
          ))}
          {read.earlier.length > 0 && (
            <details className="ms-guided-earlier">
              <summary>About the older version: {read.earlier.length} {read.earlier.length === 1 ? "finding" : "findings"}, with your decisions then</summary>
              {read.earlier.map((finding) => (
                <FindingLayers key={finding.id === "" ? finding.title : finding.id} finding={finding} muted onSee={see} />
              ))}
            </details>
          )}
          <p className="ms-guided-walks">
            Want the whole picture? Ask the reviewer to walk you through:{" "}
            {WALKS.map((one, at) => (
              <span key={one.part}>
                {at > 0 && " · "}
                <button type="button" className="ms-link-button" disabled={busy || sitting === null}
                  onClick={() => { walkThrough(one.part); }}>
                  {WALK_WORDS[one.part]}
                </button>
              </span>
            ))}
          </p>
        </section>

        <section className="ms-guided-step" aria-labelledby="step-3">
          <h2 id="step-3"><span className="ms-guided-n">3</span> Try it <small>optional</small></h2>
          <details className="ms-guided-try">
            <summary>Start this version on this computer</summary>
            {reviewed.goal !== "" && (
              <TryIt
                goal={reviewed.goal}
                reviewed={reviewed.tip}
                lead={moved
                  ? "Start runs the older version, the one this review looked at, on a port of its own. To try the current one, press Review the current version first."
                  : undefined}
              />
            )}
          </details>
        </section>

        <section className="ms-guided-step" aria-labelledby="step-4">
          <h2 id="step-4"><span className="ms-guided-n">4</span> Your verdict</h2>
          {moved ? (
            <div className="ms-guided-notice">
              <p>
                <strong>No verdict yet.</strong> This is not the version that would land, so a verdict here would not
                count. Press Review the current version above, then decide.
              </p>
            </div>
          ) : (
            <>
              <p>What happens to this goal?</p>
              <div className="ms-guided-ways">
                {ways.map((one) => (
                  <button
                    key={one}
                    type="button"
                    className={`ms-guided-way${one === way ? " ms-guided-way--recommended" : ""}`}
                    disabled={pressing || verdictBusy || sitting === null || reviewed.tip === ""}
                    onClick={() => { press(one); }}
                  >
                    <b>
                      {one === "land" ? "Looks good, land it" : "Send it back"}
                      {one === way && <span className="ms-guided-way-mark">recommended</span>}
                    </b>
                    <small>{wayConsequence(one, findings, examination)}</small>
                  </button>
                ))}
              </div>
            </>
          )}
          <p className="ms-guided-leave">
            Not ready?{" "}
            <button type="button" className="ms-link-button" onClick={() => void stepOut()}>
              Leave for now
            </button>
            : {LEAVE_SAID}{" "}
            <button type="button" className="ms-link-button" disabled={pressing || sitting === null}
              onClick={() => void give("no verdict", [], "", "", [], "")}>
              End without a verdict
            </button>
            : {END_WITHOUT_SAID}
          </p>
        </section>
        </>)}
      </main>
      <footer className="ms-guided-ask">
        <DeskAnchors.Provider value={see}>
          <Composer hint="Ask the reviewer anything about this version" seeing={`the review of ${reviewed.goal === "" ? title : reviewed.goal}`} />
        </DeskAnchors.Provider>
        <button type="button" className="ms-link-button ms-guided-talk" onClick={() => { setTalking(true); }}>
          Show the conversation
        </button>
      </footer>
      <Sheet
        open={talking}
        onOpenChange={setTalking}
        side="right"
        label="Your conversation with the reviewer"
        title="Your conversation with the reviewer"
        closeLabel="Close the conversation"
        bodyClassName="ms-guided-sheet"
        sheetName="The conversation with the reviewer"
      >
        <DeskAnchors.Provider value={see}>
          <div className="ms-dock-statement">
            <Transcript />
            {deposits.filter((card) => card.id.startsWith("local-") && card.standing !== "recorded").map((card) => (
              <DepositCard key={card.id} id={card.id} />
            ))}
          </div>
        </DeskAnchors.Provider>
      </Sheet>
      <Sheet
        open={seeing}
        onOpenChange={setSeeing}
        side="right"
        label="The change and what it cites"
        title="The change and what it cites"
        closeLabel="Close"
        bodyClassName="ms-guided-sheet ms-guided-code"
        sheetName="The change and what it cites"
      >
        <Desk record={record} />
      </Sheet>
      {/* The ask gives way to each follow-up goal's sheet in turn, so only one
          sheet holds the page at a time. */}
      {asking !== null && asking.queue.length === 0 && (
        <VerdictAsk
          asking={asking}
          busy={pressing}
          onChange={(next) => { setAsking({ ...asking, ...next }); }}
          onClose={() => { setAsking(null); }}
          onPress={() => {
            const queue = followUpsFor(asking.plan, asking.way);
            if (queue.length > 0) {
              setAsking({ ...asking, queue: queue.map((one) => one.id), opened: {} });
              return;
            }
            finish(asking.way, asking.plan, asking.reason, asking.own, {});
          }}
        />
      )}
      {asking !== null && asking.queue.length > 0 && (() => {
        const next = asking.plan.all.find((one) => one.id === asking.queue[0]);
        if (next === undefined) {
          return null;
        }
        return (
          <FollowUp
            key={next.id}
            intent={next.title}
            nextStep={next.why}
            onClose={() => {
              setAsking(null);
              setSaid(`The follow-up goal for "${next.title}" was not opened, so the verdict was not recorded. ` +
                "Decide that finding, or give your verdict again.");
            }}
            onOpened={(goal) => {
              void decideFinding(next.id, followUp(goal)).then((refused) => {
                if (refused !== "") {
                  setAsking(null);
                  setSaid(refused);
                  return;
                }
                const queue = asking.queue.slice(1);
                const opened = { ...asking.opened, [next.id]: goal };
                setAsking({ ...asking, queue, opened });
                if (queue.length === 0) {
                  finish(asking.way, asking.plan, asking.reason, asking.own, opened);
                }
              });
            }}
          />
        );
      })()}
    </div>
  );
}

/** The two ways a verdict goes. */
type Way = "land" | "send back";

/** What the verdict's one ask holds while it is open, and the follow-up goals it opens in turn. */
type Asking = {
  way: Way; plan: NodPlan; reason: string; own: string; queue: string[]; opened: Readonly<Record<string, string>>;
};

/** Step 2 where nothing was raised, said by how far the look got (RF-06), with Ask again where it did not finish. */
function NothingRaised({ examination, moved, onAgain }: { examination: ReturnType<typeof examinationOf>; moved: boolean; onAgain: () => void }) {
  const said = nothingRaised(examination);
  return (
    <div className={`ms-guided-summary${examination === "complete" && !moved ? " ms-guided-summary--clean" : ""}`}>
      <p>
        <strong>{said.lead}</strong>
        <br />
        {said.body}
      </p>
      {examination === "complete" && !moved && (
        <p>
          <strong>The reviewer recommends:</strong> land it.
        </p>
      )}
      {(examination === "stopped" || examination === "failed") && (
        <p>
          <Button onClick={onAgain}>Ask the reviewer to look again</Button>
        </p>
      )}
    </div>
  );
}

/**
 * A verdict's one ask (§3, RF-01, F-2). On a nod: the corrections that stand,
 * each by its title, what landing over them means, and one reason. On a
 * send-back with nothing to fix: what must change, in the person's words (§6
 * question 6). On both: the findings that follow the reviewer, each with what
 * will be recorded, and the follow-up goals that open in turn. Then the press.
 */
function VerdictAsk({
  asking,
  busy,
  onChange,
  onClose,
  onPress,
}: {
  asking: Asking;
  busy: boolean;
  onChange: (next: Partial<Asking>) => void;
  onClose: () => void;
  onPress: () => void;
}) {
  const reasonField = useId();
  const ownField = useId();
  const { plan, reason, own, way } = asking;
  const landing = way === "land";
  const writes = !landing && plan.corrections.length === 0;
  const needs = landing ? landingWaits(plan, reason) : writes && own.trim() === "";
  const title = landing ? "Before it lands" : "Send it back";
  const listed = askedFollowing(plan, way);
  return (
    <Sheet
      open
      onOpenChange={(open) => {
        if (!open) {
          onClose();
        }
      }}
      side="right"
      label={title}
      title={title}
      closeLabel={landing ? "Close without landing" : "Close without sending it back"}
      bodyClassName="ms-sitting-sheet ms-review-sheet"
      sheetName={title}
    >
      {landing && plan.corrections.length + plan.risks.length > 0 && (
        <>
          {plan.corrections.length > 0 && (
            <>
              <p className="ms-sitting-said">These must be fixed before landing:</p>
              <ul className="ms-guided-list">
                {plan.corrections.map((one) => <li key={one.id}>{one.title}</li>)}
              </ul>
            </>
          )}
          {plan.risks.length > 0 && (
            <>
              <p className="ms-sitting-said">These need your own decision before landing:</p>
              <ul className="ms-guided-list">
                {plan.risks.map((one) => (
                  <li key={one.id}>
                    {one.severity === "blocks" ? "Blocks landing: " : "The reviewer recommends accepting this risk: "}{one.title}
                  </li>
                ))}
              </ul>
            </>
          )}
          <p className="ms-sitting-said">{plan.impact}</p>
          <label className="ms-sitting-label" htmlFor={reasonField}>Your reason</label>
          <textarea id={reasonField} className="ms-deposit-field" rows={3} value={reason}
            onChange={(event) => { onChange({ reason: event.target.value }); }} />
          {needs && <p className="ms-deposit-needs" role="status">Write your reason first; it is recorded with each of them.</p>}
        </>
      )}
      {writes && (
        <>
          <p className="ms-sitting-said">
            Say what must change first. Your words are recorded as a finding of your own that must be fixed before
            landing, go to the builder as a correction, and the goal leaves Review until it comes back fixed.
          </p>
          <label className="ms-sitting-label" htmlFor={ownField}>What must change</label>
          <textarea id={ownField} className="ms-deposit-field" rows={4} value={own}
            onChange={(event) => { onChange({ own: event.target.value }); }} />
          {needs && <p className="ms-deposit-needs" role="status">Write what must change first.</p>}
        </>
      )}
      {listed.length > 0 && (
        <>
          <p className="ms-sitting-said">
            {listed.length === 1 ? "One finding follows" : `${String(listed.length)} findings follow`} the
            reviewer, because you decided nothing on {listed.length === 1 ? "it" : "them"}:
          </p>
          <ul className="ms-guided-list">
            {listed.map((one) => <li key={one.finding.id}>{one.finding.title} — {one.said}</li>)}
          </ul>
          {followUpsFor(plan, way).length > 0 && (
            <p className="ms-sitting-said">
              Each fix after landing opens the New goal sheet in turn; the verdict is recorded once each follow-up goal is open.
            </p>
          )}
        </>
      )}
      <div className="ms-sitting-foot">
        <Button primary disabled={busy || needs} onClick={onPress}>{landing ? "Land it" : "Send it back"}</Button>
        <Button disabled={busy} onClick={onClose}>Back</Button>
      </div>
    </Sheet>
  );
}
