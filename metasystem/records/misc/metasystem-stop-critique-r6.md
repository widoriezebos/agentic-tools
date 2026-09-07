# metasystem stop: third code critique (chain stopverb-build1, round 19)

Critic stopverb-crit6 (code-critic, Opus 5, read-only) on reviewed tree
f278230abcacdc95cb8936f30129d8a5da2499a6, whose whole acceptance was
green in the orchestrator's environment. Five material findings and
three notes. Third read of this chain; the first two are
records/misc/metasystem-stop-critique-r4.md and -r5.md.

## SVC-06-01 — high — a checkout that can be stopped but never armed

CLAIM: `stop` and `status` take `--installation` to name the engine of a
checkout whose installation is neither at the top nor at
`<checkout>/metasystem`. `arm` defines no such flag and always resolves
with an empty installation, so it refuses before it reaches the fence,
and the refusal tells the person to type a flag `arm` rejects. The long
forms are closed too, because `steward arm` and `up` both refuse under a
closed fence.

DISPOSITION: accepted, and it defeats Wido's requirement directly: only
a human arm clears a stop, and in this layout no human arm can. D75.

## SVC-06-02 — medium — the common collision gets the wrong remedy

CLAIM: a stop holds the transition lock for its whole transaction, far
longer than the ten-second lock wait, so a concurrent arm exhausts the
wait and receives a lock-holder error rather than the typed
stop-in-progress error. It therefore falls through to the default second
line and is told to run arm again, where the design says it must be told
to run status and be shown the holder.

DISPOSITION: accepted. D76. The lock wait is not the signal; the
holder's identity is.

## SVC-06-03 — low — the crashed-stop branch aborts where stop records

CLAIM: when arm follows a crashed stop and one family's records cannot be
read, the re-inventory aborts on the first failure instead of recording
that family as a survivor the way the stop transaction now does, and the
untyped error falls through to "run arm", which cannot fix it.

DISPOSITION: accepted. D77. Section 15.1 in the branch we added for
15.1.

## SVC-06-04 — low — arm --all answers with a flag parser

CLAIM: section 13.3 says slice 1 refuses `--all` with a named sentence
and the per-checkout command; `arm` has no such flag, so Go's parser
prints "flag provided but not defined" and exits 2, which is neither the
grammar nor the exit code. `stop` and `status` get this right.

DISPOSITION: accepted. D78.

## SVC-06-05 — low — the return hid its own deferrals

CLAIM: section 14.4 requires every round's return to name each unbuilt
slice-1 scenario as a gap. Round 19 declared only sandbox gaps, so six
unbuilt scenarios are invisible to anyone closing the chain from the
return.

DISPOSITION: accepted, and the orchestrator's to enforce as much as the
builder's to obey. D79.

## SVC-06-N1 — note — the last early exit, latent

CLAIM: a family whose name is not one of the six numbered shutdown steps
makes the stop transaction return a hard error after the fence is closed,
discarding every line gathered. It cannot fire today because the family
list contains exactly those six plus untracked, but it becomes live the
moment a family is added, and the failure mode is the one section 14.1
was written against.

DISPOSITION: accepted although not material today. D80. A trap that
arms itself when slice 2 adds a family is worth one line now.

## SVC-06-N2 — note — one quoted path

DISPOSITION: accepted, D81, one character of consistency.

## SVC-06-N3 — note — the deferred enumeration costs more than it looks

CLAIM: the untracked family re-runs the census on every inventory pass,
so one stop can run up to five censuses beside the supervision family's
own snapshot, where section 3 says one pass.

DISPOSITION: recorded for slice 2, goal metasystem-stop-fleet-form,
whose intent already owns the shared enumeration. The deferral stays
honest; its cost is now written down.

## Gaps the critic named, and the orchestrator's answer

- The brief's numbers disagreed with the artifacts: it said 62 files and
  twice called the record the round-12 review record while naming round
  19. The orchestrator's editing error, the second of its kind, from
  reusing an earlier brief. The critic used the artifacts and reviewed
  the right tree.
- It could run no bed and only two packages: expected, stated in the
  brief, and covered by the orchestrator's own runs.
- It did not drive a live stop, so section 4's ordering claims are read
  rather than executed: those are covered by the beds.
