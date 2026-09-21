import { describe, expect, it } from "vitest";

import { empties, emptyFor, type Empty } from "./empties";
import { sections } from "../routes";

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
    id: "overview",
    kind: "not projected",
    heading: "Overview is not projected yet",
    body: "This build does not read the records. Overview will show what needs you and what changed since your last visit, linked to its records.",
    note: "Arrives with g1-s14",
    link: { label: "About this workspace", to: "/settings" },
    action: null,
  },
  {
    id: "project",
    kind: "not projected",
    heading: "Project is not projected yet",
    body: "Intent, architecture, designs, constraints and assurance, open questions, and sittings are in the repository and will be read here.",
    note: "Arrives with gate 5",
    link: null,
    action: null,
  },
  {
    id: "backlog",
    kind: "not projected",
    heading: "The backlog is not projected yet",
    body: "Goals exist at the accepted tip. Board, outline, dependencies, list, and one workspace per goal will read them here.",
    note: "Arrives with g1-s10, the detail view with g1-s11",
    link: null,
    action: null,
  },
  {
    id: "fleet",
    kind: "not projected",
    heading: "Fleet is not projected yet",
    body: "This machine's sessions, jobs, census, and health are recorded and will be shown with observation times and gaps.",
    note: "Arrives with g1-s13",
    link: null,
    action: null,
  },
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
    body: "Runtimes and models, connections and channels, identity and authority, execution defaults, storage and retention.",
    note: "Arrives with gate 7",
    link: null,
    action: null,
  },
  {
    id: "brain",
    kind: "not projected",
    heading: "The Brain is not connected in this build",
    body: "Conversation, shared context, activity, and actions.",
    note: "Arrives with gate 3",
    link: null,
    action: null,
  },
  {
    id: "subject",
    kind: "not projected",
    heading: "No subject selected",
    body: "The subject panel will show the artifact under discussion.",
    note: "Arrives with gate 3",
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
    const wanted = [...sections.map((section) => section.id), "subject", "not-found"].sort();
    expect(covered).toEqual(wanted);
    for (const section of sections) {
      expect(emptyFor(section.id).id).toBe(section.id);
    }
  });

  it("are the not-projected kind for every section, with no action of its own", () => {
    for (const section of sections) {
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

  it("offer one link, from Overview, to a surface this build serves", () => {
    const linked = empties.filter((empty) => empty.link !== null);
    expect(linked.map((empty) => empty.id)).toEqual(["overview"]);
    expect(linked[0].link).toEqual({ label: "About this workspace", to: "/settings" });
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
