import { proofLogAddress, type BoardPayload, type BoardSeat, type Lane, type LaneEntry, type Machine, type Page, type Working } from "./api";
import { ageBetween, dateAndTime, minuteTime } from "../backlog/format";
import { flagWords, minutesWords, roundWords } from "./fleet";

/**
 * The panel's rules, kept apart from the elements that draw them: the one
 * verdict, what needs the person and where each item comes from, what each
 * machine is doing, and what the landing lane holds.
 *
 * Nothing here is a new fact. Every rule reads the two payloads this page
 * already reads — the fleet, and this computer's board with its landing lane
 * and its questions — and says what they say in plain words, one word per
 * state. What an instant reads as is the browser's to say, against the one
 * clock the page rendered with.
 */

/** The fleet's reading: none yet, a first read that failed, or a page. */
export type FleetReading =
  | { state: "loading" }
  | { state: "failed"; message: string }
  // `problem` is a later read that failed beside the reading it could not
  // replace: what is on screen is the last reading, and the verdict says so.
  | { state: "read"; page: Page; problem?: string };

/** This computer's board, read the same way. */
export type BoardReading =
  | { state: "loading" }
  | { state: "failed"; message: string }
  | { state: "read"; board: BoardPayload; problem?: string };

/**
 * What the verdict checked, in the words that stay beside its counts: the
 * seats and the lane are this computer's, and the questions only this
 * checkout's (R2-8). The whole sentence is the verdict's help.
 */
export const QUESTIONS_SCOPE = "questions: this checkout only";

/** An instant a person reads: the clock alone today, the day as well before. */
export function when(stamp: string, now: Date): string {
  const at = new Date(stamp);
  if (Number.isNaN(at.getTime())) {
    return minuteTime(stamp);
  }
  return sameDay(at, now) ? minuteTime(stamp) : dateAndTime(stamp);
}

function sameDay(at: Date, now: Date): boolean {
  return at.toDateString() === now.toDateString();
}

/** Whole minutes from an instant to now, floored and never negative. */
function minutesSince(stamp: string, now: Date): number | null {
  const at = Date.parse(stamp);
  if (Number.isNaN(at)) {
    return null;
  }
  return Math.max(0, Math.floor((now.getTime() - at) / 60000));
}

function elapsed(stamp: string | null | undefined, now: Date): string {
  if (stamp === null || stamp === undefined || stamp === "") {
    return "";
  }
  const minutes = minutesSince(stamp, now);
  return minutes === null ? "" : minutesWords(minutes);
}

/** Words joined as a sentence says a list: "a", "a and b", "a, b and c". */
export function joinWords(parts: readonly string[]): string {
  if (parts.length <= 1) {
    return parts.join("");
  }
  return `${parts.slice(0, -1).join(", ")} and ${parts[parts.length - 1]}`;
}

/** A goal as a person reads it: its title, else its id. */
export function titleOf(goal: string, titles: Readonly<Record<string, string>> | undefined): string {
  const title = titles?.[goal] ?? "";
  return title === "" ? goal : title;
}

/* ----------------------------------------------------------- the verdict -- */

/**
 * What each of the panel's four sections could not read, one line each.
 *
 * It is the one rule every reader of a section's state uses: the verdict
 * names a section with any line as one it can't read, Needs you says the
 * questions' lines, and Land now — the one act whose state is what the page
 * read — is not offered over a lane with any line. A read that failed is
 * never read as nothing, and a payload that carries a read problem is a
 * section this page cannot vouch for.
 *
 * - the fleet: its request failed, or a read behind it failed — the presence
 *   copy, the ledger's claims, this seat's health, publication and jobs, a
 *   machine's jobs. A goal's box that could not be projected is a fact about
 *   that goal, said where its work is opened, and the Partner's metadata
 *   file is a write this server made; neither is a failed read of the page;
 * - this computer's board: its request failed, its registry could not be
 *   read, or a part of it was not (the server's unreadable list, which names
 *   a ledger it could not observe);
 * - the landing lane: the board's request failed, the server does not report
 *   a lane, or the lane names any problem;
 * - questions: the board's request failed, or questionsProblem is set.
 */
export type Unread = { fleet: string[]; board: string[]; lane: string[]; questions: string[] };

/** The sections in the order the verdict names them, with the words it uses. */
const SECTIONS: readonly (readonly [keyof Unread, string])[] = [
  ["fleet", "the fleet"],
  ["board", "this computer's board"],
  ["lane", "the landing lane"],
  ["questions", "questions"],
];

function said(lines: readonly (string | undefined)[]): string[] {
  return lines.filter((line): line is string => line !== undefined && line !== "");
}

export function unreadOf(fleet: FleetReading, board: BoardReading): Unread {
  const failed = board.state === "failed" ? [board.message] : said([board.state === "read" ? board.problem : undefined]);
  const read = board.state === "read" ? board.board : null;
  if (read === null) {
    return { fleet: fleetUnread(fleet), board: failed, lane: failed, questions: failed };
  }
  return {
    fleet: fleetUnread(fleet),
    board: [...failed, ...said([read.readable ? "" : (read.reason ?? "its seats could not be listed")]), ...(read.unreadable ?? [])],
    lane: [...failed, ...(read.lane === undefined ? ["this server does not report it"] : (read.lane?.problems ?? []))],
    questions: [...failed, ...(read.questions === undefined ? ["this server does not report them"] : said([read.questionsProblem]))],
  };
}

function fleetUnread(fleet: FleetReading): string[] {
  if (fleet.state === "loading") {
    return [];
  }
  if (fleet.state === "failed") {
    return [fleet.message];
  }
  const page = fleet.page;
  return said([
    fleet.problem,
    page.copy.problem,
    page.claims.unavailable,
    page.this.health?.problem,
    page.this.publicationProblem,
    page.this.runningProblem,
    ...page.machines.map((machine) => machine.workingProblem),
  ]);
}

/** The sections the verdict names as ones it could not read. */
export function unreadSections(fleet: FleetReading, board: BoardReading): string[] {
  const unread = unreadOf(fleet, board);
  return SECTIONS.filter(([section]) => unread[section].length > 0).map(([, name]) => name);
}

/**
 * The one sentence on top, its tone, the counts beside it, and when the fleet
 * was read ("" before it was), whose title says where the reading came from.
 */
export type Verdict = { words: string; tone: "ok" | "attention" | "unread" | "reading"; facts: string[]; updated: string };

function plural(count: number, one: string, many: string): string {
  return `${String(count)} ${count === 1 ? one : many}`;
}

/**
 * One verdict from one rule: All good only when every section read and
 * nothing needs the person; a failed read names the section instead; and
 * what needs the person is counted either way. Until both reads have
 * answered it says it is reading, never All good.
 */
export function verdictOf(fleet: FleetReading, board: BoardReading, needs: readonly unknown[], now: Date): Verdict {
  const facts = factsOf(fleet, board, now);
  const updated = fleet.state === "read" ? `updated ${minuteTime(fleet.page.readAt)}` : "";
  if (fleet.state === "loading" || board.state === "loading") {
    return { words: "Reading…", tone: "reading", facts, updated };
  }
  const unread = unreadSections(fleet, board);
  const needing = needs.length === 0 ? "" : `${plural(needs.length, "thing needs", "things need")} you`;
  if (unread.length > 0) {
    const words = `Can't read ${joinWords(unread)}`;
    return { words: needing === "" ? words : `${words} · ${needing}`, tone: "unread", facts, updated };
  }
  if (needing !== "") {
    return { words: needing, tone: "attention", facts, updated };
  }
  return { words: "All good on this computer", tone: "ok", facts, updated };
}

/** The counts beside the verdict, from whatever has been read. */
function factsOf(fleet: FleetReading, board: BoardReading, now: Date): string[] {
  const facts: string[] = [];
  const read = board.state === "read" ? board.board : null;
  if (fleet.state === "read") {
    const working = fleet.page.machines.filter((machine) => doingOf(machine, seatOf(read, machine.machine), queueOf(read), now).active);
    facts.push(plural(working.length, "seat working", "seats working"));
  }
  const queue = queueOf(read);
  if (read !== null && read.lane !== undefined && read.lane !== null && !laneNotRead(read.lane)) {
    facts.push(`${String(queue.filter((entry) => entry.state === "waiting").length)} waiting to land`);
  }
  return facts;
}

/**
 * Whether nothing of the lane was read: no root, and the reason its
 * registration could not be read. A lane that is simply not registered never
 * reaches the page as a lane.
 */
export function laneNotRead(lane: Lane): boolean {
  return lane.root === null && (lane.problems ?? []).length > 0;
}

/** A machine's seat on this computer's board, where it has one. */
export function seatOf(board: BoardPayload | null, machine: string): BoardSeat | undefined {
  return board?.seats.find((seat) => seat.machine === machine);
}

/** The lane's queue, or nothing where no lane was read. */
export function queueOf(board: BoardPayload | null): LaneEntry[] {
  return board?.lane?.queue ?? [];
}

/* ------------------------------------------------------------ needs you -- */

/**
 * The one act a Needs you item offers as a button: open the goal, answer the
 * question, open the proof's log in a tab of its own, resume the lane, or stop
 * a machine of this computer (the last two run as the signed-in person,
 * fleet-panel-ux step 2, slice 2b).
 */
export type NeedAct =
  | { kind: "goal"; goal: string }
  | { kind: "answer" }
  | { kind: "log"; href: string }
  | { kind: "resume" }
  | { kind: "stop"; machine: string };

/**
 * One thing that needs the person, as one line and one thing to do.
 *
 * The line says the subject the person knows by name (the lane, a goal's
 * title in quotes, a machine), what is wrong and since when; the quiet second
 * line, where there is one, is a reason the record carries, cut at its first
 * sentence. What to do is exactly one of three: the act (a button at the
 * line's end), the command a person types (the public verb, in code), or —
 * for a silent machine holding several goals — one command per goal, under
 * its title. Never two of them. Only an unreadable health record keeps its
 * reader's message behind a Details disclosure. `at` orders the list, newest
 * first.
 */
export type Need = {
  key: string;
  at: string;
  words: string;
  note: string;
  /** What an override does, said whole before the person types it (R-143-m1e); "" when the item overrides nothing. */
  impact: string;
  act: NeedAct | null;
  command: string;
  goals: { id: string; title: string; command: string }[];
  details: string[];
};

function need(over: Partial<Need> & Pick<Need, "key" | "words">): Need {
  return { at: "", note: "", impact: "", act: null, command: "", goals: [], details: [], ...over };
}

/** The public verbs Needs you offers (R-126-m1e), as a person types them. */
const LANDING_START = "metasystem landing start";
const LANDING_STATUS = "metasystem landing status";
const SYSTEM_START = "metasystem system start";
const SYSTEM_CHECK = "metasystem system check";

export function machineStop(machine: string): string {
  return `metasystem machine stop ${machine}`;
}

/** What a stop ends, said whole before anyone presses or types it (R-143-m1e). */
export function stopImpact(machine: string): string {
  return `Stopping ${machine} ends its seat and every job on it; its steward will not start it again.`;
}

/**
 * How this page stops a machine (R-142-ui): the button for a seat of this
 * computer — one this computer's board reads — other than the seat serving
 * this page; the command at a terminal for that seat, since a stop run inside
 * its server would end the server before it could say what it did (the
 * server refuses it too); and nothing for a machine on another computer,
 * which only that computer stops.
 */
export function stopOffer(machine: Machine, board: BoardPayload | null): "button" | "command" | null {
  if (seatOf(board, machine.machine) === undefined) {
    return null;
  }
  return machine.this ? "command" : "button";
}

/**
 * The one lane act beside its state word: Pause on a running lane, Resume on
 * a paused one, never both, and neither while the lane cannot run or any part
 * of it could not be read — Land now's rule, since a press over a lane this
 * page cannot vouch for acts on a state nobody has seen.
 */
export function laneActOf(lane: Lane, unread: readonly string[]): "pause" | "resume" | null {
  if (unread.length > 0) {
    return null;
  }
  if (paused(lane)) {
    return "resume";
  }
  return laneState(lane).word === "Running" ? "pause" : null;
}

/** The take-over of one held goal: the public form, which asks the person's reason. */
function takeOver(goal: string): string {
  return `metasystem goal claim ${goal} --take-over --reason "<why>"`;
}

/** What a take-over does, said before the person types it (R-143-m1e). */
function takeOverImpact(machine: string): string {
  return `Taking a goal over moves it to the machine you run this on: ${machine} stops holding it, and what ${machine} has not pushed stays on ${machine}. It can be taken back the same way.`;
}

/** How long a Needs you line may be at 1440 px; on a phone it wraps. */
const LINE = 110;

/**
 * A line with free words in it — a title, a reason — cut at a word so the
 * whole stays under the line's length. What was cut is behind the item's act
 * or its command; nothing is rewritten, only shortened.
 */
function oneLine(before: string, cut: string, after: string): string {
  const room = LINE - 1 - before.length - after.length;
  if (cut.length <= room) {
    return `${before}${cut}${after}`;
  }
  const kept = cut.slice(0, Math.max(room - 1, 12));
  // Cut where a word ends: at the kept part's own end when the next character
  // starts a new word, else at its last space.
  const word = cut.charAt(kept.length) === " " ? kept.length : kept.lastIndexOf(" ");
  return `${before}${(word > 12 ? kept.slice(0, word) : kept).replace(/[\s,;:.]+$/u, "")}…${after}`;
}

/** A reason as the quiet line carries it: its first sentence, as the record wrote it. */
function firstSentence(reason: string): string {
  const end = /[.!?](?=\s)/u.exec(reason);
  return end === null ? reason.trim() : reason.slice(0, end.index + 1).trim();
}

/** A reason inside a line, which closes the line with its own full stop. */
function clause(reason: string): string {
  return firstSentence(reason).replace(/[.!?]$/u, "");
}

/**
 * Every source in one list, newest first, the undated last: the goals a
 * silent machine holds, a seat of this computer that is stuck, the lane
 * paused or unable to run, a branch that came back and was not handed in
 * again, a red proof nothing has answered yet, this checkout's open
 * questions, and this computer's own health record. Empty, the section is
 * not drawn at all.
 */
export function needsOf(fleet: FleetReading, board: BoardReading, now: Date): Need[] {
  const items: Need[] = [];
  if (fleet.state === "read") {
    items.push(...silentHolds(fleet.page, now), ...healthNeeds(fleet.page, now));
  }
  if (fleet.state === "read" && board.state === "read") {
    items.push(...stuckSeats(fleet.page, board.board, now));
  }
  if (board.state === "read") {
    const read = board.board;
    if (read.lane !== undefined && read.lane !== null && !laneNotRead(read.lane)) {
      items.push(...laneNeeds(read.lane, read.titles, read.ended, unreadOf(fleet, board).lane, now));
    }
    items.push(...questionNeeds(read));
  }
  return newestFirst(items);
}

function newestFirst(items: Need[]): Need[] {
  const stamp = (item: Need) => (item.at === "" ? Number.NaN : Date.parse(item.at));
  return items
    .map((item, index) => ({ item, index, at: stamp(item) }))
    .sort((left, right) => {
      const leftDated = !Number.isNaN(left.at);
      const rightDated = !Number.isNaN(right.at);
      if (leftDated !== rightDated) {
        return leftDated ? -1 : 1;
      }
      if (leftDated && left.at !== right.at) {
        return right.at - left.at;
      }
      return left.index - right.index;
    })
    .map((sorted) => sorted.item);
}

/**
 * The goals held by a machine this seat has not heard from, one item per
 * machine: a machine that went quiet holding four goals is one thing that
 * needs the person, with each goal under it and that goal's own take-over.
 * The take-over is the public verb with the goal's real id, and it asks the
 * person's reason (FR-02).
 */
function silentHolds(page: Page, now: Date): Need[] {
  const machines = new Map<string, Page["needsYou"]>();
  for (const held of page.needsYou) {
    machines.set(held.machine, [...(machines.get(held.machine) ?? []), held]);
  }
  return [...machines].map(([machine, holds]) => {
    const [first] = holds;
    if (holds.length === 1) {
      return need({
        key: `held:${machine}:${first.goal}`,
        at: first.since,
        words: oneLine("“", first.title === "" ? first.goal : first.title, `” is ${flagWithWhen(first, now)}.`),
        impact: takeOverImpact(machine),
        command: takeOver(first.goal),
      });
    }
    return need({
      key: `held:${machine}`,
      at: first.since,
      words: `${String(holds.length)} goals are ${flagWithWhen(first, now)}.`,
      impact: takeOverImpact(machine),
      goals: holds.map((held) => ({ id: held.goal, title: held.title === "" ? held.goal : held.title, command: takeOver(held.goal) })),
    });
  });
}

function flagWithWhen(held: { flag: string; since: string }, now: Date): string {
  if (held.flag === "" || held.since === "") {
    return flagWords(held);
  }
  return `${held.flag} since ${when(held.since, now)}`;
}

/** The board's reason for a card whose stage made no progress past the stall bound. */
const STALLED = "stalled";

/** The start of the board's reason for a card whose writing process is gone. */
const WRITER_DEAD = "writer dead";

/**
 * Whether the board does not believe a card because it stopped moving: its
 * stage made no progress past the stall bound, or the process writing it is
 * gone. Every other reason (unprobeable, reused, no owner, a claim that
 * moved) says nothing about the seat being stuck.
 */
function stuck(unknown: string | undefined): boolean {
  return unknown === STALLED || (unknown ?? "").startsWith(WRITER_DEAD);
}

/** A card's stage, as the Doing column says it. */
function stageWords(stage: string | undefined): string {
  return STAGES[stage ?? ""]?.words ?? stage ?? "";
}

/**
 * A seat of this computer that is stuck: one item per card the board does
 * not believe because it stopped moving, for a goal its machine holds (the
 * rule Doing applies), dated by its last progress. What to do is the stop of
 * that machine, which its steward does not undo: Stop, or for the seat serving
 * this page the command at a terminal (stopOffer). What the stop ends is said
 * whole before anyone presses or types it (R-143-m1e).
 */
function stuckSeats(page: Page, board: BoardPayload, now: Date): Need[] {
  return page.machines.flatMap((machine) => {
    const held = new Map(machine.holds.map((one) => [one.goal, one.title]));
    return (seatOf(board, machine.machine)?.goals ?? [])
      .filter((goal) => held.has(goal.goal) && (goal.stuck !== undefined || stuck(goal.unknown)))
      .map((goal) => {
        const holdTitle = held.get(goal.goal) ?? "";
        const title = holdTitle === "" ? titleOf(goal.goal, board.titles) : holdTitle;
        if (goal.stuck !== undefined) {
          return need({
            key: `stuck:${machine.machine}:${goal.goal}`,
            at: goal.lastProgressAt ?? "",
            words: oneLine("“", title, `” on ${machine.machine}: ${stuckUnitWords(goal.stuck)}.`),
            command: goal.stuck.launch === "" ? "metasystem work land --message" : `metasystem work stop j1:${goal.stuck.launch}`,
          });
        }
        const last = goal.lastProgressAt ?? "";
        const since = last === "" ? "" : ` since ${when(last, now)}`;
        const why = goal.unknown === STALLED ? ` (${stageWords(goal.stage)}).` : " (the process writing it is gone).";
        return need({
          key: `stuck:${machine.machine}:${goal.goal}`,
          at: last,
          words: oneLine("“", title, `” on ${machine.machine} has not moved${since}${why}`),
          impact: stopImpact(machine.machine),
          ...(stopOffer(machine, board) === "button"
            ? { act: { kind: "stop" as const, machine: machine.machine } }
            : { command: machineStop(machine.machine) }),
        });
      });
  });
}

/** The one role whose check has a person's command on this page. */
const STEWARD = "steward-runner";

/**
 * The steward and its record, with one command for each problem a person
 * can act on here. Other roles and the aggregate state belong to the
 * steward's own checks. An unreadable record, a stale record or an undecided
 * steward check keeps the verdict from vouching for this computer. Only the
 * unreadable record keeps its reader's message behind Details.
 */
function healthNeeds(page: Page, now: Date): Need[] {
  const health = page.this.health;
  if (health === null) {
    return [];
  }
  const item = (words: string, command: string, details: string[] = []) => [need({ key: "health", at: health.observedAt, words, command, details })];
  if (health.problem !== "") {
    return item("This computer's health record could not be read.", SYSTEM_CHECK, [health.problem]);
  }
  if (page.this.armed === "stale") {
    return item(`This computer's steward has not recorded its health since ${when(health.observedAt, now)}.`, SYSTEM_START);
  }
  const steward = health.roles.find((role) => role.role === STEWARD);
  if (steward?.status === "dead") {
    return item("This computer's steward is not running.", SYSTEM_START);
  }
  if (steward?.status === "unknown") {
    return item("This computer's steward could not be checked.", SYSTEM_CHECK);
  }
  return [];
}

/** Whether the lane is paused: the person's own stop. */
function paused(lane: Lane): boolean {
  return lane.paused === true || lane.owner.state === "stopped";
}

/**
 * The lane paused or unable to run, the returns and an unanswered red proof.
 * A paused lane offers Resume, unless part of the lane could not be read
 * (laneActOf's rule): then the command that resumes it.
 */
function laneNeeds(
  lane: Lane,
  titles: Readonly<Record<string, string>> | undefined,
  ended: Readonly<Record<string, string>> | undefined,
  unread: readonly string[],
  now: Date,
): Need[] {
  const items: Need[] = [];
  if (paused(lane)) {
    const by = lane.owner.stopped_by ?? "";
    const since = lane.owner.since ?? "";
    const because = lane.owner.stopped_because ?? "";
    const head = `The landing lane is paused${by === "" ? "" : ` by ${by}`}${since === "" ? "" : ` since ${when(since, now)}`}`;
    items.push(
      need({
        key: "lane:paused",
        at: since,
        words: because === "" ? `${head}.` : oneLine(`${head}: `, clause(because), "."),
        ...(laneActOf(lane, unread) === "resume" ? { act: { kind: "resume" as const } } : { command: LANDING_START }),
      }),
    );
  } else if (lane.owner.state === "unready") {
    // The lane's hint is prose, and its one-command fix is not in the
    // payload: the hint explains, and the lane's own status is what a person
    // types to read the whole of it (FR-03).
    const why = lane.owner.last_exit ?? "";
    items.push(
      need({
        key: "lane:unready",
        words: why === "" ? "The landing lane can't run." : oneLine("The landing lane can't run: ", clause(why), "."),
        note: lane.owner.retry_hint ?? "",
        command: LANDING_STATUS,
      }),
    );
  }
  const queue = lane.queue ?? [];
  for (const [index, entry] of queue.entries()) {
    // A return is history once its goal was handed in again, or once the
    // goal is done or abandoned; Came back still lists it.
    if (entry.state !== "returned" || handedInAgain(queue, index) || (ended?.[entry.goal] ?? "") !== "") {
      continue;
    }
    items.push(
      need({
        key: `returned:${entry.goal}:${entry.sha}`,
        at: entry.returned_at ?? "",
        words: oneLine("“", titleOf(entry.goal, titles), "” came back."),
        note: firstSentence(reasonWords(entry.reason)),
        act: { kind: "goal", goal: entry.goal },
      }),
    );
  }
  const proof = lane.last_proof;
  if (proof !== undefined && proof !== null && proof.result === "red" && !answered(lane, proof.at)) {
    const attempt = proof.attempt ?? "";
    items.push(
      need({
        key: `proof:${proof.attempt ?? proof.at}`,
        at: proof.at,
        words: oneLine("The landing lane's last proof is red: ", proof.reason === undefined || proof.reason === "" ? "the proof command failed" : clause(proof.reason), "."),
        // A record that names no attempt has no log this page can open; the
        // lane's status names it.
        ...(attempt === "" ? { command: LANDING_STATUS } : { act: { kind: "log" as const, href: proofLogAddress(attempt) } }),
      }),
    );
  }
  return items;
}

/**
 * Whether a red proof has been answered: the landing agent is still on it, a
 * newer proof runs, or the lane returned work at or after the proof ended.
 */
function answered(lane: Lane, at: string): boolean {
  if (lane.agent_alive === true || lane.running_proof?.state === "running") {
    return true;
  }
  const ended = Date.parse(at);
  return (lane.queue ?? []).some((entry) => {
    const returned = Date.parse(entry.returned_at ?? "");
    return entry.state === "returned" && !Number.isNaN(returned) && !(returned < ended);
  });
}

/** Whether the goal of the entry at index was handed in again after it. */
function handedInAgain(queue: readonly LaneEntry[], index: number): boolean {
  return queue.slice(index + 1).some((later) => later.goal === queue[index].goal);
}

function reasonWords(reason: string | undefined): string {
  return reason === undefined || reason === "" ? "no reason was recorded" : reason;
}

/** What a question without a goal is about, in words. */
const ABOUT: Record<string, string> = { lane: "the landing lane", machine: "its machine" };

/** This checkout's open questions, each with Answer. */
function questionNeeds(board: BoardPayload): Need[] {
  return (board.questions ?? []).map((question) => {
    const who = question.machine === "" ? "A seat" : question.machine;
    const about =
      question.goal !== ""
        ? ` about “${titleOf(question.goal, board.titles)}”`
        : question.about !== undefined && question.about !== ""
          ? ` about ${ABOUT[question.about] ?? question.about}`
          : "";
    return need({
      key: `question:${question.id}`,
      at: question.openedAt,
      words: oneLine(`${who} asks${about}: `, question.question, ""),
      act: { kind: "answer" },
    });
  });
}

/* --------------------------------------------------------------- seen -- */

/**
 * The Seen column: when a machine was last heard from, in the standing's own
 * visible words where it is not simply an age (FR-06) — unreachable and for
 * how long, no presence, presence unreadable, or unknown with the clock that
 * is too far ahead. The standing's whole reason is the cell's title. The
 * words come from the standing and the reason the server composed; nothing
 * here judges either.
 */
export type Seen = { words: string; tone: "" | "warn" | "bad"; title: string };

export function seenOf(machine: Machine, now: Date): Seen {
  const age = machine.seen === "" ? "" : ageBetween(machine.seen, now.toISOString());
  const title = [
    machine.reason,
    machine.standing === "unreachable" && machine.since !== "" ? `unreachable since ${dateAndTime(machine.since)}` : "",
    machine.seen === "" ? "" : `last published ${dateAndTime(machine.seen)}`,
  ]
    .filter((part) => part !== "")
    .join(" · ");
  if (machine.standing === "unreachable") {
    return { words: age === "" ? "unreachable" : `unreachable · ${age}`, tone: "bad", title };
  }
  if (machine.standing === "unknown") {
    const reason = machine.reason;
    const words = reason.startsWith("presence unreadable")
      ? "presence unreadable"
      : reason.startsWith("clock ahead")
        ? `unknown, ${reason}`
        : machine.seen === ""
          ? "no presence"
          : "unknown";
    return { words, tone: "warn", title };
  }
  return { words: age === "" ? "no presence" : `${age} ago`, tone: "", title };
}

/* ------------------------------------------------------------- doing -- */

/**
 * What a machine is doing, as the Doing column says it: the words, whether it
 * is working right now (the live dot, and the verdict's count), and where the
 * words came from, which the opened row says.
 */
export type Doing = {
  words: string;
  active: boolean;
  source: "jobs" | "board" | "lane" | "none" | "problem";
  /** A held card that stopped moving: the column says it in the marker colour. */
  stalled?: boolean;
};

/** A stage of a goal's card, as the column says it, and whether it is work in hand. */
const STAGES: Record<string, { words: string; working: boolean }> = {
  "claimed-idle": { words: "claimed idle", working: false },
  build: { words: "building", working: true },
  revise: { words: "revising", working: true },
  "unit-proof": { words: "proving", working: true },
  review: { words: "reviewing", working: true },
  landing: { words: "landing", working: true },
  judgement: { words: "waiting for a verdict", working: false },
  "land-ready": { words: "ready to land", working: false },
  joined: { words: "waiting to land", working: false },
};

/** A job's role, as the column says it. */
const ROLES: Record<string, string> = {
  building: "building",
  builder: "building",
  implementer: "building",
  revise: "revising",
  review: "reviewing",
  reviewer: "reviewing",
  critic: "reviewing",
  "code-critic": "reviewing",
  "design-critic": "reviewing",
  "unit proof": "proving",
  design: "designing",
  designer: "designing",
  working: "working",
};

/** The seat session itself: a seat's own launch, less telling than what it launched. */
const SESSION = "working";

function roleWords(role: string): string {
  return ROLES[role] ?? role;
}

function jobRoundWords(verb: string, round: number, limit: number | null): string {
  if (round <= 0 || (verb !== "reviewing" && round <= 1 && limit === null)) {
    return "";
  }
  return roundWords(round, limit);
}

function sentence(parts: readonly string[]): string {
  return parts.filter((part) => part !== "").join(" · ");
}

/**
 * The Doing column, from the board and the seat records.
 *
 * The board is this computer's own reading of the stage each goal is at, and
 * it records progress no job does (a proof's sections, a review's round of
 * its limit), so a card that is work in hand comes first. A card counts only
 * for a goal the machine holds and only where the board believes it: a card
 * whose claim moved says nothing about this machine. A held card the board
 * does not believe because it stopped moving — stalled, or its writer gone —
 * says the seat is stalled, with no live dot, ahead of the job records, which
 * a stuck seat's still-running job would otherwise read as work. Then the job
 * records — this seat's own, or what another machine published — then a
 * card that only waits, then the lane's queue, and idle. A machine this seat
 * has not heard from is a dash: what it is doing is not known here.
 */
export function doingOf(machine: Machine, seat: BoardSeat | undefined, queue: readonly LaneEntry[], now: Date): Doing {
  if (machine.workingProblem !== "") {
    return { words: machine.workingProblem, active: false, source: "problem" };
  }
  const held = new Set(machine.holds.map((one) => one.goal));
  const cards = (seat?.goals ?? []).filter(
    (goal) => held.has(goal.goal) && (goal.unknown ?? "") === "" && goal.stage !== undefined && STAGES[goal.stage] !== undefined && (goal.stage !== "claimed-idle" || (goal.landed ?? 0) > 0),
  );
  // Work that moves wins over a stalled goal beside it (step-2 design 2a.1):
  // the stalled goal is its own Needs you item.
  const busy = cards.find((goal) => STAGES[goal.stage ?? ""].working);
  if (busy !== undefined) {
    return { words: cardWords(busy, now), active: true, source: "board" };
  }
  const stalled = (seat?.goals ?? []).find((goal) => held.has(goal.goal) && stuck(goal.unknown));
  if (stalled !== undefined) {
    return { words: sentence([STALLED, stageWords(stalled.stage), elapsed(stalled.lastProgressAt, now), stuckUnitWords(stalled.stuck)]), active: false, source: "board", stalled: true };
  }
  const heard = machine.this || machine.standing === "reachable";
  if (heard) {
    const jobs = jobsDoing(machine, now);
    if (jobs !== null) {
      return jobs;
    }
  }
  const waiting = cards[0];
  if (waiting !== undefined) {
    return { words: cardWords(waiting, now), active: false, source: "board" };
  }
  if (queue.some((entry) => entry.state === "waiting" && entry.seat === machine.machine)) {
    return { words: "waiting to land", active: false, source: "lane" };
  }
  return { words: heard ? "idle" : "—", active: false, source: "none" };
}

function cardWords(goal: BoardSeat["goals"][number], now: Date): string {
  const stage = goal.stage ?? "";
  const words = stage === "claimed-idle" ? `${String(goal.landed)} ${goal.landed === 1 ? "unit" : "units"} landed` : STAGES[stage].words;
  let progress = "";
  if (stage === "unit-proof" && goal.proof !== undefined && goal.proof.planned > 0) {
    progress = `${String(goal.proof.done)} of ${String(goal.proof.planned)}`;
  } else if ((stage === "review" || stage === "revise") && goal.round !== undefined) {
    progress = roundWords(goal.round.n, goal.round.max);
  }
  return sentence([words, progress, elapsed(goal.since, now), stuckUnitWords(goal.stuck)]);
}

function stuckUnitWords(stuck: BoardSeat["goals"][number]["stuck"]): string {
  if (stuck === undefined) return "";
  return stuck.step === "" ? `round ${String(stuck.rounds)} of ${String(stuck.limit)}` : `${stuck.kind} step ${String(stuck.minutes)} min`;
}

/** What the job records say: the work in hand, a reservation, or nothing. */
function jobsDoing(machine: Machine, now: Date): Doing | null {
  const running = machine.working.filter((one) => one.job.status === "running");
  if (running.length > 0) {
    const chosen = running.find((one) => one.phase.role !== SESSION) ?? running[0];
    const others = running.filter((one) => one !== chosen && one.phase.role !== SESSION).length;
    return {
      words: sentence([workingWords(chosen), elapsed(chosen.job.startedAt, now), others > 0 ? `and ${String(others)} more` : ""]),
      active: true,
      source: "jobs",
    };
  }
  const reserved = machine.working.find((one) => one.job.status === "pending" || one.job.status === "pending-setup");
  if (reserved !== undefined) {
    return { words: sentence([workingWords(reserved), "waiting to start"]), active: false, source: "jobs" };
  }
  // A machine that publishes only its chain, from an older engine: a chain
  // with a start is work in hand.
  const chain = machine.running;
  if (machine.working.length === 0 && chain !== null && chain.startedAt !== null) {
    const verb = roleWords(chain.role);
    return { words: sentence([verb, jobRoundWords(verb, chain.round, null), elapsed(chain.startedAt, now)]), active: true, source: "jobs" };
  }
  return null;
}

function workingWords(working: Working): string {
  const verb = roleWords(working.phase.role);
  return sentence([verb, jobRoundWords(verb, working.phase.round, working.phase.roundLimit)]);
}

/* ------------------------------------------------------ the landing lane -- */

/** The lane's one state word and its tone. */
export type LaneState = { word: "Running" | "Paused" | "Needs attention"; tone: "ok" | "warn" | "bad" };

/**
 * One word for the lane: Paused when a person paused it, Needs attention when
 * it cannot run, and Running otherwise — a lane with nothing to land is a
 * running lane whose agent starts when work arrives.
 */
export function laneState(lane: Lane): LaneState {
  if (paused(lane)) {
    return { word: "Paused", tone: "bad" };
  }
  if (lane.owner.state === "unready") {
    return { word: "Needs attention", tone: "warn" };
  }
  return { word: "Running", tone: "ok" };
}

/**
 * One hand-in on one of the lane's lists, in the words its line says: the
 * whole line, and for what waits the words after the title (seat and age).
 */
export type LaneItem = { entry: LaneEntry; title: string; words: string; detail: string; at: string; again: boolean };

/**
 * The running proof, as its line says it: the whole line, the goals it holds
 * by title (none where it holds no hand-in of the queue), and the words after
 * them.
 */
export type ProvingItem = { words: string; died: boolean; goals: { id: string; title: string }[]; detail: string };

/** The lane's four lists. */
export type LaneLists = { waiting: LaneItem[]; proving: ProvingItem | null; landed: LaneItem[]; cameBack: LaneItem[] };

/**
 * What the lane holds, as four lists: what waits (title, seat, how long),
 * what is being proved (how long it has run), what landed today (the push's
 * time and the delivered sentence, else the title), and what came back (the
 * title and why, newest first). Came back keeps every return not handed in
 * again, and today's returns that were, as history.
 */
export function laneLists(lane: Lane, titles: Readonly<Record<string, string>> | undefined, now: Date): LaneLists {
  const queue = lane.queue ?? [];
  const item = (entry: LaneEntry, words: string, at: string, again = false, detail = ""): LaneItem => ({
    entry,
    title: titleOf(entry.goal, titles),
    words,
    detail,
    at,
    again,
  });
  const proving = provingOf(lane, titles, now);
  const proved = new Set((proving?.goals ?? []).map((goal) => goal.id));
  const waiting = queue
    .filter((entry) => entry.state === "waiting" && !proved.has(entry.goal))
    .map((entry) => {
      const detail = sentence([entry.seat, elapsed(entry.at, now)]);
      return item(entry, sentence([titleOf(entry.goal, titles), detail]), entry.at, false, detail);
    });
  const landed = queue
    .filter((entry) => entry.state === "landed" && entry.landed_at !== undefined && entry.landed_at !== "" && sameDay(new Date(entry.landed_at), now))
    .map((entry) => item(entry, entry.delivered !== undefined && entry.delivered !== "" ? entry.delivered : titleOf(entry.goal, titles), entry.landed_at ?? ""));
  const cameBack = queue.flatMap((entry, index) => {
    if (entry.state !== "returned") {
      return [];
    }
    const again = handedInAgain(queue, index);
    const returned = entry.returned_at ?? "";
    const today = returned !== "" && sameDay(new Date(returned), now);
    return again && !today ? [] : [item(entry, reasonWords(entry.reason), returned, again)];
  });
  return {
    waiting,
    proving,
    landed: landed.sort(newerFirst),
    cameBack: cameBack.sort(newerFirst),
  };
}

/**
 * The lane's heading line beside its state word: how many hand-ins wait, and
 * what proves — or, paused, that it lands nothing until resumed.
 */
export function laneCounts(lane: Lane, lists: LaneLists): string {
  const waiting = `${String(lists.waiting.length)} waiting`;
  if (paused(lane)) {
    return `${waiting} · lands nothing until resumed`;
  }
  const proving = lists.proving;
  if (proving === null || proving.died) {
    return `${waiting} · nothing proving`;
  }
  return `${waiting} · ${proving.goals.length === 0 ? "a proof running" : `${String(proving.goals.length)} proving`}`;
}

function newerFirst(left: LaneItem, right: LaneItem): number {
  return (Date.parse(right.at) || 0) - (Date.parse(left.at) || 0);
}

/**
 * The running proof: the hand-ins its commit holds, by title, and how long it
 * has run. A proof that holds none of the queue says how long alone.
 */
function provingOf(lane: Lane, titles: Readonly<Record<string, string>> | undefined, now: Date): ProvingItem | null {
  const proof = lane.running_proof;
  if (proof === undefined || proof === null) {
    return null;
  }
  if (proof.state === "died") {
    return { words: "stopped without a result; the next proof runs it again", died: true, goals: [], detail: "" };
  }
  const ran = elapsed(proof.since, now);
  const detail = ran === "" ? "running" : `started ${ran} ago`;
  const goals = (proof.goals ?? []).map((goal) => ({ id: goal, title: titleOf(goal, titles) }));
  const words = goals.length === 0 ? detail : `${joinWords(goals.map((goal) => goal.title))} · ${detail}`;
  return { words, died: false, goals, detail };
}
