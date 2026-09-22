/**
 * What a split left behind, read out of the records the board already has.
 *
 * The engine records a split in three places and none of them is a "members"
 * field. The parent is concluded, its id is appended to the root record's
 * decomposed list — which is what the projection's `decomposed` flag is — and
 * each new member is born with its `Arc` set to the parent's own id
 * (internal/goal/split.go). So the relationship is derivable from the payload
 * as it already stands, and nothing was added to it for this: a member is a
 * goal whose arc names a decomposed goal, and a parent's members are the
 * goals whose arc is its id. That is the same scan the engine's own split
 * validation makes.
 *
 * The consequence for the board is the one the master states: a goal retired
 * by decomposition is not a delivered outcome. It is read out of Done and
 * shown as its members instead.
 */

import type { Row } from "./api";

/** True when this goal was retired by a split rather than delivered. */
export function isParent(row: Row): boolean {
  return row.decomposed;
}

/**
 * The split this goal came out of, or null.
 *
 * An arc is the general grouping too, so naming an arc is not enough: the
 * parent has to be a goal this payload carries AND one the root retired by
 * decomposition. A goal in a planning arc therefore reads as being in an arc,
 * which is what it is, and only a split member reads as part of a parent.
 */
export function parentOf(row: Row, all: readonly Row[]): Row | null {
  if (row.arc === "") {
    return null;
  }
  return all.find((candidate) => candidate.ref.id === row.arc && isParent(candidate)) ?? null;
}

/** The goals a split produced, in the order the payload carries them. */
export function membersOf(parent: Row, all: readonly Row[]): Row[] {
  return all.filter((candidate) => candidate.arc === parent.ref.id);
}

/**
 * The arc to show on a card as an arc, or "" where there is none to show.
 *
 * A split member's arc is its parent's id and is already said, in full and
 * with a link, as "part of …". Saying it twice would read as two different
 * facts about the same goal.
 */
export function arcOn(row: Row, all: readonly Row[]): string {
  return row.arc !== "" && parentOf(row, all) === null ? row.arc : "";
}
