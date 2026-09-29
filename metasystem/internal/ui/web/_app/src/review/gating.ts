import type { Gate } from "../backlog/api";

/**
 * The landing gate's words on a card and in the inbox (g1-s70 D5, §3).
 *
 * Everything here is computed from the reading the server handed the board and
 * the instant the page was rendered at. Nothing ticks: the countdown is the
 * difference between two instants the render already has, and the next read of
 * the board is what moves it. The promise is eligibility, never a moment: a
 * goal below the tier becomes eligible to land by itself, and the seat that
 * holds it lands it on its next turn.
 */

/** The line a goal at or above the tier carries while it waits for this human. */
export const WAITS_FOR_YOUR_REVIEW = "waits for your review";

/** The line once the grace time has passed below the tier. */
export const ELIGIBLE_WAITING = "eligible to land, waiting for the holder";

/** What the Decide sheet's press is called, everywhere it is offered. */
export const LAND_WITHOUT_SITTING = "Land without a sitting";

/** What the Decide sheet says over its one field. */
export const LAND_WITHOUT_SITTING_SAID =
  "Say why this goal needs no sitting. Your reason is recorded on the goal against the tip its branch has now, and the seat that holds it lands it on its next turn.";

/** What the sheet says while the reason is missing. */
export const REASON_NEEDED = "Land without a sitting carries your reason.";

/** A duration as the card says it: at most two units, "3h 12m", "2d 4h", "45m". */
export function durationWords(ms: number): string {
  const minutes = Math.max(1, Math.ceil(ms / 60_000));
  const days = Math.floor(minutes / 1440);
  const hours = Math.floor((minutes % 1440) / 60);
  const rest = minutes % 60;
  if (days > 0) {
    return hours > 0 ? `${String(days)}d ${String(hours)}h` : `${String(days)}d`;
  }
  if (hours > 0) {
    return rest > 0 ? `${String(hours)}h ${String(rest)}m` : `${String(hours)}h`;
  }
  return `${String(rest)}m`;
}

/** Whose a hold is, as this human reads it. */
function whose(by: string, you: string): string {
  return by === you ? "your" : `${by}'s`;
}

/**
 * The card's one line about the gate, or "" where there is nothing to say:
 * "held by your sitting", "eligible to land in 3h 12m", "eligible to land,
 * waiting for the holder", "waits for your review", the human's word
 * waiting for the holder, or that word asked for again once the branch moved.
 */
export function gateLine(gate: Gate | undefined, you: string, now: Date): string {
  if (gate === undefined) {
    return "";
  }
  if (gate.landed) {
    return "landed; the goal stays open until it is done";
  }
  const held = gate.heldBy ?? [];
  if (held.length > 0) {
    return `held by ${whose(held[0].by, you)} sitting`;
  }
  if (!gate.waitsForHuman) {
    if (gate.eligible || gate.autoLandsAt === undefined) {
      return ELIGIBLE_WAITING;
    }
    const left = Date.parse(gate.autoLandsAt) - now.getTime();
    return left <= 0 ? ELIGIBLE_WAITING : `eligible to land in ${durationWords(left)}`;
  }
  const word = gate.reviewed;
  if (word !== undefined && word.moved === true && word.branchTip !== undefined) {
    // A moved tip needs the word again (G5): the engine refuses the landing.
    const given = word.kind === "clear-to-land" ? "cleared" : "decided to land without a sitting";
    return `${given} by ${word.by} at ${word.tip.slice(0, 7)}, but the branch moved to ${word.branchTip.slice(0, 7)}: needs the word again`;
  }
  if (word !== undefined && word.kind === "clear-to-land") {
    return `cleared to land by ${word.by} at ${word.tip.slice(0, 7)}, waiting for the holder`;
  }
  if (word !== undefined && word.kind === "land-without-sitting") {
    return `to land without a sitting, by ${word.by} at ${word.tip.slice(0, 7)}, waiting for the holder`;
  }
  return WAITS_FOR_YOUR_REVIEW;
}

/**
 * Whether the card offers Land without a sitting: the goal waits for this
 * human's word, because none stands at the branch's tip.
 */
export function offersLandWithoutSitting(gate: Gate | undefined): boolean {
  if (gate === undefined || !gate.waitsForHuman || gate.landed || (gate.heldBy ?? []).length > 0) {
    return false;
  }
  const word = gate.reviewed;
  return word === undefined || word.kind === "send-back" || word.moved === true;
}
