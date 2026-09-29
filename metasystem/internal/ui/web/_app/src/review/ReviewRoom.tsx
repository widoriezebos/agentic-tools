import { useEffect, useRef, useState } from "react";
import { Group, Panel, Separator } from "react-resizable-panels";
import { NavLink, useLocation, useNavigate } from "react-router";

import { loadChanges } from "./api";
import { DeskAnchors } from "./anchors";
import { MovedFiles } from "./Answers";
import { Board } from "./Board";
import { Desk } from "./Desk";
import { CandidatePill } from "./Pill";
import { PendingVerdict } from "./Verdict";
import {
  BRIEF_SAID,
  CLEAR_REFUSED,
  correctionBrief,
  fixFindings,
  NO_FIX,
  RETIP_ASKS_END,
  standingOutcome,
  countsLine,
  deskLabel,
  endOf,
  movedLine,
  NOD_LINE,
  nodded,
  openingDesk,
  ownSnapshot,
  roomWord,
  unansweredIn,
  VERDICTS,
  walksOf,
  type DeskItem,
  reviewedOf,
  steppingOut,
  type Verdict,
} from "./room";
import { NotificationsBell } from "../notifications/Bell";
import type { Present } from "../partner/api";
import { DRAFTING, END, END_SAID, END_WITHOUT, START, type Entry } from "../partner/sitting";
import { DepositCard } from "../partner/Deposit";
import { usePartner } from "../partner/store";
import { Transcript } from "../partner/Transcript";
import { Help } from "../help/Help";
import { OPEN_FROM, openGoalPath } from "../project/critiquing";
import { pilesLine } from "../project/pane";
import { backlogPath, documentPath, roomPath, SITTING_PREFIX } from "../routes";
import { useAbout } from "../shell/about";
import { Composer } from "../shell/Composer";
import { Button } from "../shell/controls";
import { Sheet } from "../shell/Sheet";
import "./room.css";
import { Trouble } from "../shell/Trouble";

/**
 * The room (g1-s65 §3), for every sitting (g1-s67 D1): one screen, two panes.
 * The desk is the laptop and the conversation stays beside it; the desk flips
 * to the board and back. The rail is not here and neither is the drawer,
 * because the room's own pane is the sitting's conversation (D2, D16), and
 * nothing else on screen pulls at the human sitting on one record. What differs
 * by purpose is the header's word, the walks, what the desk reads, and End: a
 * review examines one goal's built work, a shaping sitting shapes an intent or
 * a design with the code beside it as the checkout has it today.
 */
export function Room({ record }: { record: string }) {
  const partner = usePartner();
  const { store, sitting, table, room, setFace, putOnDesk, openDesk, busy, keepRoomNow, stop, conversation,
    verdictSaid, verdictRefusal, retryVerdict, stranded, recordStranded, verdictBusy } = partner;
  const navigate = useNavigate();
  const location = useLocation();
  // What the sitting is for, which the mark says; before the mark has been read
  // the address says which kind of room this is.
  const purpose = sitting?.purpose ?? (location.pathname.startsWith(SITTING_PREFIX) ? "shape a design" : "review");
  const reviewing = purpose === "review";
  const [ending, setEnding] = useState(false);
  const [leaving, setLeaving] = useState(false);
  const [stayed, setStayed] = useState("");
  const [moved, setMoved] = useState<{ current: string; changed: string[] } | null>(null);
  const [retipRefusal, setRetipRefusal] = useState("");
  const [retipSaid, setRetipSaid] = useState("");
  const reviewed = reviewedOf(table.source);
  const findings = table.entries.filter((entry) => entry.section === "Findings");
  const unanswered = unansweredIn(table.entries);
  // The snapshot read for this room, and not the one the page held before it.
  const here = conversation === record && ownSnapshot(store, record);
  const named = reviewing
    ? reviewed.goal === "" ? record : reviewed.goal
    : (sitting?.subject.title ?? "") === "" ? record : (sitting?.subject.title ?? record);
  // Where the human goes when the room closes or they step out: a review's goal
  // on the board, a shaping sitting's record, whose page is its door.
  const away = reviewing ? backlogPath(reviewed.goal) : documentPath(record);
  // What the room is about, which every question asked from it carries: the
  // record, at the revision the room is reading, and the desk's item.
  const up = room.desk.current >= 0 ? room.desk.items[room.desk.current] : undefined;
  useAbout(reviewing ? `Review of ${named}` : `${roomWord(purpose)} ${named}`, {
    kind: "document",
    subject: record,
    title: reviewing ? `Review of ${reviewed.goal}` : named,
    revision: table.revision,
    tab: up === undefined ? room.face : `${room.face}: ${deskLabel(up)}`,
    returnTo: roomPath(purpose, record),
  });

  // A shaping room opens on its record's sections where its mark kept no desk
  // (g1-s67 D2): a first visit, or a sitting that stood before rooms were kept.
  // Laid once per visit, when the record has been read.
  const laid = useRef("");
  useEffect(() => {
    if (!here || sitting === null || table.source === "" || laid.current === record) {
      return;
    }
    laid.current = record;
    if (room.desk.items.length === 0) {
      const desk = openingDesk(undefined, sitting.purpose, record, table.source);
      if (desk.items.length > 0) {
        openDesk(desk);
      }
    }
  }, [here, sitting, table.source, record, room.desk.items.length, openDesk]);

  // The tip is compared once on arriving (D9): a moved branch is said, and the
  // findings anchored in files that changed are marked until the new tip is
  // reviewed. It is read once, on return, and never polled.
  const compared = useRef("");
  useEffect(() => {
    if (!here || reviewed.tip === "" || compared.current === `${record}@${reviewed.tip}`) {
      return;
    }
    compared.current = `${record}@${reviewed.tip}`;
    setMoved(null);
    const aborter = new AbortController();
    loadChanges(record, false, aborter.signal)
      .then(async (changes) => {
        if (!changes.moved) {
          return;
        }
        const since = await loadChanges(record, true, aborter.signal);
        setMoved({ current: changes.current, changed: since.files.map((file) => file.path) });
      })
      .catch(() => {
        // A branch that cannot be read is not a moved one: the desk says so
        // when a human puts the change on it.
      });
    return () => {
      aborter.abort();
    };
  }, [here, record, reviewed.tip]);

  // A walk puts on the desk what it is explaining, as it goes (D5), unless the
  // human stopped its presenting for this turn.
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
      const item = deskItemOf(present);
      if (item !== null) {
        putOnDesk(item);
      }
    }
  }, [store.live, partner.stoppedPresenting, putOnDesk]);

  // The sitting ended — the Outcome was recorded, or the human left without
  // one — so the room closes: a review's board, a shaping sitting's record.
  const stood = useRef(false);
  useEffect(() => {
    if (sitting !== null) {
      stood.current = stood.current || here;
      return;
    }
    // A verdict that acted stays to say what it did (g1-s69 §3); the human
    // leaves when they have read it.
    if (stood.current && here && verdictSaid === "") {
      void navigate(away);
    }
  }, [sitting, here, navigate, away, verdictSaid]);

  const stepOut = async () => {
    setStayed("");
    const step = await steppingOut(busy, leaving, stop, () => keepRoomNow());
    if (step.kind === "warn") {
      setLeaving(true);
      return;
    }
    if (step.kind === "stay") {
      setStayed(step.said);
      return;
    }
    void navigate(away);
  };

  const mine = Object.keys(room.drafts).filter((id) => id.startsWith("local-"));
  const shownLocal = partner.deposits.filter((card) => card.id.startsWith("local-") &&
    (mine.includes(card.id) || card.standing === "recorded"));

  return (
    <div className="ms-room" data-face={room.face}>
      <header className="ms-room-head">
        <h1 className="ms-room-title">
          {roomWord(purpose)} <span className={reviewing ? "ms-mono" : undefined}>{named}</span>
          <Help id={reviewing ? "review-room" : "sitting-room"} />
        </h1>
        <p className="ms-room-facts">
          {reviewing && reviewed.tip !== "" && <span>at tip {reviewed.tip.slice(0, 9)}</span>}
          {reviewing && reviewed.landed.length > 0 && (
            <span>
              {reviewed.landed.length} landed {reviewed.landed.length === 1 ? "commit" : "commits"}
            </span>
          )}
          <span>
            {reviewing
              ? countsLine(findings.length, unanswered.length)
              : pilesLine({
                  facts: table.counts.Facts, proposals: table.counts.Proposals, decisions: table.counts.Decisions,
                  questions: table.counts["Open questions"],
                })}
          </span>
        </p>
        {reviewing && reviewed.goal !== "" && (
          <div className="ms-room-candidate">
            <CandidatePill goal={reviewed.goal} reviewed={reviewed.tip} />
          </div>
        )}
        <div className="ms-room-actions">
          <Button
            onClick={() => {
              setFace(room.face === "desk" ? "board" : "desk");
            }}
          >
            {room.face === "desk" ? "Board" : "Desk"}
          </Button>
          <Button
            onClick={() => {
              setEnding(true);
            }}
            disabled={sitting === null}
          >
            End
          </Button>
          <Button primary onClick={() => void stepOut()}>
            Step out
          </Button>
          <NotificationsBell />
        </div>
      </header>
      {leaving && busy && (
        <p className="ms-room-banner" role="status">
          Your Partner is answering. Stepping out stops the answer, and it is kept as stopped; everything else waits
          here for you.{" "}
          <Button onClick={() => void stepOut()}>Step out anyway</Button>
          <Button
            onClick={() => {
              setLeaving(false);
            }}
          >
            Stay
          </Button>
        </p>
      )}
      {stayed !== "" && (
        <p className="ms-room-banner" role="status">
          {stayed}
        </p>
      )}
      {verdictSaid !== "" && (
        <p className="ms-room-banner ms-room-banner--verdict" role="status">
          {verdictSaid}{" "}
          <Button
            primary
            onClick={() => {
              void navigate(away);
            }}
          >
            Back to the board
          </Button>
        </p>
      )}
      {stranded !== null && verdictRefusal === "" && (
        <PendingVerdict pending={stranded} busy={verdictBusy} onPress={recordStranded} />
      )}
      {verdictRefusal !== "" && (
        <p className="ms-room-banner" role="status">
          The Outcome is recorded, and the verdict is not yet on the goal: {verdictRefusal}{" "}
          <Button onClick={retryVerdict}>Record the verdict again</Button>
        </p>
      )}
      {retipSaid !== "" && (
        <p className="ms-room-banner" role="status">
          {retipSaid}
        </p>
      )}
      {reviewing && moved !== null && reviewed.tip !== "" && (
        <p className="ms-room-banner ms-room-banner--moved" role="status">
          {movedLine(reviewed.tip, moved.current)} Findings anchored in files that changed are marked "may have
          moved".{" "}
          <Button
            onClick={() => {
              putOnDesk({ kind: "changes", since: true });
            }}
          >
            Show what changed
          </Button>
          <Button
            onClick={() => {
              const drafted = standingOutcome(partner.deposits);
              void partner.reviewNewTip(moved.current).then((refusal) => {
                setRetipRefusal(refusal);
                if (refusal === "") {
                  setMoved(null);
                  // An Outcome drafted for the old tip is not recorded against
                  // the new one: the room asks for End again (g1-s69 D1).
                  setRetipSaid(drafted ? RETIP_ASKS_END : "");
                }
              });
            }}
          >
            Review the new tip
          </Button>
          {retipRefusal !== "" && <Trouble text={retipRefusal} as="span" variant="small" />}
        </p>
      )}
      {store.state === "ready" && sitting === null && !stood.current && (
        <p className="ms-room-banner" role="status">
          {reviewing
            ? `No review stands on ${record}. What it recorded is in the record; press Review it on the goal to begin another.`
            : `No sitting stands on ${record}. What it recorded is in the record; press ${START} on its page to begin another.`}
        </p>
      )}
      <Group id="ms-room" className="ms-room-panes" orientation="horizontal">
        <Panel
          id="desk"
          className={`ms-room-desk${room.face === "desk" && room.desk.items.length === 0 ? " ms-room-desk--empty" : ""}`}
          defaultSize="60%"
          minSize="30%"
        >
          {room.face === "desk" ? (
            <Desk record={record} />
          ) : (
            <div className="ms-room-board">
              <Board changed={moved?.changed ?? []} />
            </div>
          )}
        </Panel>
        <Separator className="ms-room-grip" aria-label="Resize the desk and the conversation" />
        <Panel id="conversation" className="ms-room-conversation" defaultSize="40%" minSize="25%">
          <MovedFiles.Provider value={moved?.changed ?? []}>
          <DeskAnchors.Provider value={putOnDesk}>
            <div className="ms-room-walks" role="group" aria-label="The walks">
              {walksOf(purpose).map((one) => (
                <Button
                  key={one.part}
                  disabled={busy || sitting === null}
                  onClick={() => void partner.walk(one.part)}
                >
                  {one.label}
                </Button>
              ))}
              <Help id={reviewing ? "the-walks" : "shaping-walks"} />
              {busy && store.live.turn !== partner.stoppedPresenting && (
                <Button onClick={partner.stopPresenting}>Stop presenting</Button>
              )}
            </div>
            <div className="ms-room-talk">
              <div className="ms-room-transcript ms-dock-statement">
                <Transcript />
                {shownLocal.length > 0 && (
                  <section className="ms-room-mine" aria-label={reviewing ? "Findings you made" : "Facts you made"}>
                    {shownLocal.map((card) => (
                      <DepositCard key={card.id} id={card.id} />
                    ))}
                  </section>
                )}
              </div>
            </div>
            <Composer />
          </DeskAnchors.Provider>
          </MovedFiles.Provider>
        </Panel>
      </Group>
      {endOf(purpose) === "verdict" ? (
        <EndSheet open={ending} onOpenChange={setEnding} />
      ) : (
        <ShapingEndSheet open={ending} onOpenChange={setEnding} />
      )}
    </div>
  );
}

/** A display suggestion as the desk item it puts up, or null for one the desk has no view of. */
export function deskItemOf(present: Present): DeskItem | null {
  switch (present.kind) {
    case "source":
      return present.path === undefined
        ? null
        : { kind: "source", path: present.path, from: present.from ?? 0, to: present.to ?? present.from ?? 0 };
    case "changes":
      return { kind: "changes" };
    case "diff":
      return present.path === undefined ? null : { kind: "diff", path: present.path };
    case "section":
      return present.path === undefined || present.section === undefined
        ? null
        : { kind: "section", record: present.path, section: present.section };
    default:
      return null;
  }
}

/**
 * End with a verdict line (D10). Three ways, each with its consequence: Clear to
 * land refuses while the record carries a finding whose answer is unanswered,
 * and lists them; each way asks the Partner to draft the Outcome with the
 * verdict as its first line, which the human reads, edits and records. The nod
 * line shows when the piles are empty. In this build the verdict changes nothing.
 */
function EndSheet({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const { table, closeSitting, sittingBusy, sittingRefusal, busy, conversation, brief, setBrief } = usePartner();
  const [chosen, setChosen] = useState<Verdict | "">("");
  const [sendingBack, setSendingBack] = useState(false);
  // The sheet opens on its three ways each time: a Send back step left open
  // when the sheet was closed is not where the next End begins.
  const opened = (next: boolean) => {
    if (!next) {
      setSendingBack(false);
    }
    onOpenChange(next);
  };
  const choose = async (verdict: Verdict) => {
    setChosen(verdict);
    try {
      await closeSitting(verdict);
      opened(false);
    } catch {
      // The refusal is the store's, and it is shown here.
    }
  };
  return (
    <Sheet
      open={open}
      onOpenChange={opened}
      side="right"
      label="End this review"
      title="End this review"
      closeLabel="Close without ending"
      bodyClassName="ms-sitting-sheet ms-review-sheet"
      sheetName="End this review"
    >
      <p className="ms-sitting-said">
        {"Your Partner drafts the Outcome with your verdict as its first line and what you examined after it; " +
          "you read it, edit it, and press Record it. It becomes the review\u2019s Outcome, and the review ends then."}
        <Help id="the-verdict" />
      </p>
      {sendingBack ? (
        <SendBackBrief
          entries={table.entries}
          brief={brief ?? correctionBrief(table.entries, conversation, reviewedOf(table.source).tip)}
          busy={sittingBusy || busy}
          onEdit={setBrief}
          onSend={(text) => {
            setBrief(text);
            void choose("send back");
          }}
          onBack={() => {
            setSendingBack(false);
          }}
        />
      ) : (
        <EndWays
          entries={table.entries}
          busy={sittingBusy}
          onChoose={(verdict) => {
            if (verdict === "send back") {
              setSendingBack(true);
              return;
            }
            void choose(verdict);
          }}
        />
      )}
      {chosen !== "" && sittingBusy && <p className="ms-desk-loading">Drafting the Outcome…</p>}
      {sittingRefusal !== "" && (
        <Trouble text={sittingRefusal} role="status" variant="small" />
      )}
    </Sheet>
  );
}

/**
 * Send back's own step (g1-s69 D2): the findings answered fix, the correction
 * brief composed from them for the human to read and edit, and Send back, which
 * drafts the Outcome with the verdict; the brief travels with the verdict once
 * the Outcome is recorded. It is refused in words where no finding is answered
 * fix, and it waits while the room's closing turn is unsettled.
 */
export function SendBackBrief({
  entries,
  brief,
  busy,
  onEdit,
  onSend,
  onBack,
}: {
  entries: readonly Entry[];
  brief: string;
  busy: boolean;
  onEdit: (text: string) => void;
  onSend: (text: string) => void;
  onBack: () => void;
}) {
  const fixes = fixFindings(entries);
  if (fixes.length === 0) {
    return (
      <div className="ms-send-back">
        <Trouble text={NO_FIX} role="status" act={{ verb: "Send back", object: "review" }} />
        <Button onClick={onBack}>Back to the three ways</Button>
      </div>
    );
  }
  return (
    <div className="ms-send-back">
      <p className="ms-sitting-said">
        {BRIEF_SAID}
        <Help id="the-brief" />
      </p>
      <ul className="ms-send-back-fixes" aria-label="The findings answered fix">
        {fixes.map((entry) => (
          <li key={entry.mark === "" ? entry.text : entry.mark}>
            {entry.text}
            {entry.clause.trim() !== "" && <span className="ms-mono"> {entry.clause}</span>}
          </li>
        ))}
      </ul>
      <label className="ms-send-back-label">
        The correction brief
        <textarea
          className="ms-send-back-brief ms-mono"
          value={brief}
          rows={10}
          onChange={(event) => {
            onEdit(event.target.value);
          }}
        />
      </label>
      <div className="ms-sitting-foot">
        <Button
          primary
          disabled={busy || brief.trim() === ""}
          onClick={() => {
            onSend(brief);
          }}
        >
          Send back
        </Button>
        <Button disabled={busy} onClick={onBack}>
          Back to the three ways
        </Button>
      </div>
    </div>
  );
}

/**
 * The End sheet's three ways, the refusal Clear to land carries while a finding
 * is unanswered, and the nod line where the piles are empty (D10).
 */
export function EndWays({
  entries,
  busy,
  onChoose,
}: {
  entries: readonly Entry[];
  busy: boolean;
  onChoose: (verdict: Verdict) => void;
}) {
  const unanswered = unansweredIn(entries);
  return (
    <>
      {nodded(entries) && (
        <p className="ms-end-nod" role="status">
          {NOD_LINE}
        </p>
      )}
      <ul className="ms-end-ways">
        {VERDICTS.map((one) => {
          const refused = one.verdict === "clear to land" && unanswered.length > 0;
          return (
            <li key={one.verdict} className="ms-end-way">
              <Button
                primary={one.verdict === "clear to land"}
                disabled={refused || busy}
                onClick={() => {
                  onChoose(one.verdict);
                }}
              >
                {one.label}
              </Button>
              <span className="ms-finding-consequence">{one.consequence}</span>
              {refused && (
                <div className="ms-end-unanswered">
                  <Trouble
                    text={`${CLEAR_REFUSED} ${unanswered.map((entry) => entry.text).join("; ")}`}
                    role="status"
                    act={{ verb: "Clear to land", object: "review" }}
                  >
                    {CLEAR_REFUSED}
                  </Trouble>
                  <ul>
                    {unanswered.map((entry) => (
                      <li key={entry.mark === "" ? entry.text : entry.mark}>{entry.text}</li>
                    ))}
                  </ul>
                </div>
              )}
            </li>
          );
        })}
      </ul>
    </>
  );
}

/**
 * End in a shaping room (g1-s55 D2, g1-s67 D7): the sheet the drawer's control
 * used to open, moved into the room with its two ways out and no verdict.
 *
 * It is a sheet and not a button, because ending a sitting is two acts that read
 * as one. The first asks the Partner to draft what the sitting came to; the
 * sitting is still standing when it answers, the card arrives in the
 * conversation beside the desk, and it ends when the human presses Record it on
 * the card. The second ends it now, with nothing written — allowed, and the
 * easiest mistake this design allows, so it is said before it is done and said
 * again on the record's page afterwards.
 */
function ShapingEndSheet({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const { closeSitting, endWithoutRecording, sittingBusy, sittingRefusal, sitting } = usePartner();
  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      side="right"
      label={END}
      title={END}
      closeLabel="Close without ending the sitting"
      bodyClassName="ms-sitting-sheet"
      sheetName={END}
    >
      <p className="ms-sitting-said">
        {END_SAID}
        <Help id="the-outcome" />
      </p>
      {sittingRefusal !== "" && (
        <Trouble text={sittingRefusal} role="status" />
      )}
      <EndShapingWays
        busy={sittingBusy}
        design={sitting?.purpose === "shape a design" ? sitting.subject.id : ""}
        onDraft={() => {
          void closeSitting().then(
            () => {
              onOpenChange(false);
            },
            () => {
              // The refusal is on the sheet, in the server's own words.
            },
          );
        }}
        onWithout={() => {
          void endWithoutRecording().then(
            () => {
              onOpenChange(false);
            },
            () => {
              // Said on the sheet, which stays open.
            },
          );
        }}
      />
    </Sheet>
  );
}

/**
 * A shaping End sheet's two ways out, and no verdict (g1-s67 D7) — and, in a
 * sitting on a design, the way on to the goal that builds it (g1-s66 D5): the
 * design's page, where the New goal sheet opens prefilled from the Outcome once
 * it is recorded.
 */
export function EndShapingWays({
  busy,
  onDraft,
  onWithout,
  design = "",
}: {
  busy: boolean;
  onDraft: () => void;
  onWithout: () => void;
  /** The design the sitting is on, or "" in a sitting on an intent. */
  design?: string;
}) {
  return (
    <div className="ms-sitting-foot">
      <Button primary disabled={busy} onClick={onDraft}>
        {busy ? DRAFTING : END}
      </Button>
      <Button disabled={busy} onClick={onWithout}>
        {END_WITHOUT}
      </Button>
      {design !== "" && (
        <p className="ms-sitting-said">
          <NavLink to={openGoalPath(documentPath(design))}>{OPEN_FROM}</NavLink>: on the design&apos;s page, with the intent and
          the next step taken from the Outcome you record.
        </p>
      )}
    </div>
  );
}
