/**
 * The loop from the room (g1-s66 §3, D1 to D4): a design's critique, sent,
 * read and answered from the design's own page.
 *
 * Three requests, each made when a human presses something or opens the page,
 * and at no other time: the chain as it stands, Send to critique or Answer the
 * round, and one decision. Nothing here polls and nothing here sets a timer; the
 * bell says a round is back, and the page is read again when the human comes to
 * it or presses Refresh.
 *
 * The rules below are the engine's, said where the presses are. A finding card
 * is read from its round's own return, and its identity is the round and the id,
 * so a recurring id in round 2 is a new card. Each press writes one row, in the
 * engine's four values: Fold writes accepted with the fold's amendment, Refute
 * writes refuted with the reason, Defer writes noted for a finding that is not
 * material and out-of-scope, with the evidence, for one that is. Nothing a press
 * writes goes into the design, so the design's digest stays the reviewed one
 * until a fold changes it. Answer the round runs the verb, and the page shows
 * what the engine did in its words; it never computes an exit of its own.
 */

import { ResourceError } from "./api";

const DESIGN = "/api/design/";
const REVIEW = "/review";

/** One finding, as its round's return carries it. */
export type DesignFinding = {
  id: string;
  severity: string;
  material: boolean;
  claim: string;
  evidence: string;
  /** The change it asks for and its tests, only where its own words carry them. */
  change?: string;
  tests?: string;
};

/** One decided row of a round's decisions file. */
export type DesignRow = { finding: string; disposition: string; reasoning: string; amendment: string };

export type DesignRound = {
  round: number;
  findings: DesignFinding[];
  /** The return as written, where it carries no findings this page can read. */
  prose?: string;
  decisions: DesignRow[];
  answerable: boolean;
};

export type DesignLoop = {
  design: string;
  toolCalls: number;
  chain: string;
  goal?: string;
  state: "none" | "reading" | "deciding" | "answered" | "closed" | "ended";
  status?: string;
  round: number;
  limit: number;
  critic?: string;
  rounds: DesignRound[];
  chains?: number;
};

/** What the engine did, in its own words. */
export type DesignAnswer = { outcome: string; summary: string; decision?: string; lines?: string[]; chain?: string };

/**
 * A refusal the server explained, with its code, and whether the remedy is to
 * sign in here: the act opens the sign-in sheet and is made once more.
 */
export class CritiqueError extends ResourceError {
  readonly code: string;
  readonly signIn: boolean;

  constructor(resource: string, status: number, reason: string, code: string, signIn: boolean) {
    super(resource, status, reason);
    this.name = "CritiqueError";
    this.code = code;
    this.signIn = signIn;
  }
}

function base(design: string): string {
  return DESIGN + design.split("/").map((segment) => encodeURIComponent(segment)).join("/") + REVIEW;
}

async function request<T>(resource: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  const sending = body !== undefined;
  const response = await fetch(resource, {
    signal,
    method: sending ? "POST" : "GET",
    headers: sending ? { Accept: "application/json", "Content-Type": "application/json" } : { Accept: "application/json" },
    body: sending ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) {
    let refusal: { error?: string; code?: string; signIn?: boolean } = {};
    try {
      refusal = (await response.json()) as typeof refusal;
    } catch {
      refusal = {};
    }
    throw new CritiqueError(resource, response.status, refusal.error ?? "", refusal.code ?? "", refusal.signIn === true);
  }
  return (await response.json()) as T;
}

/** The design's critique as it stands. */
export async function loadCritique(design: string, signal?: AbortSignal): Promise<DesignLoop> {
  return request<DesignLoop>(base(design), undefined, signal);
}

/** Send to critique, or Answer the round when `after` names the round answered. */
export async function sendToCritique(design: string, asked: { goal: string; toolCalls: number; after?: number }): Promise<DesignAnswer> {
  return request<DesignAnswer>(base(design), { goal: asked.goal, toolCalls: asked.toolCalls, after: asked.after ?? 0 });
}

/** One row of one round's decisions file. */
export async function decideFinding(design: string, round: number, row: DesignRow): Promise<DesignLoop> {
  return request<DesignLoop>(`${base(design)}/${String(round)}/decisions`, row);
}

/* ------------------------------------------------------------ the words -- */

export const SEND = "Send to critique";
export const ANSWER = "Answer the round";
export const FOLD = "Fold";
export const REFUTE = "Refute";
export const DEFER = "Defer";
export const USE = "Use";
export const NOT_THIS = "Not this";
export const OPEN_FROM = "Open a goal from this design";

/** The address a sitting's End sheet sends a human to, to open that goal (D5). */
export const OPEN_GOAL_PARAM = "open-goal";

/** The design's page, asking for the New goal sheet once it has read. */
export function openGoalPath(documentPath: string): string {
  return `${documentPath}?${OPEN_GOAL_PARAM}=1`;
}

/** A goal may fund a critique once it is approved; a claimed goal still is. */
const FUNDABLE = new Set(["approved", "claimed"]);

/**
 * Which goal funds the critique: preselected when the design's Goals line names
 * one approved goal, chosen by the human when it names several, and none where
 * it names no approved goal at all — the engine's refusal then says so.
 */
export function fundingGoal(goals: readonly string[], states: Readonly<Record<string, string>>): { chosen: string; choices: string[] } {
  const choices = goals.filter((goal) => FUNDABLE.has(states[goal] ?? ""));
  return { chosen: choices.length === 1 ? choices[0] : "", choices };
}

/** The page's one line about the chain. */
export function stateLine(loop: DesignLoop): string {
  if ((loop.chains ?? 0) > 1) {
    return `This design has ${String(loop.chains)} critique chains; the engine chooses none of them, and neither does this page`;
  }
  if (loop.chain === "") {
    return "";
  }
  const at = `Critique · round ${String(loop.round)} of ${String(loop.limit)}`;
  const newest = loop.rounds[loop.rounds.length - 1];
  const waiting = newest === undefined ? 0 : newest.findings.filter((finding) => !newest.decisions.some((row) => row.finding === finding.id)).length;
  switch (loop.state) {
    case "reading":
      return `${at} · ${loop.critic === undefined || loop.critic === "" ? "the critic" : loop.critic} reading`;
    case "deciding":
      return waiting === 1 ? `${at} · 1 finding waits for your decision` : `${at} · ${String(waiting)} findings wait for your decisions`;
    case "answered":
      return `${at} · every finding is decided`;
    case "closed":
      return `Critique · closed at round ${String(loop.round)}`;
    case "ended":
      return `${at} · the examination ended ${loop.status ?? ""} without findings to decide`;
    default:
      return at;
  }
}

/** What a press is about to write, before the reason and the amendment are in. */
export type Press = "fold" | "refute" | "defer";
export type Draft = { reasoning: string; amendment: string };

/** Why a press cannot be written yet, in words, or "". */
export function pressRefusal(press: Press, finding: DesignFinding, draft: Draft): string {
  if (press === "refute" && draft.reasoning.trim() === "") {
    return "A refutation carries your reason: the check you made and what it showed.";
  }
  if (press === "defer" && finding.material && draft.reasoning.trim() === "") {
    return "A material finding is deferred only as out of scope, with the evidence that it is outside the brief.";
  }
  if (press === "fold" && draft.amendment.trim() === "") {
    return "A fold carries its one-line amendment.";
  }
  return "";
}

/** The row one press writes, in the engine's own four values. */
export function rowFor(press: Press, finding: DesignFinding, draft: Draft): DesignRow {
  const disposition = press === "fold" ? "accepted" : press === "refute" ? "refuted" : finding.material ? "out-of-scope" : "noted";
  return {
    finding: finding.id,
    disposition,
    reasoning: draft.reasoning.trim(),
    amendment: press === "fold" ? draft.amendment.trim() : "",
  };
}

/** One card: a finding of one round, and the row the file holds for it. */
export type Card = { round: number; finding: DesignFinding; row: DesignRow | undefined };

/** The round's cards, each with the row a reload finds in the file. */
export function cardsOf(round: DesignRound): Card[] {
  return round.findings.map((finding) => ({
    round: round.round,
    finding,
    row: round.decisions.find((row) => row.finding === finding.id),
  }));
}

/** Answer the round stands when the engine's own join says every card has its row. */
export function answerable(loop: DesignLoop): boolean {
  const newest = loop.rounds[loop.rounds.length - 1];
  return loop.state === "answered" && newest !== undefined && newest.answerable;
}

/**
 * The heading a finding names, where its claim or its evidence names one of the
 * document's headings in full; "" where it names none and the human chooses.
 */
export function sectionFor(finding: DesignFinding, headings: readonly string[]): string {
  const said = `${finding.claim}\n${finding.evidence}\n${finding.change ?? ""}`;
  return headings.find((heading) => heading.length > 3 && said.includes(heading)) ?? "";
}

/** What Fold asks the Partner: that section anew, with the finding folded in. */
export function foldAsk(design: string, heading: string, finding: DesignFinding, round: number): string {
  return [
    `Fold finding ${finding.id} of round ${String(round)} into the section “${heading}” of ${design}.`,
    `The finding (${finding.material ? "material" : "not material"}, ${finding.severity}): ${finding.claim}`,
    finding.evidence === "" ? "" : `Its evidence: ${finding.evidence}`,
    finding.change === undefined || finding.change === "" ? "" : `The change it asks for: ${finding.change}`,
    "Draft that one section anew, heading line and all, and offer it with suggest with document and section; change nothing else.",
  ].filter((line) => line !== "").join("\n");
}

/** The engine's words, as said: the sentence, what it says is next, and its lines. */
export function answerWords(answer: DesignAnswer): string[] {
  return [answer.summary, answer.decision ?? "", ...(answer.lines ?? [])].filter((line) => line.trim() !== "");
}
