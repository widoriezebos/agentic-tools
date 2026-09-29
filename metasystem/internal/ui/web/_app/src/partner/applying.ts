import { ProposalConflict, recordProposal, type ProposalState } from "./api";
import { answeredOf, dispatchOf, type Answered, type Line, type Looked, type Written } from "./proposing";
import {
  abandonGoal,
  approveGoal,
  blockGoal,
  editGoal,
  loadBacklog,
  openGoal,
  parkGoal,
  rankGoal,
  landWithoutSitting,
  reviewGoal,
  unblockGoal,
  unparkGoal,
  withdrawGoal,
  BacklogError,
  type Backlog,
} from "../backlog/api";
import { startCandidate } from "../review/candidate";
import { answerWords, CritiqueError, sendToCritique } from "../project/critiquing";

/**
 * The impure half of applying what the Partner proposed: the read before a run,
 * the act itself, and the write that records what it answered.
 *
 * The run's own rules are in proposing.ts and are testable without a browser.
 * These three are what that run needs of the world, and they are HERE rather
 * than in either caller because there are two callers: the card in the
 * transcript and the row in the Decisions inbox (g1-s60 D4). Two copies of the
 * dispatch would be two lists of acts, and the day one more is added one
 * of them would be right.
 *
 * Nothing here reaches the network itself. The act goes through the clients
 * every button on every page already uses, and the outcome goes through the
 * conversation's own route, so this file adds no way out of the build: it names
 * the same two call sites the cut guard already counts.
 */

/**
 * The canonical branch, read once before the first line of a run.
 *
 * Fetch-first, because the compare that follows has to be against the branch as
 * of the press rather than against whatever this clone had accepted earlier. The
 * payload says what its own fetch did, and a 200 from the accepted ledger is not
 * the same thing as a fetch that landed: on anything but `advanced` or `current`
 * every line that depends on what a goal says is refused unsent, with the
 * fetch's own words (Astra S58-07).
 *
 * A read that fails at all answers as a fetch that failed, which is the same
 * refusal for the same reason: nothing here may compare against a branch it
 * could not read.
 */
export async function lookOnce(): Promise<Looked> {
  try {
    const read = await loadBacklog(undefined, true);
    return {
      rows: [...read.rows, ...read.closed],
      defaults: read.budgetDefaults,
      outcome: read.ledger.fetch.outcome,
      message: read.ledger.fetch.message === "" ? read.ledger.fetch.detail : read.ledger.fetch.message,
    };
  } catch (error: unknown) {
    return { rows: [], defaults: {}, outcome: "failed", message: reasonOf(error) };
  }
}

/**
 * One line's act, through the clients every button on every page uses.
 *
 * A line whose verb this build cannot dispatch is a refusal on the line rather
 * than a thrown error: it would mean the catalogue and the act table had come to
 * disagree, which is something to read on the card and not a broken page.
 */
export async function sendProposal(line: Line): Promise<Answered> {
  try {
    const after = await actOf(line);
    return after === null
      ? { kind: "refused", words: `this build cannot dispatch ${line.verb}` }
      : { kind: "applied", words: "" };
  } catch (error: unknown) {
    return answeredOf(error);
  }
}

/** Which client one line's act goes through, and with what. */
async function actOf(line: Line): Promise<Backlog | null> {
  const dispatch = dispatchOf(line);
  if (dispatch === null) {
    return null;
  }
  switch (dispatch.act) {
    case "approve":
      return approveGoal(dispatch.id, dispatch.budget);
    case "withdraw":
      return withdrawGoal(dispatch.id, dispatch.reason);
    case "priority":
      return rankGoal(dispatch.id, dispatch.priority, dispatch.sequence);
    case "block":
      return blockGoal(dispatch.dependent, dispatch.blocker);
    case "unblock":
      return unblockGoal(dispatch.dependent, dispatch.blocker);
    case "park":
      return parkGoal(dispatch.id, dispatch.because);
    case "unpark":
      return unparkGoal(dispatch.id);
    case "edit":
      return editGoal(dispatch.id, dispatch.edit);
    case "open":
      return openGoal(dispatch.goal);
    case "abandon":
      return abandonGoal(dispatch.id, dispatch.because, dispatch.successor);
    case "land-without-sitting":
      return landWithoutSitting(dispatch.id, dispatch.reason);
    case "review":
      return (await reviewGoal(dispatch.id, { record: dispatch.record, verdict: dispatch.verdict, brief: "", work: dispatch.work })).backlog;
    // Run answers where the candidate runs, not a backlog: the ledger did not
    // move, so the board as it stands is what the page reads next.
    case "run":
      await startCandidate(dispatch.goal);
      return loadBacklog();
    // A design's critique answers the engine's own words, not a backlog. The
    // engine's refusal is a refusal; a failure may have started something, so
    // it is unresolved; everything else, a round reading included, is applied.
    case "critique":
      await critique(dispatch.design, dispatch.goal, dispatch.toolCalls, dispatch.after);
      return loadBacklog();
    default:
      return null;
  }
}

async function critique(design: string, goal: string, toolCalls: number, after: number): Promise<void> {
  const resource = `design review ${design}`;
  try {
    const answered = await sendToCritique(design, { goal, toolCalls, after });
    if (answered.outcome === "refused") {
      throw new BacklogError(resource, 409, answerWords(answered).join("; "), "engine");
    }
    if (answered.outcome === "failed") {
      throw new BacklogError(resource, 500, answerWords(answered).join("; "), "engine");
    }
  } catch (error: unknown) {
    if (error instanceof CritiqueError) {
      throw new BacklogError(resource, error.status, error.message, error.code, error.signIn);
    }
    throw error;
  }
}

/**
 * One state written onto one line, through the one route that writes them.
 *
 * The version is the entry's own as the caller last rendered it. A write the
 * server refused with the entry comes back as a conflict carrying that entry,
 * which the caller must SHOW rather than answer with an act of its own: the
 * writer that won may have sent the act already. Anything else that went wrong
 * is a failure with its words, and a failure before the act stops the run.
 *
 * The attempt is the run's own token, carried on every write that run makes, and
 * "" on the one write no press owns: a dismissal, which publishes nothing and is
 * compared on the version alone.
 */
export async function writeOutcome(
  turn: string,
  line: Line,
  state: ProposalState,
  words: string,
  attempt = "",
  conversation = "",
): Promise<Written> {
  try {
    const answered = await recordProposal(turn, line.index, line.version, state, words, attempt, conversation);
    return { kind: "written", proposal: answered.proposal };
  } catch (error: unknown) {
    return error instanceof ProposalConflict
      ? { kind: "conflict", proposal: error.proposal }
      : { kind: "failed", words: reasonOf(error) };
  }
}

function reasonOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
