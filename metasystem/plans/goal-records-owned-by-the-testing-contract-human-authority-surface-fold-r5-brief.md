Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal goal-records-owned-by-the-testing-contract)
Date: 2026-09-10

# Fold round five: every ancestry walker learns the new top of the tree

Follow-up round on chain ha-surface1-20260910. Round four (the rebase)
was reviewed clean. Its landing receipt, run with the candidate engine
inside the receipt's beds, failed three shell sections with one cause:
the caller classification answers UNTRUSTED where the fixtures expect
DELEGATE or a main. goal-cli: "wrong-terminal refusal did not match:
this caller is UNTRUSTED" (expected DELEGATE) and "proof-grade fixture
bed classified UNTRUSTED, not HUMAN or DELEGATE"; land-fixtures
brain-land-refuses: "checkout lease is absent and caller pid is
UNTRUSTED, not an authenticated main"; dispatcher steward-continuation:
"OWNED-ELSEWHERE ... (caller is UNTRUSTED)".

Where to look. Under this seat's own shell every gate passed with the
candidate engine, and `lease classify` answers MAIN for a shell and for a
detached child alike, because the walk meets this seat's main
announcement before it reaches the top of the tree. Inside a receipt the
beds' processes are what the fixtures classify: fake agents made with
`exec -a metasystem-fake-agent /bin/sleep`, headless scripts started in
a new session, holders behind script(1). Two candidate causes, to be
settled by reproduction, not by reading:

1. identity.ParentPid now answers (0, true) at the top of the tree
   (enumerate_darwin.go, enumerate_linux.go) where it answered (x,
   false); only the loop in internal/lease/classify.go was adapted
   (`for ok && current > 0`). internal/census/ancestor_production.go sets
   ppid to 1 only on !ok and now records a parent of zero;
   internal/lease/hook_delegate.go stops on !present or parent == current;
   internal/lease/claim.go descendsFrom keeps walking from zero. A walk
   that reaches the top now behaves differently from before.
2. The new admission of root-owned protected system images with withheld
   arguments (internal/humanauthority/authority.go protectedSystemImage,
   and whatever internal/identity now reports for such a process) may
   read a fake agent's argv, `exec -a metasystem-fake-agent /bin/sleep`,
   as the system image /bin/sleep with its arguments withheld, so the
   adapter signature that makes it a DELEGATE no longer matches.

## Mandate

1. First reproduce: build the engine from this worktree, then run,
   with it as METASYSTEM_BIN and detached from any terminal through the
   engine's own proc detach verb, the goal-cli scenario wrong-terminal,
   the land-fixtures scenario brain-land-refuses and the dispatch-fixtures
   scenario steward-continuation; confirm the UNTRUSTED answers, then find
   the one classification step that differs from main's engine on the same
   bed (main's engine is bin/metasystem in this checkout's parent
   installation; a `lease classify --caller-pid` on the bed's processes
   from both engines shows the step).
2. Fix that step minimally and every sibling of it: if it is cause 1,
   adapt every caller of identity.ParentPid so a parent of zero with ok
   true is the top of the tree and not an ancestor (census ancestor,
   hook_delegate, claim.go descendsFrom, and any other a grep finds), each
   with a unit test on a fake tree whose top answers (0, true); if it is
   cause 2, the adapter-signature match must read the argv the OS
   reports for a user-owned process even when its executable is a
   protected system image, with a unit test that plays `exec -a`.
   Name the cause and every touched file in your return.
3. Change nothing else.
4. Prove: go build, vet, gofmt; go test on internal/lease, internal/census,
   internal/humanauthority, internal/identity, cmd/metasystem; then, with
   the rebuilt engine as METASYSTEM_BIN, the goal-cli bed scenario
   wrong-terminal, the land-fixtures scenario brain-land-refuses and the
   dispatch-fixtures scenario steward-continuation, each run detached
   from a terminal (the engine's own proc detach verb starts a command in
   a new session) so the walk reaches the top as it does in a receipt.

## Constraints

Wall-clock budget: 30 minutes. Return per the implementer schema and
report the round as your own. Stop at a gap that needs a decision no
page has made; report it with the resolution you propose. Never delete
written work.
