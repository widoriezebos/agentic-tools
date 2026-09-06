# claude-implementer-read-roots-writable, critique round two: dispositions

Chain cir-build1: round one denied the read roots outright and the
orchestrator's live implementer probe refused it before any critique
(the claude sandbox lets a denied ancestor override a nested allow,
so the implementer's own worktree under the repository was refused a
write). Round two denies the existing entries along the write root's
ancestor chain instead. Critic cir-critic2 (claude, claude-fable-5-1,
with a shell), reviewed tree c9fe326ed29c8219fd16daa928659eb1851e44ba,
the whole chain diff: zero material findings. Seat-side proof: the
adapter package, vet and gofmt green; the live probe with the
round-two engine wrote the worktree, ran a go test, and was refused a
new file in metasystem/, a touch of go.mod, a new file under
internal/ and a write into a sibling worktree.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| F-1 | noted | The deny list is a snapshot at settings time: entries another job creates later under an ancestor (a sibling worktree, a new ref file, .git/index.lock) stay writable for the session. Inherent to the design; the comment names the narrower residual. | none; recorded here and on the goal. |
| F-2 | noted | The scratch directory is not consulted by the denial; a read root containing it would deny it. No such root exists in the fleet's envelopes. | none. |
| F-3 | noted | A symlinked entry is denied by its link path; no such link exists along any fleet ancestor chain. | none. |
| F-4 | noted | Implementers receive no --add-dir (added only when write roots are empty) and run with the worktree as cwd, so the original hole was narrower than the goal's intent said; the denial is defence in depth. A discriminating probe with the pre-chain engine is recorded in the goal's conclusion. | none. |
| F-5 | noted | The shared test fixture's workspace was set equal to the write root because an absent workspace above a write root now fails at the directory read; no assertion changed. | none. |
