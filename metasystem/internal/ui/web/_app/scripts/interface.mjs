import { writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

/**
 * The interface's own half of the manifest, composed at bundle time from the
 * sources the pages render from.
 *
 * Nothing here is written for the Project Partner. The sections come from the
 * route table that decides what is rendered, their purposes from the help
 * register the help control reads, their availability from the empty register
 * the unprojected panes render, the lanes from the lane table the board reads,
 * and the suggested questions from the register the composer's chips read. Add
 * a section, reword a term, show a lane, and this file says so on the next
 * build without anybody editing a second account.
 *
 * Three statements about one thing are kept apart everywhere, because the
 * first live reading of this interface confused them: what a section is FOR,
 * whether this build HAS it, and what the Partner itself may DO about it. A
 * manifest that ran them together would teach a Partner to send a human to a
 * page that is a placeholder, or to offer an edit it cannot make.
 *
 * It runs under Node's own TypeScript support, so it imports the interface's
 * modules as they are rather than a copy of them.
 */

/** The shape this file writes and internal/ui/manifest reads. */
export const SCHEMA_VERSION = 1;

/** Where the built manifest sits inside the bundle. */
export const INTERFACE_FILE = "interface.json";

/**
 * A source with nothing to say about itself is a refusal rather than an empty
 * string in the answer: a Partner told that a section exists and nothing else
 * would describe it from its name, which is the guessing this whole manifest
 * exists to stop.
 */
class MissingDescription extends Error {
  constructor(what) {
    super(`${what} has no description; write one where it belongs before building the bundle`);
    this.name = "MissingDescription";
  }
}

function described(what, text) {
  if (typeof text !== "string" || text.trim() === "") {
    throw new MissingDescription(what);
  }
  return text;
}

/**
 * The shape, named so a reader of the manifest and a reader of this file agree
 * about it. internal/ui/manifest declares the same one in Go.
 *
 * @typedef {{ id: string, title: string, path: string, purpose: string, shows: string, projected: boolean, availability: string }} ManifestSection
 * @typedef {{ id: string, title: string, purpose: string, shown: boolean }} ManifestLane
 * @typedef {{ id: string, term: string, text: string }} ManifestTerm
 * @typedef {{ text: string, scope: string }} ManifestQuestion
 * @typedef {{ subject: string, questions: ManifestQuestion[] }} ManifestRegister
 * @typedef {{ schemaVersion: number, sections: ManifestSection[], lanes: ManifestLane[], terms: ManifestTerm[], questions: ManifestRegister[] }} InterfaceManifest
 */

/**
 * Composes the manifest from the interface's own registers.
 *
 * Every input is a parameter so that the guard can drive it with a register
 * that is missing a sentence; the caller below passes the real ones.
 *
 * @returns {InterfaceManifest}
 */
export function buildInterface(sources) {
  const { sections, lanes, shownLanes, help, sectionTerm, empties, suggestions } = sources;

  const shown = new Set(shownLanes.map((lane) => lane.id));
  const emptyFor = new Map(empties.map((empty) => [empty.id, empty]));

  return {
    schemaVersion: SCHEMA_VERSION,
    sections: sections.map((section) => {
      const term = sectionTerm(section.id);
      const purpose = described(`the section ${section.id}`, term === null ? "" : help[term]?.text);
      const empty = emptyFor.get(section.id);
      if (!section.projected && empty === undefined) {
        throw new MissingDescription(`the unprojected section ${section.id}`);
      }
      return {
        id: section.id,
        title: section.title,
        path: section.path,
        purpose,
        shows: described(`what the section ${section.id} shows`, section.shows),
        projected: section.projected,
        availability: section.projected
          ? `This build projects ${section.title}: the page reads the workspace and says what it read.`
          : `This build does not project ${section.title}. ${described(`the empty state for ${section.id}`, empty.body)} ${empty.note}.`,
      };
    }),
    lanes: lanes.map((lane) => {
      const term = lane.help;
      return {
        id: lane.id,
        title: lane.title,
        purpose: described(`the lane ${lane.id}`, term === null ? "" : help[term]?.text),
        shown: shown.has(lane.id),
      };
    }),
    terms: Object.entries(help).map(([id, term]) => ({
      id,
      term: term.term,
      text: described(`the help term ${id}`, term.text),
    })),
    questions: suggestions.map((register) => {
      if (register.questions.length === 0) {
        throw new MissingDescription(`the suggested questions for ${register.subject}`);
      }
      return {
        subject: register.subject,
        questions: register.questions.map((question) => ({
          text: question.text,
          scope: described(`the scope of "${question.text}"`, question.scope),
        })),
      };
    }),
  };
}

/** The interface's real registers, read from the modules the pages import. */
export async function readSources(appDir) {
  const routes = await import(path.join(appDir, "src", "routes.ts"));
  const terms = await import(path.join(appDir, "src", "help", "terms.ts"));
  const lanes = await import(path.join(appDir, "src", "backlog", "lanes.ts"));
  const empties = await import(path.join(appDir, "src", "panes", "empties.ts"));
  const suggestions = await import(path.join(appDir, "src", "partner", "suggestions.ts"));
  return {
    sections: [...routes.sections],
    lanes: [...lanes.lanes],
    shownLanes: [...lanes.shownLanes],
    help: terms.HELP,
    sectionTerm: terms.sectionTerm,
    empties: [...empties.empties],
    suggestions: suggestions.registers().map((subject) => ({
      subject,
      questions: [...suggestions.suggestionsFor(subject)],
    })),
  };
}

/** Composes the manifest and writes it into the built bundle. */
export async function writeInterface(appDir, distDir) {
  const manifest = buildInterface(await readSources(appDir));
  const file = path.join(distDir, INTERFACE_FILE);
  writeFileSync(file, `${JSON.stringify(manifest, null, 2)}\n`);
  return { file, manifest };
}

const APP_DIR = path.resolve(fileURLToPath(import.meta.url), "..", "..");

const invokedDirectly = process.argv[1] !== undefined && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url);
if (invokedDirectly) {
  const { file } = await writeInterface(APP_DIR, path.join(APP_DIR, "..", "bundle", "dist"));
  process.stdout.write(`wrote ${file}\n`);
}
