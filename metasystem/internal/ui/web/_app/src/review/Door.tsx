import { createContext, useContext, useState } from "react";
import { useNavigate } from "react-router";

import { doorLine } from "./room";
import type { ReviewDoor as Door } from "../backlog/api";
import { Help } from "../help/Help";
import { usePartner } from "../partner/store";
import { reviewPath } from "../routes";
import { Button } from "../shell/controls";

/**
 * Review it, and the door back (g1-s65 D2, D9).
 *
 * On a goal waiting to land — its Review lane card and its page — and on a done
 * goal's page, Review it opens the room. Once a review of the goal stands, the
 * same place is its door: "In review · you stepped out 2h ago · 3 findings, 1
 * unanswered", read from the record's counts and the room's time, and pressing it
 * reopens the room where the human was.
 */

/** The board's reviews, for the cards beneath it. */
export const ReviewDoors = createContext<readonly Door[]>([]);

/** The door of this human's standing review of one goal, or null. */
export function standingDoor(doors: readonly Door[], goal: string): Door | null {
  return doors.find((door) => door.goal === goal && door.standing) ?? null;
}

export function ReviewItOrDoor({ goal, doors, now = new Date() }: { goal: string; doors?: readonly Door[]; now?: Date }) {
  const board = useContext(ReviewDoors);
  const { startSitting, sittingBusy } = usePartner();
  const navigate = useNavigate();
  const [refusal, setRefusal] = useState("");
  const door = standingDoor(doors ?? board, goal);
  if (door !== null) {
    return (
      <p className="ms-review-door">
        <button
          type="button"
          className="ms-review-door-open"
          title="Go back into the review room where you left it"
          onClick={() => {
            void navigate(reviewPath(door.record));
          }}
        >
          {doorLine({ findings: door.findings, unanswered: door.unanswered, steppedOutAt: door.steppedOutAt ?? "" }, now)}
        </button>
      </p>
    );
  }
  return (
    <p className="ms-review-door">
      <Button
        disabled={sittingBusy}
        onClick={() => {
          setRefusal("");
          startSitting({ purpose: "review", subject: { kind: "goal", id: goal, title: goal } }).then(
            (opened) => {
              if (opened !== "") {
                void navigate(reviewPath(opened));
              }
            },
            (error: unknown) => {
              setRefusal(error instanceof Error ? error.message : String(error));
            },
          );
        }}
      >
        Review it
      </Button>
      <Help id="review-room" />
      {refusal !== "" && (
        <span className="ms-deposit-refusal" role="status">
          {refusal}
        </span>
      )}
    </p>
  );
}
