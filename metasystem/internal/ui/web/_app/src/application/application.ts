import type { Landed, Page, Problem } from "./api";

/**
 * What the Application page says, as statements a test can point at.
 *
 * Nothing here reaches the network, reads a clock on its own, or renders
 * anything. The server sends the rows already ordered — newest conclusion
 * first, the undated last — and this file groups them into weeks, narrows
 * them, and turns the payload's facts into the sentences the page shows.
 *
 * The one rule worth naming twice: this page is a work history and says so.
 * A conclusion dates an end, sometimes an administrative one, so the block is
 * "What concluded" and the count is "goals concluded". Nothing here infers a
 * capability from a conclusion, because nothing in the record supports one.
 */

/** How many weeks open when the block is first drawn; the rest are earlier. */
export const WEEKS_SHOWN = 4;

/** How many label chips the line carries before the rest are asked for. */
export const CHIPS_SHOWN = 4;

/** One week of concluded goals: the line, and the rows behind it. */
export type Week = {
  /** The key the group is identified by, which is its Monday as a date. */
  id: string;
  /** What the line says: the week, or that nothing dated these rows. */
  title: string;
  rows: Landed[];
  count: number;
  /** How many of them landed since the last visit here. */
  fresh: number;
};

/** What the block is narrowed to. A sitting's state, never persisted. */
export type Narrowing = { find: string; label: string };

export const noNarrowing: Narrowing = { find: "", label: "" };

export function isNarrowed(narrowing: Narrowing): boolean {
  return narrowing.find.trim() !== "" || narrowing.label !== "";
}

/**
 * The header's count line, in the words the design settled on.
 *
 * "Goals concluded" rather than "landed": a conclusion records an end, and
 * some of those ends are administrative — a duplicate withdrawn, a
 * requirement absorbed into another goal. The page must not claim that four
 * hundred goals each shipped something.
 */
export function countLine(page: Page): string {
  const parts = [`${String(page.counts.landed)} goals concluded`];
  if (page.counts.thisMonth > 0) {
    parts.push(`${String(page.counts.thisMonth)} this month`);
  }
  if (page.counts.new > 0) {
    parts.push(`${String(page.counts.new)} since your last visit`);
  }
  return parts.join(" · ");
}

/**
 * What the header says about the engine.
 *
 * "Last published" is the whole of the honesty: presence is a record at one
 * tick, and a seat that has not ticked since it was rebuilt publishes the
 * build it had when it last ticked. A seat with no record at all says so,
 * rather than showing an empty build a human would read as one.
 */
export function engineLine(page: Page): string {
  const engine = page.engine;
  if (engine === null || engine.build === "") {
    return "no engine build available";
  }
  const parts = [`last published MetaSystem engine ${engine.build}`, `generation ${String(engine.generation)}`];
  const at = dayAndMinute(engine.publishedAt);
  if (at !== "") {
    parts.push(`published ${at}`);
  }
  return parts.join(" · ");
}

/**
 * What the header says about the subject and the mode.
 *
 * The self-hosted case is the one that can be misread: the workspace is the
 * MetaSystem building itself, and the instance doing the work is a different
 * thing from the product under development. So it is said in words rather
 * than left to a mode chip.
 */
export function subjectLine(page: Page): string {
  return page.mode === "self-hosted" ? `${page.subject} · self-hosted` : `${page.subject} · built with MetaSystem`;
}

/** The sentence beneath it, where the two could be confused. */
export function modeWords(page: Page): string {
  return page.mode === "self-hosted"
    ? "The MetaSystem as the product under development; the instance doing the work is on Fleet."
    : "";
}

/**
 * The rows the tools leave on screen.
 *
 * Find reads the id, the intent and the conclusion, because those are the
 * three things a human remembers about work that landed: what it was called,
 * what it was for, and the commit it went in as.
 */
export function shown(rows: readonly Landed[], narrowing: Narrowing): Landed[] {
  const wanted = narrowing.find.trim().toLowerCase();
  return rows.filter((row) => {
    if (narrowing.label !== "" && !row.labels.includes(narrowing.label)) {
      return false;
    }
    if (wanted === "") {
      return true;
    }
    return `${row.id} ${row.intent} ${row.concluded}`.toLowerCase().includes(wanted);
  });
}

/** The labels of the rows on screen, with how many carry each, commonest first. */
export function labelsIn(rows: readonly Landed[]): { label: string; count: number }[] {
  const counted = new Map<string, number>();
  for (const row of rows) {
    for (const label of row.labels) {
      counted.set(label, (counted.get(label) ?? 0) + 1);
    }
  }
  return [...counted.entries()]
    .map(([label, count]) => ({ label, count }))
    .sort((a, b) => (a.count === b.count ? a.label.localeCompare(b.label) : b.count - a.count));
}

/** How many are shown of how many there are, where the two differ. */
export function shownCount(total: number, showing: number): string {
  const concluded = `${String(total)} concluded`;
  return showing === total ? concluded : `${concluded} · ${String(showing)} shown`;
}

/**
 * The rows as weeks, newest week first, in the order the server sent them.
 *
 * A week is Monday to Sunday in the reader's own time zone, because a human
 * reading "this week" means the week on the wall behind them. The rows are
 * already ordered, so the groups come out in order without a second sort, and
 * a row nothing dated cannot be placed in a week at all: those are one group
 * of their own at the end, which says so rather than pretending to a date.
 */
export function weeksOf(rows: readonly Landed[]): Week[] {
  const weeks: Week[] = [];
  const byId = new Map<string, Week>();
  for (const row of rows) {
    const id = weekOf(row.doneAt);
    let week = byId.get(id);
    if (week === undefined) {
      week = { id, title: weekTitle(id), rows: [], count: 0, fresh: 0 };
      byId.set(id, week);
      weeks.push(week);
    }
    week.rows.push(row);
    week.count += 1;
    if (row.new) {
      week.fresh += 1;
    }
  }
  return weeks;
}

/** The id of the week a conclusion falls in, or the undated group's own. */
const UNDATED = "undated";

function weekOf(doneAt: string): string {
  const at = parse(doneAt);
  if (at === null) {
    return UNDATED;
  }
  const monday = new Date(at.getFullYear(), at.getMonth(), at.getDate());
  // getDay is 0 on Sunday, which belongs to the week that began six days
  // before it rather than to the one starting the next morning.
  monday.setDate(monday.getDate() - ((monday.getDay() + 6) % 7));
  return `${String(monday.getFullYear())}-${pad(monday.getMonth() + 1)}-${pad(monday.getDate())}`;
}

const MONTHS = [
  "January", "February", "March", "April", "May", "June",
  "July", "August", "September", "October", "November", "December",
];

function weekTitle(id: string): string {
  if (id === UNDATED) {
    return "Nothing dated these";
  }
  const monday = parse(`${id}T00:00:00`);
  if (monday === null) {
    return id;
  }
  return `Week of ${String(monday.getDate())} ${MONTHS[monday.getMonth()]} ${String(monday.getFullYear())}`;
}

/** What the "n new" part of a week's line says, or "" where none is. */
export function freshLine(week: Week): string {
  return week.fresh === 0 ? "" : `${String(week.fresh)} new`;
}

/**
 * A record's first statement, without the rest of it.
 *
 * A row is one line, and a conclusion is often three sentences naming the
 * commit, the machine and what is left. The first one is what tells this row
 * from its neighbours; the whole of it is in the open row.
 */
export function firstSentence(text: string): string {
  const trimmed = text.trim();
  const cut = trimmed.search(/[.\n]/);
  return cut > 0 ? trimmed.slice(0, cut).trim() : trimmed;
}

/** The local calendar day of a recorded instant, or "" where none was. */
export function dayOf(stamp: string): string {
  const at = parse(stamp);
  if (at === null) {
    return "";
  }
  return `${String(at.getFullYear())}-${pad(at.getMonth() + 1)}-${pad(at.getDate())}`;
}

/** The local day and clock to the minute, for the engine's own tick. */
export function dayAndMinute(stamp: string): string {
  const at = parse(stamp);
  if (at === null) {
    return "";
  }
  return `${dayOf(stamp)} ${pad(at.getHours())}:${pad(at.getMinutes())}`;
}

/* ------------------------------------------------------ known problems -- */

/**
 * The word a concluded row's status opens with.
 *
 * It travels onto the row's one line because an accepted limitation still
 * exists: "ACCEPTED" and "FIXED" are two different things to know about a
 * problem, and a block that showed neither would read as a list of solved
 * ones. The whole status is in the open row; nothing here interprets it.
 */
export function statusWord(problem: Problem): string {
  const opening = problem.status.trim().split(/[\s,:]/)[0];
  return opening === undefined ? "" : opening;
}

/** What the "Show concluded" disclosure says. */
export function concludedLine(problems: readonly Problem[]): string {
  return `Show concluded (${String(problems.length)})`;
}

/**
 * The one line for the rows the reader could not read.
 *
 * It exists because five of this register's open rows carry three or four
 * cells today. A block that silently skipped them would be a page claiming
 * this project knows about fewer problems than it does, so the count is said
 * and the register itself is one click away.
 */
export function unreadLine(unread: number): string {
  if (unread === 0) {
    return "";
  }
  return unread === 1
    ? "1 issue could not be read as a row"
    : `${String(unread)} issues could not be read as rows`;
}

/* ----------------------------------------------------------- the small -- */

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
