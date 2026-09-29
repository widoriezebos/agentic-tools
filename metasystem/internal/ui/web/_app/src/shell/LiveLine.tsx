import { useEffect, useState } from "react";

import type { Look } from "../partner/api";
import type { Live } from "../partner/conversation";

/**
 * The live line (g1-s74 D1): that an agent is alive, and at what.
 *
 * Four parts, the same wherever a turn is running — the answer's place and the
 * closed drawer's bar. The dot breathes, which is the whole of the motion: a
 * breath says "present", where a spinner would say "blocked, wait", and the
 * human is not blocked. The words are the turn's doing line as the stream sent
 * it, or "Thinking" when nothing is named. The tally is what the answer has
 * read so far, counted as the Looked line counts. The clock is how long the
 * turn has run, from the instant the server admitted it.
 *
 * The words and the tally are one polite status, so a screen reader hears each
 * change of what the Partner is doing and nothing else; the dot and the clock
 * are hidden from it, because a second read aloud every second is noise.
 */

/** What the words say when the turn names nothing it is doing. */
export const THINKING = "Thinking";

/** One second, the clock's only step. */
const SECOND = 1000;

/**
 * The dot alone. It is the one mark both feeds of this slice share (D6): the
 * Partner's turn draws it beside its words, and a working machine draws it on
 * the rail's Fleet row and in the Fleet table.
 */
export function LiveDot({ className }: { className?: string }) {
  return <span className={className === undefined ? "ms-live-dot" : `ms-live-dot ${className}`} aria-hidden="true" />;
}

/**
 * What the answer has read so far, counted as the Looked line counts: the
 * page's own entry is not one of them, a failed read is named and never
 * counted, and a partial read is. Nothing before the first look.
 */
export function tallyOf(looked: readonly Look[]): { read: number; failed: number } | null {
  const chosen = looked.filter((look) => look.page !== true);
  if (chosen.length === 0) {
    return null;
  }
  const read = chosen.filter((look) => look.outcome !== "failed").length;
  return { read, failed: chosen.length - read };
}

/**
 * The time since the server admitted the turn: m:ss, and h:mm:ss past an hour.
 * A browser clock behind the server's reads nothing rather than a negative
 * time, and a turn whose start is not known yet has no clock at all.
 */
export function clockOf(startedAt: string, now: number): string {
  const started = startedAt === "" ? Number.NaN : Date.parse(startedAt);
  if (Number.isNaN(started)) {
    return "";
  }
  const seconds = Math.max(0, Math.floor((now - started) / SECOND));
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const rest = String(seconds % 60).padStart(2, "0");
  return hours > 0 ? `${String(hours)}:${String(minutes).padStart(2, "0")}:${rest}` : `${String(minutes)}:${rest}`;
}

/**
 * The running turn's line. It is mounted only while a turn runs, and its one
 * timer with it: a second's interval that moves the clock and nothing else,
 * set while the server's start is known and cleared when the turn ends or the
 * line unmounts. It reads nothing and reaches no network.
 */
export function LiveLine({ live }: { live: Live }) {
  const [now, setNow] = useState(() => Date.now());
  const startedAt = live.startedAt;
  useEffect(() => {
    if (startedAt === "") {
      return;
    }
    setNow(Date.now());
    const ticking = setInterval(() => {
      setNow(Date.now());
    }, SECOND);
    return () => {
      clearInterval(ticking);
    };
  }, [startedAt]);
  const tally = tallyOf(live.looked);
  const clock = clockOf(startedAt, now);
  return (
    <span className="ms-live">
      <LiveDot />
      <span className="ms-live-words" role="status" aria-live="polite">
        {live.doing === "" ? THINKING : live.doing}
        {tally !== null && ` · ${String(tally.read)} read`}
        {tally !== null && tally.failed > 0 && <span className="ms-live-failed">{`, ${String(tally.failed)} failed`}</span>}
      </span>
      {clock !== "" && <span className="ms-live-clock" aria-hidden="true">{` · ${clock}`}</span>}
    </span>
  );
}
