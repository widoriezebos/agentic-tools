# metasystem stop: closing code critique (chain stopverb-build1, round 18)

Critic stopverb-crit5 (code-critic, Opus 5, read-only) on reviewed tree
abc603784e4cdc59162095b9f8e8ed1244847688, the tree whose whole
acceptance was green in the orchestrator's environment: five fixture
beds, all eight slice-1 scenarios, and the eleven-package matrix. Three
material findings and three notes. The second read of this chain; the
first is records/misc/metasystem-stop-critique-r4.md.

## SVC5-01 — high — the record contradicts the page

CLAIM: a stop that cannot read one family's records prints `stop
incomplete` and exits 1, but writes a durable record saying the checkout
is fully stopped with an empty survivor list, because the unreadable
family produced no item to attach a survivor to. The next arm then opens
the fence without probing anything, and whatever that family was hiding,
live delegate jobs or monitored runs, is running again under an open
fence.

DISPOSITION: accepted, and the most serious finding of either review.
Wido's requirement is that after a stop nothing comes back on its own;
this is the one path where the record itself grants permission. D69.

## SVC5-02 — medium — arm's unprovable-creator refusal loops

CLAIM: when arm refuses because a creation claim's creator can be
neither proven alive nor proven dead, its second line says to run arm
again, which re-probes the same unreadable identity and refuses
identically. The command that clears the state, a second stop, is never
named.

DISPOSITION: accepted. D70. Same class as SVC-02 of the first review, in
a branch this chain added after it.

## SVC5-03 — low — arm advises killing what stop refuses to touch

CLAIM: after a crashed stop, arm copies every re-inventoried item into
the survivor list, including untracked processes and adopted-custody
runs, then advises ending them by pid. Stop itself reports both as not
the metasystem's and never signals them.

DISPOSITION: accepted. D71. A system that calls a process none of its
business must not then tell a person to kill it.

## SVC5-04 — note (critic: not material) — a successful retry is not credited

CLAIM: in the late-arrivals passes a component re-stopped successfully
keeps the survivor entry from its first attempt, so the run reports NOT
STOPPED, records the survivor and exits 1 for something it did stop.

DISPOSITION: accepted anyway. D72. The critic is right that the error
direction is safe, understating what the run achieved, but the record
and the page must describe the end of the run, not its first attempt,
and SVC5-01 is the same principle in the other direction.

## SVC5-05 — note (critic: not material) — the turn verdict answers before it reads

CLAIM: the turn verdict returns its stopped answer before evaluating its
other rules and reports a ledger status it never read.

DISPOSITION: recorded, not actioned here. Changing when the verdict
evaluates its rules touches the seat's own stop behaviour, which is not
this slice's subject, and the answer it gives is correct. It rides goal
metasystem-stop-escalation-proofs.

## SVC5-06 — note (critic: not material) — run launch reads the fence third

CLAIM: run launch reads the fence after two other gates, where section
9's precedence rule says the stopped refusal comes first; neither gate
can fail because the checkout is stopped, so nothing is defeated.

DISPOSITION: accepted as a consistency fix, D73, because the rule is
cheap to hold everywhere and a future gate added above it would
silently break it.

## Gaps the critic named, and the orchestrator's answer

- The brief named two reviewed trees: the orchestrator's editing error
  when it reused the first review's brief. The artifact reference was
  right and the critic reviewed the right tree.
- It could run no fixture bed and not the whole package matrix: expected
  and stated in the brief; the orchestrator ran all five beds and the
  matrix outside the sandbox on this tree.
- Two areas were read but not exhaustively: the orphaned-turn path in
  the mission stop, and the concurrency claims of the transition lock.
  Both are covered by execution rather than by reading: mission-stop
  proves the orphan path, and the lock's contention and dead-holder
  rules have package tests.
