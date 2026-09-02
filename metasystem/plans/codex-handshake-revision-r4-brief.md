Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal codex-handshake-budget-load-fragile)
Date: 2026-09-02

# Goal

Revision 4 of metasystem/plans/codex-handshake-design.md (revision 3
landed, in your worktree). Revision 3 left ONE open point in section 7,
the claim of a sessionless follow-up, and the orchestrator has decided it.
Fold the decision, then the consistency pass over D2.7, section 4 and the
`exit-before-session` round-2 leg in section 5. Edit in place; diffBoundary
is that one file. Keep it under ten minutes; do not re-read beyond the
lines named here.

# The decision (orchestrator m0b, 2026-09-02 17:05Z)

Candidate (a), the truthful record: the sessionless follow-up stays a
follow-up. Candidate (b) is rejected because a round-2 record whose
`dispatchMode` reads `fresh` while `round`, `parentJob` and `resumeMode`
say follow-up is a record that lies, and both the gate's repeated-follow-up
branch and the hazard check key on `dispatchMode`.

But WITHOUT bumping `LaunchFingerprintVersion`. Reason, to be recorded in
the design: metasystem/internal/dispatch/hazard.go line 339 admits a linked
evidence job only when its `fingerprintVersion` equals the current
constant, and metasystem/internal/dispatch/claim.go line 113 refuses a
standing record whose version differs as legacy; a bump would therefore
disown every existing proven evidence job (every critique register's
`--reviews` chain) and every record in flight across the upgrade. So the
extension is version-compatible:

1. `CanonicalLaunchRequest` (metasystem/internal/dispatch/claim_fingerprint.go)
   gains `ParentSessionless bool` with `json:"parentSessionless,omitempty"`,
   so the canonical form of every existing request is byte-identical and
   every v2 digest stays reproducible; state that explicitly.
2. `validateCanonicalLaunchRequest` (lines 275-292): the rule "follow-up
   requires a resumed session" becomes "follow-up requires a resumed
   session unless ParentSessionless"; a fresh dispatch with
   ParentSessionless set is refused ("fresh dispatch cannot mark a
   sessionless parent"); ParentSessionless with a non-empty resumed
   session is refused (contradiction). Name the test rows in the
   fingerprint test file (find it: the file that pins
   `validateCanonicalLaunchRequest` today).
3. dispatch.sh: the three claim lines (1934, 1961, 1965) pass
   `--session "$runtime:$child"` (the child's own key, as a fresh dispatch
   does at lines 1471, 1498, 1502) and `--resumed-session ""` plus a new
   flag `--parent-sessionless` when `sessionless_parent == 1`; the
   `claim-launch --preflight`, `claim-occupancy-prepare` and `claim-launch`
   verbs in metasystem/cmd/metasystem/dispatch_verbs.go accept the flag and
   set the field. `resumedSessionId` on the round-2 record is the empty
   string (it is an immutable identity field, record.go line 69; say what
   hazard.go lines 280 and 386 do with an empty value — read them — and
   whether the critic hazard check needs one guard).
4. Section 5, `exit-before-session` round-2 leg: assert `dispatchMode ==
   follow-up`, `resumeMode` fresh-context, `resumedSessionId == ""`,
   `fingerprintVersion == 2`, and the register's one synthetic finding for
   round 1. Section 4: claim_fingerprint.go and dispatch_verbs.go rows
   move from "unchanged" to changed with the above; section 7 becomes
   "None" plus the two implementer facts.

Bump the header to revision 4 naming the round. R-31: no benchmarks.

# Constraints

Wall-clock budget: 10 minutes. Design only; edit nothing but the design
file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the design file.

# Gap Rule

stop and report a gap; never fill it silently.
