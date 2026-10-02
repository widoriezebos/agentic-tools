# The fleet panel: see what the machine is doing, and act on it

- Kind: design
- Id: 01M3ZA7QK4FLEETUX0000000001
- Status: draft
- Goals: fleet-panel-ux

## Part 1: Intent

Wido (2026-10-02): "the UX of the bottom half of the fleet panel is a mess. I need a proper well thought out UX design (first think what I want to be able to do in that screen, use cases, and then design an elegant, intuitive UX and build it)".

### Who looks, and when

One person, the owner of the fleet, glancing between other things: on the laptop, or on the phone in the pub. Most visits last seconds. They answer one of these questions; the screen is designed around them, in this order.

| # | The person wants to… | Today |
|---|---|---|
| J1 | know at a glance whether all is well | no verdict anywhere; health is spread over pills and notes |
| J2 | see what needs them, and deal with it on the spot | "Needs you" ignores the lane: a stopped lane, a returned branch, a red proof, a stuck seat |
| J3 | see what each seat is working on and how far along it is | a one-line text per seat with goal ids and stage codes, no title, no progress, no time |
| J4 | see what is landing: what waits, what is being proved, what landed, what came back and why | raw queue states, hashes, pids, attempt strings; landed work is a bare count |
| J5 | act: land now, pause or resume the lane, stop a seat, forget an unreachable seat, talk to an agent | one button (Land now); every other fix is a command to type |
| J6 | read what was delivered today, in plain words | nowhere on this screen |

### What "elegant" means here

1. One verdict on top, in words. Green when nothing needs the person.
2. Each fact said once. No repeated summary, no twin pills, no second list of the same machines.
3. Plain English by default: goal titles, "proving", "waiting", "came back because …". Hashes, pids, paths and log links live behind one "Details" disclosure per item.
4. The action sits next to the thing it acts on, as a button with a plain label. A person never copies a command; every button is also a verb (UI parity).
5. One word per state, used everywhere: Pause / Resume (never stop/start/started again in the same place).
6. It reads well on a phone: one column, cards that stack.

## Part 2: Design

Wido's screenshot (2026-10-02) showed: the table is liked; below it a stale lane block (the deleted batch lane, from a UI server running since Sep 30, restarted 2026-10-02), wrong "idle" in Running, and the stale "This host" list.

### Layout (the top keeps "This seat"; the table stays)

```
┌ All good · 5 seats working · 1 waiting to land · updated 12s ago ─┐   ← verdict strip (J1)
├ Needs you (only when something does) ────────────────────────────┤   ← (J2)
│  The landing lane is paused by m1e since 22:40.        [Resume]   │
│  "Plain lane landing" came back: the full test run is red. [Open] │
├ The fleet (the table Wido likes; Running becomes Doing) ─────────┤   ← (J3, J5)
│ MACHINE STANDING    SEEN      DOING                      HOLDS     │
│ m1f     reachable  1 min ago building · unit 2 of 5 · 42m  One folder…│
│ ui      reachable  now       reviewing · round 1 · 9 min   Work review…│
│ wr-m1   unreachable 3 d ago  —                [Forget]                 │
│   (expanded row: engine, generation, [Stop] [Talk])                   │
├ Landing lane  ● Running                     [Pause] [Land now]   ┤   ← (J4, J5)
│  Waiting   Seat path lands without help  · m1g · 4 min            │
│  Proving   Plain lane fix  · started 6 min ago (usually ~17 min) │
│  Landed today                                                    │
│   21:54  Stuck agents now ask you on Telegram and wait.          │   ← (J6) the delivered sentence
│   18:54  The fleet card can land work now.                       │
│  Came back                                                       │
│   lane-check-red · the full test run is red  [Open goal]         │
└──────────────────────────────────────────────────────────────────┘
```

### The parts

1. **Verdict strip.** One sentence from one rule: "All good" when Needs you is empty; else "N things need you". Then counts (seats working, waiting to land) and "updated …". Colour only reinforces the words.
2. **Needs you.** Collected from every source in one list, newest first, each with its one action:
   - lane paused or unready → [Resume] or the plain reason;
   - a branch returned → [Open goal];
   - a red proof → [Open log];
   - a seat silent past its threshold (the steward's seat-idle signal) → [Talk] / [Stop];
   - an open question for the person → [Answer] (link to where it is answered).
   Empty means the section is not shown at all.
3. **Seats: the fleet table, kept and completed.** Wido likes the table (screenshot 2026-10-02); it stays the one place for machines, and the "This host" text list below it is removed (it repeats the table in worse words and shows stale cards, e.g. "m1g: one-folder-deployed-and-evolved unknown: claim moved to m1f"). Changes to the table:
   - the Running column becomes **Doing**, filled from the board and the seat records: "building · unit 2 of 5 · 12 min", "reviewing · round 3 of 20", "waiting to land", "idle". Today it says "idle" for seats that are working (m1f in revise round 3, ui building): a data bug, fixed here;
   - Holds shows the goal's title, with the id in the link;
   - Engine collapses to the short version; the generation number moves into the expanded row;
   - per-row actions in the expanded row: [Stop] for a running seat; [Talk] when ui-connects-to-a-running-agent lands; [Forget] for an unreachable seat (e.g. wr-m1, 3 days) when fleet-forgets-an-unreachable-seat lands;
   - the board's stale cards are dropped: a card for a goal the machine no longer holds is not shown.
4. **Landing lane.** One state word (Running, Paused, Needs attention) with the one or two buttons that make sense in that state ([Pause] or [Resume]; [Land now] only when work waits and nothing proves). Then a small pipeline: Waiting (title · seat · age), Proving (title · elapsed · usual duration), Landed today (time · the delivered sentence, else the goal title), Came back (title · reason in words · [Open goal]). Each item has a Details disclosure with the commit, tree, log link, pid and root, for when the person wants them.
5. **Freshness.** The panel re-reads with the rest of the page, shows a loading skeleton instead of nothing, and a failed read of one section says so in that section only.

### Data: nothing new to invent

All facts exist: plain lane status (queue, running_proof, last_proof, last_push; internal/landing/plain/status.go), board seat views (internal/board/view.go), steward seat records and signals, questions (`question list`), goal titles (ledger). The work is presentation plus three small server additions: goal titles in the board and lane payload, the delivered sentence per landed entry (slice C stores it in the queue line), and endpoints for Pause/Resume/Stop that call the existing verbs (`landing stop/start`, `work stop`).

### Step 1 and Deferred

Step 1: the verdict strip, Needs you (lane, returned, red proof, open questions), Seats cards with Stop, the lane block with Pause/Resume/Land now and the four lists, Details disclosures, one wording, phone layout. Deferred: [Talk] (goal ui-connects-to-a-running-agent), [Forget] (goal fleet-forgets-an-unreachable-seat) — the cards reserve their place; progress bars where the board has no N of M yet.

### Threat model and rabbit-hole risks

Threat model: the template default (our own agents and operators make mistakes; nobody attacks). Rabbit holes: a dashboard framework or charts (no: words and one card per thing); live-streaming everything (the page's existing refresh is enough); re-inventing the fleet model (present what exists); configurable layouts (one good layout).
