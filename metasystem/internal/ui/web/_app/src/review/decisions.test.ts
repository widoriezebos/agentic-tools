import { describe, expect, it } from "vitest";

import {
  askedFollowing,
  followUpsFor,
  earlierTurnsOf,
  verdictAnswers,
  verdictDecisions,
  landingWaits,
  leadParagraph,
  choiceSaid,
  seeing,
  DECISIONS,
  decisionsFor,
  effectiveDecision,
  examinationOf,
  findingsSummary,
  goalHeading,
  nodPlan,
  outcomeBody,
  recommendedWay,
  reviewerWay,
  riskSaid,
  versionWhen,
  wayConsequence,
  type RoomFinding,
} from "./room";
import { ACCEPTED, FIX, followUp, NOT_A_PROBLEM } from "../partner/sitting";
import type { Message } from "../partner/api";

/**
 * The guided review's rules (review-findings-read-as-decisions §3, folded with
 * Astra's round 1): a summary before any card, the recommended way by rule,
 * the nod that asks instead of refusing, and the Outcome the interface
 * composes from the record.
 */

function finding(over: Partial<RoomFinding> = {}): RoomFinding {
  return {
    id: "deposit:t1#0", title: "A press that dies halfway leaves the ledger locked.",
    why: "The next press waits forever.", severity: "blocks", recommend: "must-fix",
    reason: "Release the lock on every way out.", evidence: "", anchor: "", consequence: "",
    answer: "", recorded: false, ...over,
  };
}

const BLOCKS = finding();
const WORTH = finding({ id: "deposit:t1#1", title: "Only the happy path is tested.", severity: "fix", recommend: "fix-later",
  reason: "Open a follow-up goal to test each failure." });
const NOTE = finding({ id: "deposit:t1#2", title: "The design still describes two locks.", severity: "note",
  recommend: "not-a-problem", reason: "The design is history." });

describe("the summary line", () => {
  it("counts the findings by severity and says what the reviewer recommends, and why", () => {
    expect(findingsSummary([BLOCKS, WORTH, NOTE])).toEqual({
      counts: "3 findings. 1 blocks landing, 1 is worth fixing, 1 is a note.",
      recommends: "send it back, because of the one that blocks.",
    });
    expect(findingsSummary([WORTH, NOTE, { ...NOTE, id: "n2" }])).toEqual({
      counts: "3 findings. 1 is worth fixing, 2 are notes.",
      recommends: "land it.",
    });
    expect(findingsSummary([BLOCKS, { ...BLOCKS, id: "b2" }]).recommends).toBe("send it back, because of the 2 that block.");
    expect(findingsSummary([finding({ severity: "fix" })]).recommends)
      .toBe("send it back, because one finding must be fixed before landing.");
  });

  it("never says the reviewer recommends landing a finding it recommended nothing for (F-2 of read 54dafc9b)", () => {
    const older = finding({ id: "deposit:t0#0", title: "The owner reads the wrong tree.", why: "", severity: "", recommend: "", reason: "" });
    expect(findingsSummary([older]).recommends)
      .toBe("nothing: it gave no recommendation for this finding, so it needs your own decision.");
    expect(findingsSummary([older, { ...older, id: "deposit:t0#1" }]).recommends)
      .toBe("nothing: it gave no recommendation for these findings, so each needs your own decision.");
    expect(findingsSummary([older, NOTE]).recommends)
      .toBe("land it, for the findings it gave a recommendation for; one has no recommendation and needs your own decision.");
  });
});

describe("the recommended way", () => {
  it("is Send it back while any finding's decision is must fix, whatever its severity (RF-01)", () => {
    expect(recommendedWay([BLOCKS, WORTH, NOTE])).toBe("send back");
    expect(recommendedWay([{ ...BLOCKS, answer: ACCEPTED("the next begin releases it"), recorded: true }, WORTH])).toBe("land");
    expect(recommendedWay([])).toBe("land");
    expect(recommendedWay([WORTH, NOTE])).toBe("land");
    // A person's own must fix on a finding worth fixing sends it back too.
    expect(recommendedWay([{ ...WORTH, answer: FIX, recorded: true }, NOTE])).toBe("send back");
    // And the reviewer's word alone is the summary's: blocks and must fix.
    expect(reviewerWay([{ ...WORTH, recommend: "must-fix" }])).toBe("send back");
  });

  it("reads a decision the person made before the reviewer's", () => {
    expect(effectiveDecision(BLOCKS)).toBe("fix");
    expect(effectiveDecision({ ...BLOCKS, answer: followUp("tests-for-failures"), recorded: true })).toBe("follow-up");
    expect(effectiveDecision({ ...BLOCKS, answer: "unanswered", recorded: true })).toBe("fix");
    expect(effectiveDecision(finding({ recommend: "" }))).toBe("");
  });
});

describe("the four decisions", () => {
  it("are offered by severity: a note is fixed later or is not a problem", () => {
    expect(decisionsFor("note")).toEqual(["follow-up", "not a problem"]);
    expect(decisionsFor("fix")).toEqual(["fix", "follow-up", "not a problem", "accepted"]);
    expect(decisionsFor("blocks")).toEqual(["fix", "follow-up", "not a problem", "accepted"]);
  });

  it("say what the press records and what follows it, and nothing it does not do (fix round 4)", () => {
    expect(DECISIONS.find((one) => one.decision === "fix")?.consequence).toBe(
      "The builder gets this as a correction when you send the goal back; landing over it asks you first.",
    );
    expect(DECISIONS.find((one) => one.decision === "fix")?.consequence).not.toContain("does not land until");
  });

  it("say only what step 1 records: an accepted risk is on this review, and the goal's own is a terminal's", () => {
    const accept = DECISIONS.find((one) => one.decision === "accepted");
    expect(accept?.consequence).toBe("Say why. Your acceptance and your reason are recorded on this review, in your name.");
    expect(accept?.consequence).not.toContain("may land");
    expect(DECISIONS.map((one) => one.label)).toEqual([
      "Must fix before landing", "Fix after landing", "Not a problem", "I accept this risk",
    ]);
  });
});

describe("the nod", () => {
  it("is one press when every finding is decided and nothing must be fixed", () => {
    const plan = nodPlan([{ ...WORTH, answer: followUp("tests"), recorded: true }, { ...NOTE, answer: NOT_A_PROBLEM("history"), recorded: true }]);
    expect(plan.asks).toBe(false);
    expect(plan.following).toEqual([]);
    expect(plan.corrections).toEqual([]);
  });

  it("is one press on a review with no findings at all", () => {
    expect(nodPlan([]).asks).toBe(false);
  });

  it("asks once, listing each undecided finding with what will be recorded", () => {
    const plan = nodPlan([WORTH, NOTE]);
    expect(plan.asks).toBe(true);
    expect(plan.following.map((one) => [one.finding.title, one.said])).toEqual([
      ["Only the happy path is tested.", "fix after landing: a follow-up goal is opened for it"],
      ["The design still describes two locks.", "not a problem, with the reviewer's reason: The design is history."],
    ]);
    expect(plan.followUps.map((one) => one.id)).toEqual([WORTH.id]);
  });

  it("refuses an empty reason where it lands over a correction, and needs none where it does not", () => {
    expect(landingWaits(nodPlan([BLOCKS]), "  ")).toBe(true);
    expect(landingWaits(nodPlan([BLOCKS]), "the next begin releases it")).toBe(false);
    expect(landingWaits(nodPlan([WORTH]), "")).toBe(false);
  });

  it("asks for a reason first where a finding must be fixed, naming each correction and its consequence (RF-01)", () => {
    const plan = nodPlan([BLOCKS, NOTE, { ...WORTH, answer: FIX, recorded: true }]);
    expect(plan.asks).toBe(true);
    expect(plan.corrections.map((one) => one.title)).toEqual([BLOCKS.title, WORTH.title]);
    expect(plan.impact).toBe(
      "2 findings must be fixed before landing: one blocks landing in the reviewer's view, and you decided one must be fixed. " +
        "Landing records each of these as a risk you accept, in your name and with your reason; the builder gets no correction. " +
        "To keep them, press Send it back instead; to reverse it once recorded and before it lands, start a new review from the board and send it back.",
    );
  });
});

describe("what a verdict records (fix round 1, F-2)", () => {
  const ACCEPTING = finding({ id: "deposit:t1#3", title: "The retry has no ceiling.", severity: "fix", recommend: "accept",
    reason: "The lease bounds it." });

  it("on a send-back, records every untouched finding as the reviewer recommended, in the same write as the corrections", () => {
    const plan = nodPlan([BLOCKS, WORTH, NOTE, ACCEPTING]);
    expect(verdictAnswers(verdictDecisions("send back", plan, "", { [WORTH.id]: "tests-for-failures" }))).toEqual([
      { id: BLOCKS.id, answer: `${FIX} (as the reviewer recommended)` },
      { id: WORTH.id, answer: followUp("tests-for-failures") },
      { id: NOTE.id, answer: `${NOT_A_PROBLEM("The design is history.")} (as the reviewer recommended)` },
      // Never an acceptance in the person's name from a recommendation alone
      // (fix round 4, R-143-m1e): the recommendation is kept, as not decided.
      { id: ACCEPTING.id, answer: "not decided — the reviewer recommends accepting the risk: The lease bounds it." },
    ]);
  });

  it("on a send-back, lists and opens what the untouched risks follow, and records no follow-up without its goal", () => {
    const blockingLater = finding({ id: "deposit:t1#5", title: "Only one press is tested.", recommend: "fix-later", reason: "Open a goal." });
    const plan = nodPlan([blockingLater, ACCEPTING]);
    expect(followUpsFor(plan, "send back").map((one) => one.id)).toEqual([blockingLater.id]);
    expect(followUpsFor(plan, "land")).toEqual([]);
    expect(askedFollowing(plan, "send back").map((one) => [one.finding.id, one.said])).toEqual([
      [blockingLater.id, "fix after landing: a follow-up goal is opened for it"],
      [ACCEPTING.id, "not decided: the reviewer's recommendation to accept the risk is kept (The lease bounds it.)"],
    ]);
    expect(askedFollowing(plan, "land")).toEqual([]);
    expect(verdictAnswers(verdictDecisions("send back", plan, "", {})).map((one) => one.id)).toEqual([ACCEPTING.id]);
    expect(verdictAnswers(verdictDecisions("send back", plan, "", { [blockingLater.id]: "one-press-tests" }))[0])
      .toEqual({ id: blockingLater.id, answer: followUp("one-press-tests") });
  });

  it("on a nod, asks the person's own reason for every untouched finding that blocks or whose recommendation is to accept (fix round 4, R-143-m1e)", () => {
    const blockingAccept = finding({ id: "deposit:t1#4", title: "The ledger can be read half-written.", recommend: "accept",
      reason: "Readers retry." });
    const plan = nodPlan([blockingAccept, ACCEPTING, NOTE]);
    expect(plan.risks.map((one) => one.id)).toEqual([blockingAccept.id, ACCEPTING.id]);
    expect(plan.following.map((one) => one.finding.id)).toEqual([NOTE.id]);
    expect(plan.asks).toBe(true);
    expect(landingWaits(plan, " ")).toBe(true);
    expect(plan.impact).toBe(
      "2 findings you have not decided would be recorded as risks you accept, in your name and with your reason: " +
        "one blocks landing in the reviewer's view, and the reviewer recommends accepting one. " +
        "Landing lets through what they describe, and that risk stays with the goal after it lands. " +
        "To undo it before it lands, start a new review from the board and send it back.",
    );
    expect(verdictAnswers(verdictDecisions("land", plan, "readers retry, and the lease bounds the retry", {}))).toEqual([
      { id: blockingAccept.id, answer: ACCEPTED("readers retry, and the lease bounds the retry") },
      { id: ACCEPTING.id, answer: ACCEPTED("readers retry, and the lease bounds the retry") },
      { id: NOTE.id, answer: `${NOT_A_PROBLEM("The design is history.")} (as the reviewer recommended)` },
    ]);
    expect(wayConsequence("land", [blockingAccept], "complete")).toContain("needs your own decision before landing");
  });

  it("on a nod, asks the person's own decision for an untouched finding the reviewer recommended nothing for, the impact first (RULING-R-143-m1e of read 530a7c87)", () => {
    // A review record written before the layers: no severity, no recommendation.
    const older = finding({ id: "deposit:t0#0", title: "The owner reads the wrong tree.", why: "", severity: "", recommend: "", reason: "" });
    const plan = nodPlan([older, NOTE]);
    expect(plan.risks.map((one) => one.id)).toEqual([older.id]);
    expect(plan.following.map((one) => one.finding.id)).toEqual([NOTE.id]);
    expect(plan.asks).toBe(true);
    expect(landingWaits(plan, " ")).toBe(true);
    expect(plan.impact).toBe(
      "One finding you have not decided would be recorded as a risk you accept, in your name and with your reason: " +
        "the reviewer recommended nothing for one. " +
        "Landing lets through what it describes, and that risk stays with the goal after it lands. " +
        "To undo it before it lands, start a new review from the board and send it back.",
    );
    expect(riskSaid(older)).toBe("The reviewer recommended nothing: ");
    expect(riskSaid(BLOCKS)).toBe("Blocks landing: ");
    expect(riskSaid(finding({ severity: "fix", recommend: "accept" }))).toBe("The reviewer recommends accepting this risk: ");
    const landed = verdictAnswers(verdictDecisions("land", plan, "the tree is read once, at the press", {}));
    expect(landed).toEqual([
      { id: older.id, answer: ACCEPTED("the tree is read once, at the press") },
      { id: NOTE.id, answer: `${NOT_A_PROBLEM("The design is history.")} (as the reviewer recommended)` },
    ]);
    // Nothing is ever recorded as not decided on a land.
    expect(landed.map((one) => one.answer).join(" ")).not.toContain("not decided");
    expect(wayConsequence("land", [older], "complete")).toContain("needs your own decision before landing");
    // A send-back keeps what the reviewer did not recommend undecided, as before.
    expect(verdictAnswers(verdictDecisions("send back", nodPlan([older]), "", {}))).toEqual([
      { id: older.id, answer: "not decided — the reviewer recommended nothing" },
    ]);
  });

  it("on a nod, records the corrections accepted with the person's reason and the rest as the reviewer recommended", () => {
    const plan = nodPlan([BLOCKS, WORTH, { ...NOTE, answer: NOT_A_PROBLEM("mine"), recorded: true }]);
    expect(verdictAnswers(verdictDecisions("land", plan, "the next begin releases it", { [WORTH.id]: "tests" }))).toEqual([
      { id: BLOCKS.id, answer: ACCEPTED("the next begin releases it") },
      { id: WORTH.id, answer: followUp("tests") },
    ]);
  });

  it("keeps a decision the person made, and keeps those decisions through the next version", () => {
    const decided = { ...BLOCKS, answer: FIX, recorded: true };
    expect(verdictAnswers(verdictDecisions("send back", nodPlan([decided]), "", {}))).toEqual([]);
  });
});

describe("a look that has not finished, with findings already raised (fix round 1, R-143-m1e)", () => {
  it("says before the press that the report is incomplete, what landing lets through, what may still come, and how to undo it", () => {
    const said = wayConsequence("land", [WORTH], "reviewing");
    expect(said).toContain("The reviewer has not finished looking at this version: what you see is what it has raised so far, and more may still come.");
    expect(said).toContain("Landing now lets through whatever it has not looked at yet.");
    expect(said).toContain("To undo it before it lands, start a new review from the board and send it back.");
    expect(wayConsequence("land", [WORTH], "stopped")).toContain("The reviewer's look at this version did not finish");
    expect(wayConsequence("land", [WORTH], "complete")).not.toContain("not finished");
  });

  it("says Send it back keeps what was not decided as the reviewer recommends", () => {
    expect(wayConsequence("send back", [BLOCKS, WORTH], "complete"))
      .toContain("What you did not decide is recorded as the reviewer recommends.");
  });

  it("says Send it back records not decided where the reviewer recommended nothing", () => {
    const older = finding({ id: "deposit:t0#0", title: "The owner reads the wrong tree.", why: "", severity: "", recommend: "", reason: "" });
    expect(wayConsequence("send back", [older, WORTH], "complete"))
      .toContain("What you did not decide is recorded as the reviewer recommends, or as not decided where the reviewer recommended nothing.");
    expect(wayConsequence("send back", [BLOCKS, WORTH], "complete")).not.toContain("recommended nothing");
  });
});

describe("the ways' consequences", () => {
  it("say that a nod on an unfinished look lands without the reviewer's report (RF-06)", () => {
    expect(wayConsequence("land", [], "reviewing")).toContain("The reviewer has not finished looking at this version");
    expect(wayConsequence("land", [], "complete")).toBe(
      "The seat lands it on its next turn. Nothing else follows: no finding, no follow-up. This is recorded on the goal in your name.",
    );
    expect(wayConsequence("send back", [], "complete")).toBe(
      "Say what must change first. Your words go to the builder as a correction, and the goal leaves Review until it comes back fixed.",
    );
  });
});

describe("the examination", () => {
  const opening = (outcome?: Message["outcome"]): Message[] => [
    { id: "m1", turn: "t1", role: "human", text: "Open this review of the work recorded in x.md.", at: "", interface: true },
    ...(outcome === undefined ? [] : [{ id: "m2", turn: "t1", role: "partner" as const, text: "", at: "", outcome }]),
  ];
  it("is complete, still running, stopped or failed, by the opening turn's own outcome (RF-06)", () => {
    expect(examinationOf(opening("complete"), "")).toBe("complete");
    expect(examinationOf(opening(), "t1")).toBe("reviewing");
    expect(examinationOf(opening(), "")).toBe("reviewing");
    expect(examinationOf(opening("stopped"), "")).toBe("stopped");
    expect(examinationOf(opening("failed"), "")).toBe("failed");
    expect(examinationOf(opening("refused"), "")).toBe("failed");
  });

  it("is complete only once every walk asked after the newest opening ended done too (fix round 2, F-2)", () => {
    const walk = (turn: string, outcome?: Message["outcome"]): Message[] => [
      { id: `${turn}q`, turn, role: "human", text: "Walk me through Built for the review in x.md: what was built.", at: "", interface: true },
      ...(outcome === undefined ? [] : [{ id: `${turn}a`, turn, role: "partner" as const, text: "", at: "", outcome }]),
    ];
    expect(examinationOf([...opening("complete"), ...walk("t2")], "t2")).toBe("reviewing");
    expect(examinationOf([...opening("complete"), ...walk("t2")], "")).toBe("reviewing");
    expect(examinationOf([...opening("complete"), ...walk("t2", "failed")], "")).toBe("failed");
    expect(examinationOf([...opening("complete"), ...walk("t2", "stopped")], "")).toBe("stopped");
    expect(examinationOf([...opening("complete"), ...walk("t2", "complete")], "")).toBe("complete");
    // A walk asked before the newest opening was about an earlier version.
    expect(examinationOf([...walk("t0", "failed"), ...opening("complete")], "")).toBe("complete");
  });

  it("follows the look asked once more after one that stopped, and keeps what it raised as this version's (fix round 3, F-2)", () => {
    const once = (outcome?: Message["outcome"]): Message[] => [
      { id: "r1", turn: "t3", role: "human", text: "Open this review once more, in x.md: your look ... did not finish.", at: "", interface: true },
      ...(outcome === undefined ? [] : [{ id: "r2", turn: "t3", role: "partner" as const, text: "", at: "", outcome }]),
    ];
    expect(examinationOf([...opening("stopped"), ...once()], "t3")).toBe("reviewing");
    expect(examinationOf([...opening("stopped"), ...once("complete")], "")).toBe("complete");
    expect(earlierTurnsOf([...opening("stopped"), ...once("complete")])).toEqual([]);
    // A move to the current version is what makes what came before it earlier.
    const moved: Message[] = [{ id: "a1", turn: "t4", role: "human", text: "Open this review again, in x.md, on the version ...", at: "", interface: true }];
    expect(earlierTurnsOf([...opening("stopped"), ...once("complete"), ...moved])).toEqual(["t1", "t3"]);
  });
});

describe("the Outcome the interface composes", () => {
  it("says what was looked at and every finding with its decision", () => {
    const body = outcomeBody([
      { ...BLOCKS, answer: ACCEPTED("the next begin releases it"), recorded: true },
      { ...WORTH, answer: followUp("tests-for-failures"), recorded: true },
    ], "Examined: the reviewer's report");
    expect(body).toBe(
      "Examined: the reviewer's report\n\n" +
        "Findings:\n" +
        "- Blocks landing: A press that dies halfway leaves the ledger locked. — accepted — the next begin releases it\n" +
        "- Worth fixing: Only the happy path is tested. — follow-up — goal tests-for-failures",
    );
    expect(outcomeBody([], "Examined: the reviewer's report")).toBe(
      "Examined: the reviewer's report\n\nFindings: none was raised.",
    );
  });
});

describe("the header in words", () => {
  it("says a version by its day and time, and a goal by its own sentence", () => {
    expect(versionWhen("2026-10-02T12:19:37Z", "UTC")).toBe("2 October, 12:19");
    expect(versionWhen("2026-10-03T07:55:02Z", "Europe/Amsterdam")).toBe("3 October, 09:55");
    expect(versionWhen("", "UTC")).toBe("");
    expect(goalHeading("fleet-page-redesign", "What: The Fleet page is elegant. And more.")).toBe("The Fleet page is elegant.");
    expect(goalHeading("fleet-page-redesign", "")).toBe("fleet-page-redesign");
  });
});

describe("what a choice says on its card", () => {
  it("says its consequence before it is pressed, what was recorded once it is, and what follows if nothing is decided", () => {
    expect(choiceSaid("accepted", BLOCKS)).toBe(
      "Say why. Your acceptance and your reason are recorded on this review, in your name. " +
        "Accepting it on the goal itself is a separate step, at a terminal: metasystem goal accept-risk.",
    );
    expect(choiceSaid("accepted", WORTH)).toBe("Say why. Your acceptance and your reason are recorded on this review, in your name.");
    expect(choiceSaid("follow-up", { ...WORTH, answer: followUp("tests-for-failures"), recorded: true }))
      .toBe("Follow-up opened: tests-for-failures.");
    expect(choiceSaid("not a problem", { ...NOTE, answer: NOT_A_PROBLEM("history"), recorded: true })).toBe("Recorded: history");
    expect(choiceSaid("fix", { ...BLOCKS, answer: FIX, recorded: true }))
      .toBe("Recorded. The builder gets this as a correction when you send the goal back.");
    expect(choiceSaid("not a problem", NOTE)).toBe("If you decide nothing, it is recorded with the reviewer's reason: The design is history.");
  });
});

describe("the evidence's two presses", () => {
  it("open the change in the file a finding sits in, and the record it cites", () => {
    expect(seeing("internal/owner/owner.go:21-24")).toEqual({ change: { kind: "diff", path: "internal/owner/owner.go" }, cites: null });
    expect(seeing("plans/designs/reading.md § Part 2")).toEqual({
      change: { kind: "changes" }, cites: { kind: "section", record: "plans/designs/reading.md", section: "Part 2" },
    });
    expect(seeing("plans/designs/reading.md")).toEqual({
      change: { kind: "changes" }, cites: { kind: "source", path: "plans/designs/reading.md", from: 0, to: 0 },
    });
    expect(seeing("")).toEqual({ change: { kind: "changes" }, cites: null });
  });
});

describe("what you are looking at", () => {
  it("is the reviewer's own first paragraph, before its five parts", () => {
    expect(leadParagraph("The goal: one lock. This version adds a test.\n\n## Asked\n\nmore")).toBe("The goal: one lock. This version adds a test.");
    expect(leadParagraph("## Asked\n\nno lead")).toBe("");
    expect(leadParagraph("")).toBe("");
  });
});
