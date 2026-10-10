# Brief: review-chain-stops-and-records, unit round-result

Working Mode: Implement
Goal 2. build-outcomes and tree-boundaries (all parts) are committed on this branch. Spec, and ONLY this text (do not read more scope into it): plans/designs/review-chain-stops-and-records.md Decision 4 paragraph 4 ("At publication, fetch the current branch tip once ... Record the commit operation before writing it and reconcile that ..."), the round-result rows of the Units and public-verb tables, plus the two items moved here from tree-boundaries (design page "Decided by m1e for Wido" 03:50): committed-critic children kept in the tree owner's custody across commands, and a person's recovery when the advisory ownership record is unreadable.

Build: each round records its actual result tree, parent, patch digest and proof identity when its build ends; at publication the branch tip is fetched once; if it moved, the round's frozen patch is replayed onto the tip in scratch; a conflict holds with the exact conflict and names a correction bound to the retained round (an unrelated tip move never asks for a rebuild); the replayed tree passes the cheap rung before committing; the read is reused only through the existing change/fold-equivalence checks, otherwise the replayed result is read; a correction commit passes its own static and cheap gate before it is installed; the commit operation is recorded before writing and reconciled on replay. Plus: committed critics stay in the owner's custody; a person can recover through `work stop run:RUN` even when the ownership record is unreadable.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 (result recording, the replay at publication and its conflict hold first) and report the rest.
Public-verb test (from the design): `work review G --work U` after an unrelated branch advance replays U once, gates that exact replay and publishes; a conflicting advance holds with the conflict and a correction bound to the round; no rebuild. Mutations: rebuild on an unrelated tip move; publish the replay without the cheap rung.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
