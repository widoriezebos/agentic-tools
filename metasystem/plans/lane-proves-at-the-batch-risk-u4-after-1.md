# Unit U4 correction 1: an overdue full proof must not stall a busy lane; ancestry includes superseded hand-ins

Working Mode: Implement. Correction of unit U4 (brief `plans/lane-proves-at-the-batch-risk-u4-depth-class-and-full-due.md`, design D5/D6) after its Opus read. The worktree holds U4 uncommitted; change only what this brief names.

## F-1 (material): while full-due is raised, ledger-only moves of main force a whole full proof again and again

Traced: the batch's full proof goes green; a seat's goal verb pushes a ledger-only commit to origin/main; the push is refused (HEAD does not contain main); the agent merges main and runs `landing prove`; Settled now refuses the ledger-only inheritance while overdue (`prove.go` ~605-607); `proofScope` returns full (`proof_admission.go` ~50) and Run skips inheritance because the scope is not "inherited" (`prove.go` ~832): a whole full suite runs again, and any ledger commit during it repeats the loop; case 8's trunk proof never fires while lines wait. Second gap: an inherited push never resets the clock (`wake.go` ~139 excludes "inherits green from tree"), so even with inheritance every batch would stay full.

Fix: a ledger-only refresh (the merge changes no proof input: same code tree) inherits the batch's own fresh FULL green exactly as before U4, overdue or not, and that inherited green PAYS the full-proof clock when pushed: carry FullTree/FullAt from the full green it inherits (the clock rule at `wake.go` ~130-139 then sees a pushed full-scope result whose FullTree is the proven tree; adjust the "inherits green from tree" exclusion so an inheritance FROM A FULL GREEN counts and an inheritance from a scoped or impact green does not). Skill case 5's promise ("reports the green at once") holds again.

Test: full-due raised; batch proves full green; a ledger-only commit moves main; the refresh merge's `landing prove` inherits (no runner call) and the push resets the clock (mutation: refuse the inheritance -> a second full run, test fails; mutation: keep the exclusion -> clock not reset, test fails). Keep the existing test that an inheritance from an impact/scoped green does not reset the clock.

## F-2 (material): the ancestry rule looks only at waiting tips

Traced: goal A (tier 3) handed in at A1 then again at A2 (fix-forward); cheap goal B stacked on A1; B contains neither A2 nor is contained by it, so `depth.go` ~63-79 sees no relation; B is selected alone; admission iterates every queue line incl. A's superseded A1, A is unselected, the candidate contains A1 and main does not -> refused "the candidate contains an unselected, superseded, returned or held hand-in: A" (`batch.go` ~722); the prepared batch is reused on the next selection, so it repeats.

Fix: the ancestry relation between two waiting goals holds when either goal's branch (its waiting tip) contains ANY hand-in commit of the other goal still on the queue (current or superseded lines), or is contained by one. Test: the traced fixture (A1 superseded, A2 waiting, B on A1) -> one batch of A and B at full (mutation: tips only -> B alone, refused at admission).

## Checks

`go test -count=1 -timeout 30m ./internal/landing/plain`, the cmd tests by name (`TestLandingSelect|TestSelectBatch|TestFullDue|TestWake|TestLandingDepth|TestLandingProve|TestLandingPush|TestLandingFresh|TestLandingFlake|TestLandingProofPermission|TestLandingStatus`), every existing test using a seam you changed (grep, by name), `go run ./cmd/devgate static` (ignore the design-record complaint about the ledger: this worktree predates the goal's ledger commit; report it and nothing else). Never edit testing.json; never open metasystem.conf.local; do not commit. Report: diff --stat of the correction, each exit, the two traced scenarios before/after.
