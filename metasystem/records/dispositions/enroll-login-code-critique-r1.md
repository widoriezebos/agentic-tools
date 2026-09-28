# Dispositions: enroll-login-crit1, round 1

Chain under review: enroll-login-build1 (reviewed tree
fcee5b005713c89603a90a52d913328e647a0234). Critic: enroll-login-crit1,
one material finding. Orchestrator: m1b.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| ELN-01 | accepted | Ran on the reviewed tree: `go test ./cmd/metasystem/ -run ClaimLaunchCapabilitySurvivesRelativeExecutableProcessShape` fails at the helper's relative-path precondition with the absolute path of the copied test binary; the whole command package has no other failure. The relative shape came from the old argument-embedded exec path and cannot occur through the new image-path reader. | Follow-up round 2 per metasystem/plans/enroll-terminal-login-node-fold2-brief.md: the helper pins the new contract (absolute path resolving to the launched copy) and keeps the capability assertions and the relative launch. |
| ELN-02 | noted | Read internal/run/verbs.go kinship: the ancestor set now extends past the root login to Terminal.app on macOS, so two Terminal.app tabs are kin the way two tmux windows and two Linux shells already are. The predicate's looseness predates this change and is outside the declared scope; no artifact changes. | none |
| ELN-03 | noted | The readable-then-withheld sequence refuses ARGV_UNREADABLE (main's outcome) with a reason that misdescribes the process; the branch cannot fire for a real setuid login, so it is not material. Folded anyway because D5 requires a true reason. | Reason text and one table case, in the same round-2 follow-up. |
| ELN-04 | noted | The first-read precedence (executable and owner before arguments) is what D6(c) requires: the rule cannot judge a login program without its executable, and both outcomes carry the workaround sentence. | none |
| ELN-05 | out-of-scope | True as fact: an agent that starts its own session already holds a shell that is its own session leader. The build brief records this boundary as one the goal neither opens nor closes, and the review brief's threat model puts it out of scope. | none |
