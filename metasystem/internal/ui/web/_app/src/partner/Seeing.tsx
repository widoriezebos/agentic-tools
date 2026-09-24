import { useEffect, useState } from "react";

import { seeing, type Seeing as Composed } from "./api";
import { seeingLine } from "./capture";
import { DraftChip, SubjectChip } from "./Chips";
import { usePartner } from "./store";
import { effectiveSubject } from "./subject";
import { Sheet } from "../shell/Sheet";
import { useSubject } from "../shell/about";

/**
 * What the Partner will see, said in one line and opened in full.
 *
 * The line speaks of the NEXT question and says so: "Seeing". An answer's own
 * meta line speaks of what was given and says "Saw". Astra's first finding is
 * the reason the two are different words — clicking Refresh must not read as
 * "the Partner has now been told", and an answer already given must not change
 * its stamp because the board moved afterwards.
 *
 * The sheet shows the block the server composes from this capture, through the
 * same composer the turn's own block goes through. It is a read: opening it
 * sends nothing and the Partner is told nothing by it. The forty-character
 * hash and the ISO instant an answer's meta line shortens are in there, whole.
 */

/** The one line, as the closed drawer's bar shows it. */
export function Seeing() {
  const [open, setOpen] = useState(false);
  return (
    <span className="ms-drawer-about">
      <SeeingButton
        onOpen={() => {
          setOpen(true);
        }}
      />
      {open && (
        <SeeingSheet
          onClose={() => {
            setOpen(false);
          }}
        />
      )}
    </span>
  );
}

/**
 * The composer card's top row: what the next question will carry, the subject
 * it is about with the × that gives it back to the page, and the offer to take
 * the page's reading again where it has moved on.
 *
 * It is the card's own first row rather than a line of its own above the
 * transcript, because everything a human touches belongs to one object.
 */
export function SeeingRow() {
  const { chosen, clearChosen, sheetDraft, clearSheetDraft, moved, refresh } = usePartner();
  const page = useSubject();
  const subject = effectiveSubject(chosen, page);
  const [open, setOpen] = useState(false);
  return (
    <div className="ms-composer-seeing">
      <SeeingButton
        onOpen={() => {
          setOpen(true);
        }}
      />
      {subject !== null && <SubjectChip subject={subject} onClear={chosen === null ? null : clearChosen} />}
      {/* A sheet a human handed over stands here beside the subject: both are
          answers to "what is this question about", and both are theirs to
          take back. */}
      {sheetDraft !== null && <DraftChip draft={sheetDraft} onClear={clearSheetDraft} />}
      {/* The page moved on since the Partner last looked; the control brings
          its view up to date for the next question, and the words say whose
          view that is (Wido, 2026-09-24: not "refresh", not "what I see"). */}
      {moved && (
        <>
          <span className="ms-seeing-moved">page moved on</span>
          <button type="button" className="ms-seeing-refresh" onClick={refresh}>
            Show it the latest
          </button>
        </>
      )}
      {open && (
        <SeeingSheet
          onClose={() => {
            setOpen(false);
          }}
        />
      )}
    </div>
  );
}

/** The words themselves, which open the sheet wherever they stand. */
function SeeingButton({ onOpen }: { onOpen: () => void }) {
  const { capture } = usePartner();
  return (
    <button type="button" className="ms-seeing-chip" aria-haspopup="dialog" onClick={onOpen}>
      <span className="ms-seeing-what">Seeing:</span>
      <b>{seeingLine(capture)}</b>
    </button>
  );
}

/** The block itself, composed by the server from this capture. */
function SeeingSheet({ onClose }: { onClose: () => void }) {
  const { capture } = usePartner();
  const [read, setRead] = useState<Composed | null>(null);
  const [failed, setFailed] = useState("");
  useEffect(() => {
    const aborter = new AbortController();
    seeing(capture)
      .then((composed) => {
        if (!aborter.signal.aborted) {
          setRead(composed);
        }
      })
      .catch((error: unknown) => {
        if (!aborter.signal.aborted) {
          setFailed(error instanceof Error ? error.message : String(error));
        }
      });
    return () => {
      aborter.abort();
    };
  }, [capture]);
  return (
    <Sheet
      open
      onOpenChange={(next) => {
        if (!next) {
          onClose();
        }
      }}
      side="right"
      label="What the Project Partner will see"
      title="What the Project Partner will see"
      closeLabel="Close"
      bodyClassName="ms-sheet-body--seeing"
      // It takes the work area's modality like every other sheet, and names
      // itself in no capture: this sheet IS the capture, and a block that
      // reported the reading of itself would be telling the human about the
      // act of looking rather than about the page.
      sheetName=""
    >
      <p className="ms-seeing-note">
        This is what the next question will carry. Nothing has been sent, and opening this sends nothing.
      </p>
      {read !== null && (
        <>
          <p className="ms-seeing-source">
            Will see: {read.source}
            {read.displayed !== "" && ` — this page had rendered from ${read.displayed}`}
          </p>
          {read.total > 0 && (
            <p className="ms-seeing-source">
              {read.supplied} of {read.total} rows supplied; the rest are one tool call away.
            </p>
          )}
          <pre className="ms-seeing-block">
            <code>{read.block}</code>
          </pre>
        </>
      )}
      {read === null && failed === "" && <p className="ms-seeing-note">Composing…</p>}
      {failed !== "" && <p className="ms-partner-failed">{failed}</p>}
    </Sheet>
  );
}
