# Task: fold critique round 4 into plans/designs/lane-policies-and-helm.md and accept it

Working Mode: Design
Round 4 (the last) found 2 material findings, each with a concrete change. Fold both exactly, mark each as an acceptance item of its unit (a line "Acceptance item (round 4): ..." under the unit), and set Status: accepted. Add under the header lines: "- Critique: closed at round 4 on 2 material findings folded as acceptance items (rounds 7, 5, 4, 2; U2, U3, U6 split to lane-reads-its-policies and lane-drain-and-fresh-claims under the plan's design stop rule)". Never open any metasystem.conf.local. Edit only this page.

1. U1, Decision 1: accept a direct-person proof (Proof.Helm == nil, EnrolledTerminalFor) at the calling checkout, and failing that at the destination; refuse only when neither holds; say which root is used when no checkout contains the working directory; do not change the roster caller (intent_roster.go:65 keeps proving at --repo). U1 test added: a terminal enrolled only at the lane, running the command from a directory outside any checkout, succeeds and records set-by. An enrollment with no name records set-by "author unknown".
2. U5, Decision 5: an unreadable queue counts as unknown areas (the claim publishes with the "areas unknown" warning naming the queue path), never a hold; a computer with no configured lane reads as an empty queue (landing_plain.go:40). U5 test: a corrupt queue file, the claim publishes with the warning.
Also, one sentence each: helm return becomes person-only, and its help text and the hook message at internal/hooks/runtime_hook_stop.go:145 change with it (U4); a released claim on another computer is outside step 1 (Deferred).
Return the final units table and the two acceptance items.
