/**
 * Times a human reads.
 *
 * The server sends instants, always in UTC; a human reads the wall clock in
 * front of them, so every instant is rendered in the machine's own time zone.
 * An instant nothing recorded arrives as an empty string and is rendered as
 * "unknown", never as an epoch or a blank: "not known" is an answer.
 *
 * Nothing here counts down. The page sets no timer, so a time on screen is a
 * time from the response, and it stays what it was until the next response.
 */

/** What every formatter says when the record carries no instant. */
export const UNKNOWN = "unknown";

function parse(stamp: string): Date | null {
  if (stamp === "") {
    return null;
  }
  const at = new Date(stamp);
  return Number.isNaN(at.getTime()) ? null : at;
}

function pad(value: number): string {
  return String(value).padStart(2, "0");
}

/** The local calendar day and clock, to the minute. */
export function dateAndTime(stamp: string): string {
  const at = parse(stamp);
  if (at === null) {
    return UNKNOWN;
  }
  return `${String(at.getFullYear())}-${pad(at.getMonth() + 1)}-${pad(at.getDate())} ${pad(at.getHours())}:${pad(at.getMinutes())}`;
}

/** The local clock to the second, for times that are seconds apart. */
export function clockTime(stamp: string): string {
  const at = parse(stamp);
  if (at === null) {
    return UNKNOWN;
  }
  return `${pad(at.getHours())}:${pad(at.getMinutes())}:${pad(at.getSeconds())}`;
}

/**
 * How long ago, in the largest unit that still says something useful. An
 * instant in the future reads as no time at all rather than as a negative age.
 */
export function ageBetween(from: string, to: string): string {
  const start = parse(from);
  const end = parse(to);
  if (start === null || end === null) {
    return UNKNOWN;
  }
  const seconds = Math.max(0, Math.floor((end.getTime() - start.getTime()) / 1000));
  if (seconds < 60) {
    return "less than a minute";
  }
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return `${String(minutes)} min`;
  }
  const hours = Math.floor(minutes / 60);
  if (hours < 48) {
    return `${String(hours)} h`;
  }
  return `${String(Math.floor(hours / 24))} d`;
}

/** The short object name a human compares by eye. */
export function shortTip(tip: string): string {
  return tip.slice(0, 7);
}
