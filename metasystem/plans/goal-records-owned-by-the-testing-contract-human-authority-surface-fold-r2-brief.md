Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal goal-records-owned-by-the-testing-contract)
Date: 2026-09-10

# Fold round two: carry the reviewed hp-terminal change with the surface

Follow-up round on chain ha-surface1-20260910. Round one added the
human-authority and report-scan surfaces to metasystem/testing.json
(critic ha-surfacecrit1b-20260910: four notes, nothing material). Its
landing receipt failed on this Mac in the new humanauthority-standard
group, although every test passed: one test in internal/humanauthority
on main, TestLinuxDoesNotAdmitLoginWithWithheldArguments, skips outside
Linux, and the proof run counts a skipped expected test as a failed
group. The package on main can therefore never pass its own group here.

The reviewed change of chain hp-terminal-build1-20260909 (fourteen
rounds, twelve critics, nothing material left) removes both OS-specific
system-login files from internal/humanauthority and rewrites its tests
so that none skips on Darwin; it could not land because those paths
were unowned, which is what round one of this chain fixes. The two
changes depend on each other, so this round carries both.

## Mandate

1. Apply the hp-terminal chain's fourteenth-round diff verbatim on top
   of this chain's round one. The diff artifact lives in this checkout's
   artifacts directory under the job directory named
   hp-terminal-build1-20260909, in its rounds directory, round 14, file
   diff.patch (paths in it are relative to the metasystem directory; it
   applies cleanly to main today: I checked with git apply). Twenty-five
   files: cmd/metasystem (goalsync_mutations, identity, session_stop and
   their tests), internal/goal (authority_test, obligation_authority_test,
   order_test, recover, recover_test, sessionstop, stop_test, verbs,
   verbs_test), internal/humanauthority (authority.go, authority_test.go,
   and the deletion of system_login_darwin.go and system_login_linux.go),
   internal/identity (enumerate_darwin, enumerate_linux, enumerate_test),
   internal/lease/classify.go, internal/refusal/register.go,
   scripts/agents/goal-cli-fixtures.sh.
2. Change nothing else. Do not edit the diff, do not reformat, do not
   resolve anything by hand: if a hunk does not apply, stop and report
   which one.
3. Prove: go build, go vet, gofmt; go test for internal/humanauthority,
   internal/goal, internal/identity, internal/lease, internal/refusal and
   cmd/metasystem; confirm that go test -json on internal/humanauthority
   reports no skipped test on this machine.

## Constraints

Wall-clock budget: 20 minutes. Return per the implementer schema and
report the round as your own. Stop at a gap that needs a decision no
page has made; report it with the resolution you propose. Never delete
written work.
