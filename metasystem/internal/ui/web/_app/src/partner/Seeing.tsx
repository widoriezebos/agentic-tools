import { useEffect, useState } from "react";

import { seeing, type Seeing as Composed } from "./api";
import { seeingLine } from "./capture";
import { AttachmentChip, DraftQuestions } from "./Chips";
import { usePartner } from "./store";
import { Sheet } from "../shell/Sheet";

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
 * The composer card's top row: what the next question will carry, everything
 * attached to it with the × that takes each one back, and the offer to take
 * the page's reading again where it has moved on.
 *
 * It is the card's own first row rather than a line of its own above the
 * transcript, because everything a human touches belongs to one object.
 */
export function SeeingRow() {
  const { attachments, detach, moved, refresh, sheetDraft } = usePartner();
  const [open, setOpen] = useState(false);
  return (
    <div className="ms-composer-seeing">
      <SeeingButton
        onOpen={() => {
          setOpen(true);
        }}
      />
      {/* Everything attached, in the order the acts were made, each with its ×
          and the line that says how long it lives. What is here is what the
          capture carries, because both are the same list. Where the human is
          standing is not among them: it has no × and is not a chip, it is the
          Seeing line to the left. */}
      {attachments.map((attachment) => (
        <AttachmentChip
          key={attachment.id}
          attachment={attachment}
          onRemove={() => {
            detach(attachment.id);
          }}
        />
      ))}
      {/* A sheet was handed over, so the two questions that ask the Partner to
          write stand beside its chip. They are the ordinary suggested
          questions: with an empty composer each one sends. */}
      {sheetDraft !== null && <DraftQuestions />}
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
