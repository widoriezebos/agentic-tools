import { useEffect, useRef, useState } from "react";
import { Group, Panel, Separator } from "react-resizable-panels";
import { NavLink, useLocation, useNavigate } from "react-router";

import { DeskAnchors } from "./anchors";
import { Board } from "./Board";
import { Desk } from "./Desk";
import { GuidedReview } from "./Guided";
import {
  deskLabel,
  openingDesk,
  ownSnapshot,
  recordedOutcome,
  roomWord,
  walksOf,
  type DeskItem,
  steppingOut,
} from "./room";
import { DrawingPresses, type Presses } from "../drawing/Drawing";
import { drawingsIn } from "../drawing/drawings";
import { NotificationsBell } from "../notifications/Bell";
import type { Present } from "../partner/api";
import { DRAFTING, END, END_SAID, END_WITHOUT, START } from "../partner/sitting";
import { DepositCard } from "../partner/Deposit";
import { usePartner } from "../partner/store";
import { Transcript } from "../partner/Transcript";
import { Help } from "../help/Help";
import { OPEN_FROM, openGoalPath } from "../project/critiquing";
import { pilesLine } from "../project/pane";
import { documentPath, roomPath, SITTING_PREFIX } from "../routes";
import { useAbout } from "../shell/about";
import { Composer } from "../shell/Composer";
import { Button } from "../shell/controls";
import { Sheet } from "../shell/Sheet";
import "./room.css";
import { Trouble } from "../shell/Trouble";

/**
 * The room (g1-s65 §3), for every sitting (g1-s67 D1). A review is the guided
 * one (review-findings-read-as-decisions §3): one column, four steps, the code
 * and the conversation as sheets. A sitting that shapes an intent or a design
 * keeps one screen of two panes: the desk is the laptop and the conversation
 * stays beside it; the desk flips to the board and back. The rail is not here
 * and neither is the drawer, because the room's own pane is the sitting's
 * conversation (D2, D16).
 */
export function Room({ record }: { record: string }) {
  const { sitting } = usePartner();
  const location = useLocation();
  // A review is the guided one (review-findings-read-as-decisions §3); a
  // shaping sitting keeps its desk and its conversation side by side.
  const purpose = sitting?.purpose ?? (location.pathname.startsWith(SITTING_PREFIX) ? "shape a design" : "review");
  return purpose === "review" ? <GuidedReview record={record} /> : <SittingRoom record={record} />;
}

/**
 * The room of a sitting that shapes an intent or a design (g1-s67): the desk
 * beside the conversation, under one header.
 */
function SittingRoom({ record }: { record: string }) {
  const partner = usePartner();
  const { store, sitting, table, room, setFace, putOnDesk, openDesk, busy, keepRoomNow, stop, conversation } = partner;
  const navigate = useNavigate();
  const location = useLocation();
  // What the sitting is for, which the mark says; before the mark has been read
  // the address says which kind of room this is.
  const purpose = sitting?.purpose ?? (location.pathname.startsWith(SITTING_PREFIX) ? "shape a design" : "review");
  const [ending, setEnding] = useState(false);
  const [leaving, setLeaving] = useState(false);
  const [stayed, setStayed] = useState("");
  // The snapshot read for this room, and not the one the page held before it.
  const here = conversation === record && ownSnapshot(store, record);
  const named = (sitting?.subject.title ?? "") === "" ? record : (sitting?.subject.title ?? record);
  // Where the human goes when the room closes or they step out: the record,
  // whose page is its door.
  const away = documentPath(record);
  // What the room is about, which every question asked from it carries: the
  // record, at the revision the room is reading, and the desk's item.
  const up = room.desk.current >= 0 ? room.desk.items[room.desk.current] : undefined;
  useAbout(`${roomWord(purpose)} ${named}`, {
    kind: "document",
    subject: record,
    title: named,
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
      const item = deskItemOf(present, record);
      if (item !== null) {
        putOnDesk(item);
      }
    }
  }, [store.live, partner.stoppedPresenting, putOnDesk, record]);

  // The sitting ended — the Outcome was recorded, or the human left without
  // one — so the room closes on the record.
  const stood = useRef(false);
  // The design a shaping sitting stood on, kept past its end: once its Outcome
  // is recorded the room stays to offer the goal that builds it (g1-s66 D5).
  const designed = useRef("");
  const handoff = sitting === null && stood.current && designed.current !== "" && recordedOutcome(partner.deposits)
    ? designed.current
    : "";
  useEffect(() => {
    if (sitting !== null) {
      stood.current = stood.current || here;
      designed.current = sitting.purpose === "shape a design" ? sitting.subject.id : "";
      return;
    }
    // A design's recorded Outcome stays to offer its goal; the human leaves
    // when they have read it.
    if (stood.current && here && handoff === "") {
      void navigate(away);
    }
  }, [sitting, here, navigate, away, handoff]);

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
  // What the room does with a drawing (g1-s71 D2, D3): put it on the desk, keep
  // it under the record's Drawings, and say whether the record keeps it.
  const kept = drawingsIn(table.source).map((one) => one.id);
  const presses: Presses = {
    put: putOnDesk,
    keep: partner.keepDrawing,
    kept: (id) => kept.includes(id),
  };
  const shownLocal = partner.deposits.filter((card) => card.id.startsWith("local-") &&
    (mine.includes(card.id) || card.standing === "recorded"));

  return (
    <DrawingPresses.Provider value={presses}>
    <div className="ms-room" data-face={room.face}>
      <header className="ms-room-head">
        <h1 className="ms-room-title">
          {roomWord(purpose)} <span>{named}</span>
          <Help id="sitting-room" />
        </h1>
        <p className="ms-room-facts">
          <span>
            {pilesLine({
              facts: table.counts.Facts, proposals: table.counts.Proposals, decisions: table.counts.Decisions,
              questions: table.counts["Open questions"],
            })}
          </span>
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
      {stayed !== "" && (
        <p className="ms-room-banner" role="status">
          {stayed}
        </p>
      )}
      {handoff !== "" && (
        <OutcomeRecordedWays
          design={handoff}
          onBack={() => {
            void navigate(away);
          }}
        />
      )}
      {store.state === "ready" && sitting === null && !stood.current && (
        <p className="ms-room-banner" role="status">
          {`No sitting stands on ${record}. What it recorded is in the record; press ${START} on its page to begin another.`}
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
              <Board />
            </div>
          )}
        </Panel>
        <Separator className="ms-room-grip" aria-label="Resize the desk and the conversation" />
        <Panel id="conversation" className="ms-room-conversation" defaultSize="40%" minSize="25%">
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
              <Help id="shaping-walks" />
              {busy && store.live.turn !== partner.stoppedPresenting && (
                <Button onClick={partner.stopPresenting}>Stop presenting</Button>
              )}
            </div>
            <div className="ms-room-talk">
              <div className="ms-room-transcript ms-dock-statement">
                <Transcript />
                {shownLocal.length > 0 && (
                  <section className="ms-room-mine" aria-label="Facts you made">
                    {shownLocal.map((card) => (
                      <DepositCard key={card.id} id={card.id} />
                    ))}
                  </section>
                )}
              </div>
            </div>
            <Composer />
          </DeskAnchors.Provider>
        </Panel>
      </Group>
      <ShapingEndSheet open={ending} onOpenChange={setEnding} />
    </div>
    </DrawingPresses.Provider>
  );
}

/** A display suggestion as the desk item it puts up, or null for one the desk has no view of. */
export function deskItemOf(present: Present, record: string): DeskItem | null {
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
    case "evidence":
      // A file of the review's evidence, by its evidence-relative path, read
      // through this room's record (g1-s71 D4).
      return present.path === undefined || present.path === "" ? null : { kind: "evidence", record, path: present.path };
    default:
      return null;
  }
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
 * sitting on a design, where the way on to the goal that builds it comes
 * (g1-s66 D5): not here, because nothing is recorded yet, but in the room once
 * the Outcome is (OutcomeRecordedWays).
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
          Once you record the Outcome, the room offers {OPEN_FROM}, with the intent and the next step taken from it.
        </p>
      )}
    </div>
  );
}

/**
 * A sitting on a design ended with its Outcome recorded (g1-s66 D5): the way on
 * to the goal that builds it, on the design's page, where the New goal sheet
 * opens prefilled from that Outcome, and the way back to the design.
 */
export function OutcomeRecordedWays({ design, onBack }: { design: string; onBack: () => void }) {
  return (
    <p className="ms-room-banner" role="status">
      The Outcome is recorded and the sitting has ended.{" "}
      <NavLink to={openGoalPath(documentPath(design))}>{OPEN_FROM}</NavLink>, with the intent and the next step taken
      from the Outcome.{" "}
      <Button primary onClick={onBack}>
        Back to the design
      </Button>
    </p>
  );
}
