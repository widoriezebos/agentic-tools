Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal human-carried-landing-carry, third round on chain hcl-build3-20260911)
Date: 2026-09-11

# Fold: the third read's four findings, and the host's run of round 2

Round 2 of this chain (reviewed tree fdfcd002b69a1a39fe681956d9226e245aea7127)
was read on Opus (hcl-cc3-20260911): eight findings, four material, the
verdict "one fold: HCL-C-66, -67, -68, -69". The coordinator also ran
every bed and the affected packages on the host. The decisions below are
binding and small; change nothing else. Rule of the round, unchanged: no
fixture claims what it does not drive; no waiting on the sandbox (name a
sandbox-bound test within five minutes and move on). Cited context files
under artifacts/ are not in your worktree; every decision you need is in
this brief.

# What the host found on your round-2 tree

- The land bed (cap scale 8): 23 of 25 legs green, every existing leg and
  the carried legs fresh, asks, ledger-path, crash, two-seat,
  debt-abandoned, debt-expired, intent-failure, red-battery and second
  among them. Red: `carried-prefixed` and `carried-crash-local`, the two
  your sandbox could not run.
- `carried-prefixed` fails before its assertion, at the supporting green
  battery: `test run` refuses with
  "TEST_POLICY_ENGINE_REQUIRED: retained destination engine does not bind
  the captured policy base: read enrolled source ENGINE projection: git
  archive 5c1a17307c520075565267aa8e1b92b55afd5dc9:metasystem: exit status
  128 (fatal: not a valid object name: ...:metasystem)". The leg's
  receipt-runner enrollment captured a policy base commit whose tree has
  no `metasystem/` prefix, so the engine projection cannot be read under
  the prefixed installation. Fix the leg's construction (seed the prefixed
  repository so the captured base has the prefix, and enroll the runner
  with the prefixed root, the way this repository is laid out), then make
  the leg prove what its name says: the same fresh carried landing as
  carried-fresh, installed under `metasystem/`, lands.
- `carried-crash-local` fails at "rerun did not push the preserved local
  commit", exactly the third read's HCL-C-67 (decision 2 below).
- The goal-cli bed: every scenario green. The dispatch bed and the
  affected package tests were still running when this brief was written;
  the coordinator names anything they find in the next message if they
  find anything.

# The four decisions

1. **HCL-C-66 (high), the contract names the new test.**
   `TestHCL51CarriedCounselorAppendBelongsToItsRow` is added to the
   `carry-landing-standard` group's named test list in testing.json, so
   `TestHCL34PlanExecutesEveryFixture` (internal/testpolicy) passes again.
   Then run `go test ./internal/testpolicy` and `metasystem test check
   --root .` and say the result; both are in the proof list from now on.
2. **HCL-C-67 (high), the crash-local leg gets the page's oracle.** The
   leg `carried-crash-local` proves HCL-06-CRASH-BEFORE-PUSH-RESUMES and
   HCL-19-RECOVERY-KEEPS-REBASED-COMMIT: the first run is killed at the
   `before-push` seam with the reservation OPEN (the seam fires before the
   exit trap can abandon, or the leg kills the wrapper by its recorded pid
   as the design's pause seam describes, so no closer row is written); the
   rerun `land.sh --carried <opid>` takes the `local:` branch, keeps the
   rebased commit and pushes it. The oracle: origin's history holds exactly
   ONE code commit found by `git log --grep 'Carry: <opid>'`, its tree is
   the candidate's tree rebased (compare the tree of the commit's
   `metasystem/` payload with the first run's staged payload, not the
   sha), the goal holds exactly one `carrying` row and one `carried` row
   for the word, and the counselor line is written once. Do not compare
   origin's tip with the first run's local sha: the record's ledger
   commits follow the code commit on the same branch.
3. **HCL-C-68 (medium), the row the table lacked.** Add to the carried
   match table the row: word `group:G`, ordinary observation would-refuse
   `missing-declaration`, delivery with G missing or failing, expected
   `carry-refusal-mismatch`. Prove it the way the critic did: with
   `allowedOrdinary` forced true in decideCarriedMatch the table must
   FAIL; say that you ran the mutant.
4. **HCL-C-69 (medium), paying the debt must not block the next carry.**
   Decision: in carried mode the append-only admission of decision A10
   (the first fold) covers every append-only counselor register that
   metasystem/internal/landing/registers.go lists, not only
   carried-landings.jsonl: an appended line is admitted when it belongs to
   a row on the tree (a carried-landings line to a carried row; an
   accepted-risk-register line to an accepted-risk row whose chain is
   `human-carried`); any other change to those files, and any new
   goal-less file under records/counselor/, is refused as before. The
   `carried-second` leg stops deleting the accepted-risk register and its
   lock files: after `goal accept-risk` pays the first carry's debt, the
   second carried landing stages the register's new line and lands
   (HCL-08-DEBT-PAID-LANDS). Say in the return which registers.go list you
   extended and cite the observer's admission site.

# Not actioned, recorded

HCL-C-70 (trunk's rulings id-mint check under a prefix; the coordinator's
backlog), HCL-C-71 (the red-battery trailer regex), HCL-C-72 (negative
fence rows), HCL-C-73 (a malformed reservation line): recorded by the
coordinator, no change in this round.

# Proof before you return

`go build ./... && go vet ./...`; `go test ./internal/landing
./internal/testpolicy ./internal/refusal` (sandbox-bound tests named, not
waited for); `bash scripts/agents/go-build.sh` then `metasystem test check
--root .`; `bash scripts/agents/go-gate.sh --fast`; `bash -n` on every
changed script; the legs `carried-crash-local` and `carried-second` with
`--fixture-bed-child` if the sandbox allows (say which could not run; the
coordinator runs every leg on the host).

# Return

`diffBoundary` and `files` are repository-root paths. Under `evidence`
every command with its observed result; under `deviations` anything left
out with the reason. Return BEFORE the cap: this round is four items;
at 60 minutes run the proof list and return.
