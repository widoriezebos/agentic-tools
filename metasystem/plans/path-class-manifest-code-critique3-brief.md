Working Mode: implement
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Final closing code review of the path-class manifest, first part (chain
root path-class-build1, terminal round path-class-build1-r4; its round
evidence holds diff.patch and review.json with the reviewed tree). Your
previous register, metasystem/records/misc/path-class-manifest-code-critique-r2.md,
left two material findings and one gate fact, folded by
metasystem/plans/path-class-manifest-build1-fix3-brief.md. Verify the
three closures on the final tree and confirm nothing regressed; the
chain closes on your zero.

# Mandate

1. PCM-CC2-001: the waiver rule in internal/validate/conformance.go
   refuses behavior, ledger, runtime and unclassified inside the
   installation and leaves record and outside paths waivable; the test
   covers each class.
2. PCM-CC2-002: in the root layout (installation at the repository root,
   empty git prefix) the waiver decides by mode before namespace and an
   application file is waivable; the root-layout test exists.
3. PCM-CC2-003: internal/landing has a coverage floor in both baselines
   at the measured number, and internal/pathclass keeps its floor.
4. Regression check of the whole first-part diff against the certified
   design (metasystem/plans/path-class-manifest-design.md revision 2) and
   the earlier closures (PCM-CC1-001 to 005, PCM-R2-001 and 004): still
   met on the final tree; boundary unchanged except the two baselines.

A finding is material only if it changes what gets built and names the
artifact. If nothing material remains, say so; that closes the chain.

# Constraints

Wall-clock budget: 20 minutes. Return per the code-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
