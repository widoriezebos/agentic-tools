import type { Outcome, Recorder } from "../partner/recording";

/**
 * What a drawing is, apart from how it is drawn (g1-s71 D2, D3).
 *
 * A drawing is a mermaid fence: in an answer, on the desk, or kept in the
 * record's Drawings section with the question that produced it. Everything here
 * is text in and text out, so it is read and tested without a browser; the
 * chunk that turns a fence into a picture is ./render.ts, loaded only when one
 * is on screen.
 */

/** The language a fence names to be drawn. */
export const MERMAID = "mermaid";

/** The heading kept drawings go under, created on the first keep. */
export const DRAWINGS = "Drawings";

/** The data attribute a style attribute travels under while markup is parsed (./render.ts). */
export const CARRIED_STYLE = "data-ms-style";

/** Whether a fence is one this build draws. */
export function drawable(lang: string): boolean {
  return lang.trim().toLowerCase() === MERMAID;
}

/**
 * Markup with every style attribute carried under CARRIED_STYLE and every
 * style element carrying the nonce, so that parsing it is refused nothing by
 * the page's policy. Only an attribute inside a tag is carried: text that says
 * style= is text, and markup escapes the angle brackets of text.
 */
export function carried(markup: string, nonce: string): string {
  return markup
    .replace(/(<[A-Za-z][^<>]*?\s)style=/gu, `$1${CARRIED_STYLE}=`)
    .replace(/<style(?=[\s>])/gu, `<style nonce="${nonce.replace(/[^A-Za-z0-9+/=_-]/gu, "")}"`);
}

/**
 * A drawing's identity: the turn it was answered in and its source. The same
 * answer read again after a reload names the same drawing, which is what lets
 * Keep it refuse a second keep.
 */
export function drawingId(turn: string, source: string): string {
  let hash = 0x811c9dc5;
  for (const character of `${turn}\n${source}`) {
    hash ^= character.codePointAt(0) ?? 0;
    hash = Math.imul(hash, 0x01000193) >>> 0;
  }
  return hash.toString(16).padStart(8, "0");
}

/** One kept drawing, as the record carries it. */
export type Kept = { id: string; caption: string; date: string; source: string };

const CAPTION = /^Asked (\d{4}-\d{2}-\d{2}): (.*) \[d:([0-9a-f]{8})\]$/u;

/** The caption line one kept drawing is written under. */
function captionLine(kept: Kept): string {
  const asked = kept.caption.replace(/\s+/gu, " ").trim();
  return `Asked ${kept.date}: ${asked === "" ? "the conversation" : asked} [d:${kept.id}]`;
}

/** A fence long enough that no line of the source closes it. */
function fenceFor(source: string): string {
  let longest = 2;
  for (const run of source.matchAll(/`+/gu)) {
    longest = Math.max(longest, run[0].length);
  }
  return "`".repeat(longest + 1);
}

/** The lines of a source with, for each, whether it lies inside a fence. */
function fenced(lines: readonly string[]): boolean[] {
  const inside: boolean[] = [];
  let open = "";
  for (const line of lines) {
    const fence = /^(`{3,}|~{3,})/u.exec(line.trim())?.[1] ?? "";
    if (open === "" && fence !== "") {
      open = fence;
      inside.push(true);
      continue;
    }
    if (open !== "" && fence.startsWith(open[0]) && fence.length >= open.length && line.trim() === fence) {
      open = "";
      inside.push(true);
      continue;
    }
    inside.push(open !== "");
  }
  return inside;
}

/** Where the Drawings section runs in these lines: its heading and the line after its last. */
function drawingsSection(lines: readonly string[]): { at: number; end: number } | null {
  const inside = fenced(lines);
  const at = lines.findIndex((line, index) => !inside[index] && line.trim() === `## ${DRAWINGS}`);
  if (at < 0) {
    return null;
  }
  let end = lines.length;
  for (let index = at + 1; index < lines.length; index += 1) {
    if (!inside[index] && /^#{1,2}\s+/u.test(lines[index])) {
      end = index;
      break;
    }
  }
  return { at, end };
}

/** Every drawing the record keeps, in the order it keeps them. */
export function drawingsIn(source: string): Kept[] {
  const lines = source.split("\n");
  const section = drawingsSection(lines);
  if (section === null) {
    return [];
  }
  const found: Kept[] = [];
  for (let index = section.at + 1; index < section.end; index += 1) {
    const caption = CAPTION.exec(lines[index].trim());
    if (caption === null) {
      continue;
    }
    let open = index + 1;
    while (open < section.end && lines[open].trim() === "") {
      open += 1;
    }
    const fence = /^(`{3,})\s*mermaid\s*$/u.exec(lines[open] ?? "")?.[1];
    if (fence === undefined) {
      continue;
    }
    const body: string[] = [];
    let close = open + 1;
    while (close < section.end && lines[close].trim() !== fence) {
      body.push(lines[close]);
      close += 1;
    }
    found.push({ id: caption[3], caption: caption[2], date: caption[1], source: body.join("\n") });
    index = close;
  }
  return found;
}

/** What Keep it answers when the record already keeps this drawing. */
export const ALREADY_KEPT = "The record already keeps this drawing.";

/**
 * The record with one drawing appended under its Drawings section, which is
 * created at the foot on the first keep; null where the record already keeps a
 * drawing of this id, so a reload that presses Keep it again writes nothing.
 */
export function keptDrawing(source: string, kept: Kept): string | null {
  if (drawingsIn(source).some((one) => one.id === kept.id)) {
    return null;
  }
  const fence = fenceFor(kept.source);
  const block = [captionLine(kept), "", `${fence}${MERMAID}`, ...kept.source.replace(/\s+$/u, "").split("\n"), fence];
  const lines = source.replace(/\s+$/u, "").split("\n");
  const section = drawingsSection(lines);
  if (section === null) {
    return `${[...lines, "", `## ${DRAWINGS}`, "", ...block].join("\n")}\n`;
  }
  let last = section.end;
  while (last > section.at + 1 && lines[last - 1].trim() === "") {
    last -= 1;
  }
  const after = lines.slice(section.end);
  const next = [...lines.slice(0, last), "", ...block, ...(after.length > 0 ? ["", ...after] : [])];
  return `${next.join("\n")}\n`;
}

/**
 * Keep it, through the sitting's recorder under its reading rule: the append is
 * composed from the record as it stands when the write runs, so a reload that
 * presses it again over a reread record writes nothing and is answered as kept.
 * `said` is "" when the record keeps the drawing, and the refusal otherwise.
 */
export async function keepDrawing(
  held: Pick<Recorder, "rewrite">,
  into: string,
  kept: Kept,
): Promise<{ said: string; outcome: Outcome }> {
  const outcome = await held.rewrite(into, (source) => keptDrawing(source, kept), ALREADY_KEPT);
  const keeps = outcome.kind === "recorded" || (outcome.kind === "failed" && outcome.reason === ALREADY_KEPT);
  return { said: keeps ? "" : outcome.reason, outcome };
}

/** What one drawing came to: drawn, or refused in the words the page says over its source. */
export type Drawn = { state: "drawn" } | { state: "refused"; said: string };

/** The chunk's one export, as the page loads it. */
export type Chunk = { renderDrawing: (source: string, into: Element, id: string) => Promise<void> };

/**
 * Draw one fence through the chunk the loader loads — when the drawing is on
 * screen and at no other time — or say why not: the chunk that did not load,
 * or the parse that failed, each in words over the source shown instead.
 */
export async function drawWith(load: () => Promise<Chunk>, source: string, into: Element, id: string): Promise<Drawn> {
  let chunk: Chunk;
  try {
    chunk = await load();
  } catch (error: unknown) {
    return { state: "refused", said: `The drawing could not be loaded (${reasonOf(error)}), so its source is shown instead.` };
  }
  try {
    await chunk.renderDrawing(source, into, id);
    return { state: "drawn" };
  } catch (error: unknown) {
    return { state: "refused", said: `The drawing could not be drawn: ${reasonOf(error).replace(/\.$/u, "")}. Its source is shown instead.` };
  }
}

function reasonOf(error: unknown): string {
  const said = error instanceof Error ? error.message : String(error);
  return said.split("\n")[0].trim() || "no reason was given";
}
