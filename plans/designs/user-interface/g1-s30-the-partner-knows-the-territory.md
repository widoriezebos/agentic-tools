# g1-s30 The Partner knows the territory

- Kind: design
- Id: 01M37RK7DZ74773VR9DBFZ838A
- Status: draft
- Cites: 01M34HS374KF1RSS3EWKBGD2WE

Revision 1, 2026-09-23, Claude on Fable, on Wido's question "Is the project partner an expert on both the metasystem and also the UI? If not; what supporting system for the agent do we need to get in place to make this possible and easy to maintain/extend?" and his ruling "write the design, get it critiqued by astra and then build it". Draft until Astra has read it. Builds on g1-s28 (the Partner reads and explains) and g1-s29 (it is told what the human sees and can look further).

## Outcome

The Partner answers questions about the interface and about the metasystem as an expert would: it knows what every section, act, lane, setting and record kind is for in this build, it knows what the project's memory holds and where, and it goes to the right source for a question instead of guessing. Every piece of that knowledge lives beside the thing it describes and is generated or listed from it, so adding a section, a term, a record or a skill teaches the Partner without anyone writing a second account.

## The principle

Knowledge is kept where it is true. The interface describes itself from its own sources; the project's memory is listed by its own resolver; the way to answer is a skill in the kit like every other. Nothing is copied into a manual for the Partner.

## Step 1

1. **The interface describes itself.** A manifest, `interface.json`, generated at bundle time from the sources the pages render from: the sections (id, title, the help text, what the page shows), the acts (name, what it does, what it needs: a signed-in human, a proven seat), the lanes (title, help text, order), the record kinds and their homes, the settings under `ui.` with their meaning and defaults, the suggested questions and the help terms. The generator refuses to build when a source has no description, so the manifest cannot be incomplete. The server serves it at `GET /api/interface`; the tool server exposes it as `interface()`; the bundle's digest covers it.
2. **A map of the project's memory at session start.** When a Partner session opens, the standing rule carries the index the resolver already produces: the intent book's chapters, the doctrine book's chapters, the decisions, the designs and the open questions, each as one line with its id, title, status and summary, bounded at 300 lines and 12,000 characters with a line naming what was left out; the read tools fetch any of them. The index is composed at session open and again when a fresh session is opened after a loss.
3. **A Partner skill in the kit.** `skills/project-partner/SKILL.md`, routed from `wow.md` like every other skill, in two parts. *Where to look*: a question about a term → the help register in the manifest; about what a page or act does → the manifest; about why the project exists → the intent book; about a rule the code is held to → the doctrine book; about a choice already made → the decisions; about how something will be built → the designs; about a goal, a lane or the board → the ledger tools; about what is on screen → the snapshot it was given, first. *How to answer*: in plain words, in the interface's own vocabulary, citing the source and the tip or revision, saying what it could not see, and never claiming an act it cannot do. The skill's text is appended to the standing rule at session open, after the index.
4. **A small answer check.** `internal/ui/partner/answers_test.go` holds a dozen questions with the facts a right answer must contain, run against the fake runtime with a scripted answer that is checked for those facts, so the composition (manifest, index, skill) is proven to carry them; and one script under the walkthrough that asks a real runtime the same dozen and prints which facts each answer carried, run by hand before a release, never in a test.

## Later, when it hurts

- Knowledge beyond the records: an adopted application's own domain, taught through its docs books.
- Usage and cost per turn on the page; the check run on a cadence by the steward.
- Answers that cite with links into the interface for every source, once the interface has pages for skills and settings.
