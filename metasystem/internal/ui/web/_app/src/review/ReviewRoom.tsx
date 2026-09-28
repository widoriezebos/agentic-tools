import { useEffect, useRef, useState } from "react";
import { Group, Panel, Separator } from "react-resizable-panels";
import { useNavigate } from "react-router";

import { loadChanges } from "./api";
import { DeskAnchors } from "./anchors";
import { MovedFiles } from "./Answers";
import { Desk } from "./Desk";
import {
  CLEAR_REFUSED,
  countsLine,
  deskLabel,
  movedLine,
  NOD_LINE,
  nodded,
  unansweredIn,
  VERDICTS,
  WALKS,
  type DeskItem,
  reviewedOf,
  type Verdict,
} from "./room";
import { NotificationsBell } from "../notifications/Bell";
import type { Present } from "../partner/api";
import type { Entry } from "../partner/sitting";
import { DepositCard } from "../partner/Deposit";
import { usePartner } from "../partner/store";
import { SittingTable } from "../partner/Table";
import { Transcript } from "../partner/Transcript";
import { Help } from "../help/Help";
import { backlogPath, reviewPath } from "../routes";
import { useAbout } from "../shell/about";
import { Composer } from "../shell/Composer";
import { Button } from "../shell/controls";
import { Sheet } from "../shell/Sheet";
import "./room.css";

/**
 * The review room (g1-s65 §3): one screen, two panes. The desk is the laptop and
 * the conversation stays beside it; the desk flips to the board and back. The
 * rail is not here and neither is the drawer, because the room's own pane is the
 * sitting's conversation (D2, D16), and nothing else on screen pulls at the
 * human examining one goal's work.
 */
export function Room({ record }: { record: string }) {
  const partner = usePartner();
  const { store, sitting, table, room, setFace, putOnDesk, busy, keepRoomNow, stop, conversation } = partner;
  const navigate = useNavigate();
  const [ending, setEnding] = useState(false);
  const [leaving, setLeaving] = useState(false);
  const [moved, setMoved] = useState<{ current: string; changed: string[] } | null>(null);
  const [retipRefusal, setRetipRefusal] = useState("");
  const reviewed = reviewedOf(table.source);
  const findings = table.entries.filter((entry) => entry.section === "Findings");
  const unanswered = unansweredIn(table.entries);
  const here = conversation === record;
  // What the room is about, which every question asked from it carries: the
  // review record, at the revision the room is reading, and the desk's item.
  const up = room.desk.current >= 0 ? room.desk.items[room.desk.current] : undefined;
  useAbout(`Review of ${reviewed.goal === "" ? record : reviewed.goal}`, {
    kind: "document",
    subject: record,
    title: `Review of ${reviewed.goal}`,
    revision: table.revision,
    tab: up === undefined ? room.face : `${room.face}: ${deskLabel(up)}`,
    returnTo: reviewPath(record),
  });

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

  // The sitting ended — the Outcome was recorded, or the human left without a
  // verdict — so the room closes and the board is where they are.
  const stood = useRef(false);
  useEffect(() => {
    if (sitting !== null) {
      stood.current = true;
      return;
    }
    if (stood.current && here) {
      void navigate(backlogPath(reviewed.goal));
    }
  }, [sitting, here, navigate, reviewed.goal]);

  const stepOut = async () => {
    if (busy && !leaving) {
      setLeaving(true);
      return;
    }
    if (busy) {
      stop();
    }
    await keepRoomNow();
    void navigate(backlogPath(reviewed.goal));
  };

  const mine = Object.keys(room.drafts).filter((id) => id.startsWith("local-"));
  const shownLocal = partner.deposits.filter((card) => card.id.startsWith("local-") &&
    (mine.includes(card.id) || card.standing === "recorded"));

  return (
    <div className="ms-room" data-face={room.face}>
      <header className="ms-room-head">
        <h1 className="ms-room-title">
          Reviewing <span className="ms-mono">{reviewed.goal === "" ? record : reviewed.goal}</span>
          <Help id="review-room" />
        </h1>
        <p className="ms-room-facts">
          {reviewed.tip !== "" && <span>at tip {reviewed.tip.slice(0, 9)}</span>}
          {reviewed.landed.length > 0 && (
            <span>
              {reviewed.landed.length} landed {reviewed.landed.length === 1 ? "commit" : "commits"}
            </span>
          )}
          <span>{countsLine(findings.length, unanswered.length)}</span>
        </p>
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
      {moved !== null && reviewed.tip !== "" && (
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
              void partner.reviewNewTip(moved.current).then((refusal) => {
                setRetipRefusal(refusal);
                if (refusal === "") {
                  setMoved(null);
                }
              });
            }}
          >
            Review the new tip
          </Button>
          {retipRefusal !== "" && <span className="ms-deposit-refusal">{retipRefusal}</span>}
        </p>
      )}
      {store.state === "ready" && sitting === null && !stood.current && (
        <p className="ms-room-banner" role="status">
          No review stands on {record}. What it recorded is in the record; press Review it on the goal to begin
          another.
        </p>
      )}
      <Group id="ms-room" className="ms-room-panes" orientation="horizontal">
        <Panel id="desk" className="ms-room-desk" defaultSize="60%" minSize="30%">
          {room.face === "desk" ? (
            <Desk record={record} />
          ) : (
            <div className="ms-room-board">
              <SittingTable changed={moved?.changed ?? []} />
            </div>
          )}
        </Panel>
        <Separator className="ms-room-grip" aria-label="Resize the desk and the conversation" />
        <Panel id="conversation" className="ms-room-conversation" defaultSize="40%" minSize="25%">
          <MovedFiles.Provider value={moved?.changed ?? []}>
          <DeskAnchors.Provider value={putOnDesk}>
            <div className="ms-room-walks" role="group" aria-label="The walks">
              {WALKS.map((one) => (
                <Button
                  key={one.part}
                  disabled={busy || sitting === null}
                  onClick={() => void partner.walk(one.part)}
                >
                  {one.label}
                </Button>
              ))}
              <Help id="the-walks" />
              {busy && store.live.turn !== partner.stoppedPresenting && (
                <Button onClick={partner.stopPresenting}>Stop presenting</Button>
              )}
            </div>
            <div className="ms-room-transcript ms-dock-statement">
              <Transcript />
              {shownLocal.length > 0 && (
                <section className="ms-room-mine" aria-label="Findings you made">
                  {shownLocal.map((card) => (
                    <DepositCard key={card.id} id={card.id} />
                  ))}
                </section>
              )}
            </div>
            <Composer />
          </DeskAnchors.Provider>
          </MovedFiles.Provider>
        </Panel>
      </Group>
      <EndSheet open={ending} onOpenChange={setEnding} />
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
  const { table, closeSitting, sittingBusy, sittingRefusal } = usePartner();
  const [chosen, setChosen] = useState<Verdict | "">("");
  const choose = async (verdict: Verdict) => {
    setChosen(verdict);
    try {
      await closeSitting(verdict);
      onOpenChange(false);
    } catch {
      // The refusal is the store's, and it is shown here.
    }
  };
  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
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
      <EndWays entries={table.entries} busy={sittingBusy} onChoose={(verdict) => void choose(verdict)} />
      {chosen !== "" && sittingBusy && <p className="ms-desk-loading">Drafting the Outcome…</p>}
      {sittingRefusal !== "" && (
        <p className="ms-deposit-refusal" role="status">
          {sittingRefusal}
        </p>
      )}
    </Sheet>
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
                <div className="ms-end-refused" role="status">
                  <p>{CLEAR_REFUSED}</p>
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
