Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal turn-verdict-hardening)
Date: 2026-09-02

# Goal

Independent critique of metasystem/plans/turn-verdict-hardening-design.md
(landed, in your worktree) — the design answering your own round-1 critique
of the seat-stop analysis (metasystem/records/misc/seat-stop-analysis-critique-r1.md;
the analysis is metasystem/records/misc/seat-stop-analysis.md). Wido's order:
deterministic Go code that makes a seat ending its turn on ready work
impossible, or as near as the machinery can get.

# Your mandate

1. CLOSURE CHECK: for each of your eleven SSA-R1 findings, does the design
   close it, name it as a residual honestly, or miss it? A finding the
   design claims to close but does not is material.
2. ATTACK READY (section 1): the queued clause extracts the claim verb's
   checks into a side-effect-free ClaimAdmission so READY and claim cannot
   disagree. Read metasystem/internal/goal/verbs.go — is the extraction
   complete, and can the extraction itself drift (two call sites) rather
   than the checks? Does the machine+lineage scoping produce a two-seat
   refusal loop neither seat can lawfully exit?
3. ATTACK RELEVANT INFLIGHT (section 2): is the liveness proof (live pid
   probe or live waiter) sound against the job records that exist
   (metasystem/internal/report/scanjobs.go, metasystem/internal/lease/verbs.go)?
   Is the residual list (gate/mission runs never excuse READY, run records
   unjoined in slice 1) acceptable or does it recreate the false refusal
   the design exists to avoid?
4. ATTACK FAIL CLOSED (section 3): the twenty-one row table; the Stop
   budget re-ordering (ceremonies behind the verdict, timeouts to 20 s) —
   trace it against the hook metasystem/scripts/agents/supervision-hook.sh
   and the runtime facts in metasystem/internal/runtimes/runtimes.go. Is a
   fail-closed hook a wedge with no lawful exit on an offline machine, and
   is "human word only" an acceptable exit?
5. FRESHNESS and HUMANSTOP (sections 4, 5): is the cursor a correctness
   witness or another banner? Is compare-and-consume inside the existing
   flock actually atomic against the second Stop, and is the
   enrolled-terminal proof the right authority?
6. SLICES: does slice 1 alone refuse all three specimens as replayed? Is
   anything in slice 1 actually more than 240 reserved minutes?

Findings material and grounded, quoting the disagreeing text or code. Your
sandbox is read-only: verify by reading, do not run go. The two open asks in
section 9 are Wido's and not findings.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
