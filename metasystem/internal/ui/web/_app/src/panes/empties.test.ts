import { describe, expect, it } from "vitest";

import { empties, emptyFor, type Empty } from "./empties";
import { projectedSections, unprojectedSections } from "../routes";

/**
 * What every pane says, word for word.
 *
 * The table below is the design's, written out again here so that a change to
 * the words is a change a reviewer sees in a diff rather than a sentence that
 * quietly drifts. The rules after it are the ones that hold whatever the words
 * are: every section is the not-projected kind, none of them offers an action,
 * only Overview offers a link, and no pane claims that anything is absent.
 */

const expected: Empty[] = [
  {
    id: "decisions",
    kind: "not projected",
    heading: "Decisions are not projected yet",
    body: "Questions from every seat, approvals, delegations, and rulings are in the ledger and will be read here.",
    note: "Arrives with g1-s12",
    link: null,
    action: null,
  },
  {
    id: "application",
    kind: "not projected",
    heading: "Application is not projected yet",
    body: "Implemented and released behaviour and its evidence are recorded and will be shown here.",
    note: "Arrives with gate 6",
    link: null,
    action: null,
  },
  {
    id: "settings",
    kind: "not projected",
    heading: "Settings pages are not built yet",
    body: "Pages for runtimes and models, connections and channels, identity and authority, execution defaults, and storage and retention will be read and changed here.",
    note: "Arrives with gate 7",
    link: null,
    action: null,
  },
  {
    id: "not-found",
    kind: "not a section",
    heading: "No page at this address",
    body: "The address matches no section of this workspace.",
    note: "Check the address in the address bar",
    link: null,
    action: { label: "Go to Overview", to: "/overview" },
  },
];

/**
 * Wording that asserts absence. This build reads no records, so no pane may
 * say that there are none: there are 156 live goals at the accepted tip, and a
 * pane that says otherwise is a lie a human would act on.
 */
/**
 * The sections this build projects from the server's answer. They have no
 * empty state, because their panes say what they read rather than what has not
 * been built; every other section still has a row above.
 *
 * Which ones they are is read from the section table rather than written here
 * again: the table is what routes a section to its pane, so a section that
 * starts projecting stops needing an empty row in the same edit.
 */
const PROJECTED = projectedSections.map((section) => section.id);

const unprojected = unprojectedSections;

const ABSENCE = [
  "nothing",
  "no goals",
  "no seats",
  "no decisions",
  "no records",
  "no sessions",
  "is empty",
  "are empty",
  "does not exist",
  "do not exist",
  "none yet",
];

describe("the empty states", () => {
  it("say what the design says, word for word", () => {
    expect(empties).toEqual(expected);
  });

  it("cover every destination that has a pane, and nothing more", () => {
    const covered = empties.map((empty) => empty.id).sort();
    const wanted = [...unprojected.map((section) => section.id), "not-found"].sort();
    expect(covered).toEqual(wanted);
    for (const section of unprojected) {
      expect(emptyFor(section.id).id).toBe(section.id);
    }
  });

  // A projected section reads the repository or the ledger, so it has no empty
  // state of its own: its pane says what it read, and an empty part of it says
  // so in one line rather than in a card.
  it("leave a projected section to its own pane", () => {
    for (const id of PROJECTED) {
      expect(empties.map((empty) => empty.id)).not.toContain(id);
      expect(() => emptyFor(id)).toThrow();
    }
  });

  it("are the not-projected kind for every section, with no action of its own", () => {
    for (const section of unprojected) {
      const empty = emptyFor(section.id);
      expect({ id: empty.id, kind: empty.kind }).toEqual({ id: empty.id, kind: "not projected" });
      expect({ id: empty.id, action: empty.action }).toEqual({ id: empty.id, action: null });
    }
  });

  it("name the slice or gate that brings each view", () => {
    for (const empty of empties) {
      if (empty.kind === "not projected") {
        expect({ id: empty.id, note: empty.note.startsWith("Arrives with ") }).toEqual({ id: empty.id, note: true });
      }
    }
  });

  // Overview carried the one link, to About this workspace, because it could
  // say nothing of its own. It reads the records now, so no row offers one.
  it("offer no link, now that the one section that did reads its own records", () => {
    expect(empties.filter((empty) => empty.link !== null)).toEqual([]);
  });

  it("offer one action, on the pane that is not a section", () => {
    const acting = empties.filter((empty) => empty.action !== null);
    expect(acting.map((empty) => empty.id)).toEqual(["not-found"]);
    expect(acting[0].kind).toBe("not a section");
  });

  it("never offer a link and an action at once", () => {
    for (const empty of empties) {
      expect({ id: empty.id, both: empty.link !== null && empty.action !== null }).toEqual({ id: empty.id, both: false });
    }
  });

  it("claim nothing about records this build does not read", () => {
    const offenders: string[] = [];
    for (const empty of empties) {
      const text = `${empty.heading} ${empty.body} ${empty.note}`.toLowerCase();
      for (const phrase of ABSENCE) {
        if (text.includes(phrase)) {
          offenders.push(`${empty.id}: ${phrase}`);
        }
      }
    }
    expect(offenders).toEqual([]);
  });

  it("end a body as a sentence and a heading as a name", () => {
    for (const empty of empties) {
      expect({ id: empty.id, body: empty.body.endsWith(".") }).toEqual({ id: empty.id, body: true });
      expect({ id: empty.id, heading: empty.heading.endsWith(".") }).toEqual({ id: empty.id, heading: false });
    }
  });
});
