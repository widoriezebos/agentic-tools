# Dispositions: show-cc1-20260907, round 1 (the chain's first review)

Chain under review: show-build1d-20260907 (reviewed tree
d512426b64fe1103aff6c45bf38b497efa6c53bb, round 1). Critic:
show-cc1-20260907 (claude, Fable), five material findings and two
notes; the critic ran the reviewed binary on scratch checkouts for
each claim. The chain folded once (job show-build1d-20260907-r2, brief
plans/stop-hook-open-work-refusal-repeats-fold-brief.md) and got one
re-review. Orchestrator: m1d. Seat replay on the reviewed tree outside
the sandbox: build, vet and gofmt clean; go test of internal/report
and cmd/metasystem green; the whole supervision-hook fixture suite
passed, the new once-only, deadline-expiry, running-job and
placeholder assertions included, which shows the fixtures of round one
did not reach the defects the critic found by running the binary.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| SHO-01 | folded | True: the open-chain rule was reachable only from the report verb; the verdict's scanner still counted pending or running job records alone, so an open chain silenced the report and left the verdict blocking. | fold item 1: the verdict counts open chains; a verdict test |
| SHO-02 | folded | True: bad JSON, a foreign schema version, a nil map or a digest mismatch made the verdict exit 1, which the hook turns into a blocking "turn-verdict unavailable" on every stop until a human deletes the file. | fold item 2: fail open with one visible line; tests per shape |
| SHO-03 | folded | True: the deadline parent's stop-block marked every open line seen while publishing only the fixed deadline sentence, so a line first seen during an expiry never blocked; the round-one fixture leg pinned that behaviour as the contract. | fold item 3: the deadline path reads only; the leg asserts the line still blocks once |
| SHO-04 | folded | True: the key was the digest of the clipped 200-byte display, so an edit past the clip stayed silent. | fold item 4: digest of the full plan line |
| SHO-05 | folded | True: entries were only ever added. | fold item 5: stale entries dropped on write |
| SHO-06 | noted | True and narrow: the verdict marks before it publishes, so a worker killed inside the deadline window spends a refusal the seat never saw. Recorded; item 3 narrows it. | none |
| SHO-07 | folded as a note | True: the terminal status vocabulary duplicates the dispatch package's. | fold item 6 |
