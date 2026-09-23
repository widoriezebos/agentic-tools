import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { buildInterface, readSources, SCHEMA_VERSION } from "./interface.mjs";

/**
 * The manifest is composed from the interface's own registers, so what is
 * asserted here is that it says what they say and refuses what they leave
 * unsaid. The words themselves are asserted where they are written: the help
 * register's guard, the empty register's guard, the lane table's guard.
 */

const APP_DIR = path.resolve(fileURLToPath(import.meta.url), "..", "..");

async function real() {
  return buildInterface(await readSources(APP_DIR));
}

describe("the interface manifest", () => {
  it("carries every section of the rail, in rail order", async () => {
    const manifest = await real();
    expect(manifest.schemaVersion).toBe(SCHEMA_VERSION);
    expect(manifest.sections.map((section) => section.id)).toEqual([
      "brain",
      "overview",
      "project",
      "backlog",
      "fleet",
      "decisions",
      "application",
      "settings",
    ]);
  });

  // The whole point of the availability field. A Partner asked where to
  // inspect the fleet must say that this build has no Fleet page, and it can
  // only say that if purpose, availability and what it may itself do are
  // three fields rather than one paragraph.
  it("tells a section's purpose apart from whether this build has it", async () => {
    const manifest = await real();
    const fleet = manifest.sections.find((section) => section.id === "fleet");
    expect(fleet?.projected).toBe(false);
    expect(fleet?.availability).toContain("This build does not project Fleet");
    expect(fleet?.availability).toContain("Arrives with g1-s13");
    expect(fleet?.purpose).toContain("machines and seats");

    const backlog = manifest.sections.find((section) => section.id === "backlog");
    expect(backlog?.projected).toBe(true);
    expect(backlog?.availability).toContain("This build projects Backlog");
  });

  it("carries the lanes in the order work moves through them, and says which are shown", async () => {
    const manifest = await real();
    expect(manifest.lanes.map((lane) => lane.id)).toEqual([
      "draft",
      "to-do",
      "ready",
      "in-progress",
      "review",
      "waiting",
      "done",
      "abandoned",
      "unknown",
    ]);
    const shown = manifest.lanes.filter((lane) => lane.shown).map((lane) => lane.id);
    expect(shown).toEqual(["to-do", "ready", "in-progress", "review", "waiting"]);
    for (const lane of manifest.lanes) {
      expect({ id: lane.id, said: lane.purpose !== "" }).toEqual({ id: lane.id, said: true });
    }
  });

  it("carries the help register and the suggested questions", async () => {
    const manifest = await real();
    const partner = manifest.terms.find((term) => term.id === "partner");
    expect(partner?.text).toContain("does not write");
    expect(partner?.text).not.toContain("write with you");
    const goal = manifest.questions.find((register) => register.subject === "goal");
    expect(goal?.questions.map((question) => question.text)).toEqual([
      "Why is it here?",
      "What would move it?",
      "What does its design say?",
    ]);
    for (const register of manifest.questions) {
      for (const question of register.questions) {
        expect({ text: question.text, scoped: question.scope !== "" }).toEqual({ text: question.text, scoped: true });
      }
    }
  });
});

describe("a source with nothing to say about itself", () => {
  async function sources() {
    const read = await readSources(APP_DIR);
    return { ...read, sections: read.sections.map((section) => ({ ...section })) };
  }

  it("refuses a section with no sentence about what it shows", async () => {
    const read = await sources();
    read.sections[1].shows = "";
    expect(() => buildInterface(read)).toThrow(/what the section overview shows has no description/);
  });

  it("refuses a section the help register does not explain", async () => {
    const read = await sources();
    expect(() => buildInterface({ ...read, sectionTerm: () => null })).toThrow(
      /the section brain has no description/,
    );
  });

  it("refuses an unprojected section with no empty state", async () => {
    const read = await sources();
    expect(() => buildInterface({ ...read, empties: [] })).toThrow(
      /the unprojected section fleet has no description/,
    );
  });

  it("refuses a lane with no sentence", async () => {
    const read = await sources();
    const help = { ...read.help, "lane-ready": { term: "Ready for Work", text: "" } };
    expect(() => buildInterface({ ...read, help })).toThrow(/the lane ready has no description/);
  });

  it("refuses a subject whose suggested questions are empty", async () => {
    const read = await sources();
    const suggestions = [{ subject: "goal", questions: [] }];
    expect(() => buildInterface({ ...read, suggestions })).toThrow(
      /the suggested questions for goal has no description/,
    );
  });
});
