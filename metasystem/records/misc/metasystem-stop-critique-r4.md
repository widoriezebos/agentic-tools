# metasystem stop: first code critique (chain stopverb-build1, round 12)

Critic stopverb-crit4 (code-critic, Opus 5, read-only) on reviewed tree
08cf9af5bbe5da917edad56bd61ab06197be6eb0. Six material findings, all
accepted. This was the chain's first independent read after twelve build
rounds; the acceptance beds and the eleven-package matrix were green in
the orchestrator's environment before it ran, so every finding here is
about the code's judgment rather than its execution.

## SVC-01 — high — the headless handover does not exist

CLAIM: a human at a terminal cannot restart a mission on a stopped
checkout. Section 7 says a HUMAN caller of `mission start` or `resume`
takes the transition lock, opens the fence, releases it and arms. The
built launcher reads the fence and refuses whoever is calling.
`Transition.OpenFence` is defined and never called by production code.

DISPOSITION: accepted, and the orchestrator's own fault. Fold-10's
decision D42 told the builder to read the fence before arming and said
nothing about the human path; the builder implemented exactly that. The
correction is D48, and the scenario that would have caught it comes back
into this slice under D54.

## SVC-02 — high — arm's refusals hand the human a loop

CLAIM: an arm refused because a stop is in progress, or because a
process survived the last stop, prints `run: metasystem arm` in both
cases. Section 9 requires `metasystem status` for the first and
`metasystem stop` plus section 2's words naming the pid and its start
time for the second. Only the remote-job branch builds a correct second
line, and it is the only one with a test.

DISPOSITION: accepted. D49. This matters most in the situation the goal
exists for, a checkout a stop could not fully clear.

## SVC-03 — high — the follow-up dispatch path has no fence gate

CLAIM: the ordinary dispatch path calls the fence gate before the census
gate, as round 10 required. The follow-up path, which launches every
continuation round of every chain, calls the census gate with no fence
read before it, so a stopped checkout is told its census failed and
pointed at the arming script.

DISPOSITION: accepted. D50. Nothing is created either way, because the Go
claim path reads the fence later, so this is the precedence rule rather
than a durability hole; it is still the exact rule the design singled
out.

## SVC-04 — high — two claim-directory states wedge a checkout

CLAIM: listing the creation claims fails on the first file that will not
parse, and the stop transaction turns that into an exit taken AFTER the
fence is written closed and BEFORE the inventory runs: nothing is
stopped, everything keeps running, and every later stop fails the same
way. Separately, a claim whose creator identity cannot be inspected is
never removed, so every later stop waits out its window and reports a
survivor that does not exist, and arm refuses forever. The only way out
is a person deleting a file in a directory the engine owns, and no
refusal says so.

DISPOSITION: accepted, and the worst of the six. D51.

## SVC-05 — medium — an unreadable fence record has no repair path

CLAIM: the reader rightly refuses malformed JSON and an unsupported
schema version, so every creation path refuses. But all three human verbs
refuse too: status discards the inventory it had already assembled, so
the operator's only diagnostic goes dark, and each verb's second line
names the command that just failed. A future schema version is not
hypothetical, since an older engine on a shared state root reaches this
state.

DISPOSITION: accepted. D52.

## SVC-06 — medium — eight named scenarios do not exist and no gap says so

CLAIM: the design puts every section 10 scenario except the fleet one in
this slice; five exist in the supervision bed and one in the dispatch
bed. Missing: mission-stop, proof-run-stop, remote-job, wrong-terminal,
slow-owner, crash-recovery, arm-refuses-survivor, ignored-signal. Two of
them would have caught SVC-01 and SVC-02 by execution.

DISPOSITION: accepted in part, and it corrects the orchestrator's cut.
The eight were deferred deliberately by fold-5's decision D20 to goal
metasystem-stop-escalation-proofs, so the scope decision stands for six
of them, but the cut was invisible where the list lives and no return
named it as a gap: D53 marks each scenario's slice in section 10 and D55
requires the return to name a deferral as a gap. And the cut was wrong
for two: mission-stop and arm-refuses-survivor prove behaviour THIS
slice claims, and their absence is why two high findings had to be
caught by reading. D54 brings both back into slice 1 with the held fake
host they need.

## Coordinator's reading (m1b, 2026-09-07)

The value of this review is settled: reading found two defects that
twelve green rounds did not, and both were in scenarios I had cut. The
lesson is not that critique is expensive but that a scope cut which
removes the proof of a behaviour the slice still claims is not a cut,
it is a hole.
