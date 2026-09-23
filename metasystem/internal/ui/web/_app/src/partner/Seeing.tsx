import { useEffect, useState } from "react";

import { seeing, type Seeing as Composed } from "./api";
import { seeingLine } from "./capture";
import { usePartner } from "./store";
import { Sheet } from "../shell/Sheet";

/**
 * What the Partner will see, said in one line and opened in full.
 *
 * The chip speaks of the NEXT question and says so: "Will see". An answer's
 * own stamp speaks of what was given and says "Saw". Astra's first finding is
 * the reason the two are different words — clicking Refresh must not read as
 * "the Partner has now been told", and an answer already given must not change
 * its stamp because the board moved afterwards.
 *
 * The sheet shows the block the server composes from this capture, through the
 * same composer the turn's own block goes through. It is a read: opening it
 * sends nothing and the Partner is told nothing by it.
 */
export function Seeing() {
  const { capture, moved, refresh } = usePartner();
  const [open, setOpen] = useState(false);
  return (
    <span className="ms-drawer-about">
      <button
        type="button"
        className="ms-seeing-chip"
        aria-haspopup="dialog"
        onClick={() => {
          setOpen(true);
        }}
      >
        <span className="ms-seeing-what">Seeing:</span>
        <b>{seeingLine(capture)}</b>
      </button>
      {moved && (
        <button
          type="button"
          className="ms-seeing-refresh"
          onClick={refresh}
        >
          Refresh what it sees
        </button>
      )}
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
