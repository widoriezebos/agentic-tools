import { useCallback, useEffect, useState } from "react";

import { onFleetEvent, onStreamReopen } from "../notifications/stream";
import { Trouble } from "../shell/Trouble";
import { type BoardPayload, failureMessage, loadBoard } from "./api";
import { LaneBlock } from "./LandingLane";

/**
 * This host's board (batch-lane design D14-r2, U10d): one line per armed
 * seat of this host, saying what it works on and how far it is, under the
 * host's landing lane (U12), which the same response carries.
 *
 * It reads when the Fleet page shows it, and again on the same `fleet` event
 * and reconnect the fleet re-reads on; it holds no timer. A read that fails
 * keeps nothing stale: it says the board could not be read.
 */
export function HostBoard() {
  const [board, setBoard] = useState<BoardPayload | null>(null);
  const [problem, setProblem] = useState("");
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const aborter = new AbortController();
    loadBoard(aborter.signal)
      .then((answered) => {
        setBoard(answered);
        setProblem("");
      })
      .catch((error: unknown) => {
        if (!aborter.signal.aborted) {
          setProblem(failureMessage(error));
        }
      });
    return () => {
      aborter.abort();
    };
  }, [attempt]);

  const again = useCallback(() => {
    setAttempt((previous) => previous + 1);
  }, []);

  useEffect(() => {
    const stopFleet = onFleetEvent(again);
    const stopOpen = onStreamReopen(again);
    return () => {
      stopFleet();
      stopOpen();
    };
  }, [again]);

  if (problem !== "") {
    return (
      <section className="ms-fleet-block">
        <h2 className="ms-fleet-heading">This host</h2>
        <Trouble text={`The board could not be read: ${problem}`} role="status" />
      </section>
    );
  }
  return board === null ? null : <HostBlocks board={board} onLanded={again} />;
}

/**
 * What one board read draws: the host's landing lane above its seats. Both
 * come from the one response, so the lane adds no request of its own.
 */
export function HostBlocks({ board, onLanded }: { board: BoardPayload; onLanded?: () => void }) {
  return (
    <>
      <LaneBlock lane={board.lane} onLanded={onLanded} />
      <BoardBlock board={board} />
    </>
  );
}

/** The block itself, from the server's payload. */
export function BoardBlock({ board }: { board: BoardPayload }) {
  return (
    <section className="ms-fleet-block">
      <h2 className="ms-fleet-heading">This host</h2>
      {!board.readable && <p className="ms-fleet-quiet">The board could not be read: {board.reason ?? ""}</p>}
      {board.readable && board.lines.length === 0 && <p className="ms-fleet-quiet">No seat is armed on this host.</p>}
      {board.lines.length > 0 && (
        <ul className="ms-fleet-facts">
          {board.lines.map((line) => (
            <li key={line.machine} className="ms-fleet-fact ms-fleet-board-row">
              <span className="ms-mono">{line.machine}</span>: {line.text}
            </li>
          ))}
        </ul>
      )}
      <p className="ms-fleet-provenance">bridge {board.bridge}</p>
    </section>
  );
}
