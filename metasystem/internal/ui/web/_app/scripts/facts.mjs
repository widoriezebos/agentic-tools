/**
 * The stage Overview shows for each goal of the host board, by goal id, in the
 * words the board gives it. A goal the board marks Unknown supplies none, as a
 * goal without a stage supplies none.
 */
export function expectedStages(board) {
  const goals = (board?.seats ?? []).flatMap((seat) => seat.goals ?? []);
  const held = goals.filter((goal) => (goal.unknown ?? "") === "" && typeof goal.stage === "string" && goal.stage !== "");
  return new Map(held.map((goal) => [goal.goal, stageText(goal)]));
}

/** A stage as board.StageText words it: review round 2 of 3, unit proof 120 of 189, claimed idle. */
function stageText({ stage, round, proof }) {
  let text = stage.replaceAll("-", " ");
  if (proof && proof.planned > 0 && stage === "unit-proof") {
    text += ` ${String(proof.done)} of ${String(proof.planned)}`;
  }
  if (round && (stage === "review" || stage === "revise")) {
    text += ` round ${String(round.n)}` + (round.max == null ? "" : ` of ${String(round.max)}`);
  }
  return text;
}
