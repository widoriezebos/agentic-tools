# Brief: review-chain-stops-and-records, unit tree-boundaries-3 (publication boundaries and the remaining waits)

Working Mode: Implement
tree-boundaries parts 1 and 2 with fixes are committed on this branch. Read part 2's handoff, /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/handoff-review-chain-stops-and-records-tree-boundaries-2.md, "Next step": this unit is that remainder. Spec: plans/designs/review-chain-stops-and-records.md Decision 4 paragraphs 1-3.

Build: the publication and other remaining tree-writing entries through the reservation, with tree/unit/run order in ReviewSubject (unit_review.go:105-109); release command and branch locks during supervisor startup (launch.go:310) and remaining remote waits; committed-critic children kept in the owner's custody across commands; the fresh tree comparison under the tree lock at publication (a tree changed after the read is never a clean publication). Also, from part 2's read: `work wait run:OP` on a live branch operation waits instead of answering at once (intent_work.go:1936-1944); a failed lock retake in waitLaunch (tree_reservation.go:316) prints a plain retry remedy instead of a raw error.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 (publication boundary and fresh comparison first) and report the rest.
Public-verb test: change the tree after the read: publication is not clean; a committed critic outlives its command and the tree stays owned until it ends; a competing build during supervisor startup is not blocked by a held command lock; `work wait run:OP` waits for a live rebase. Mutations: omit the fresh publication comparison; drop the critic from custody.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
