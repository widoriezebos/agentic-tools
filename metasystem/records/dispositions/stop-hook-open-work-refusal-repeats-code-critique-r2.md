# Dispositions: show-cc2b-20260907, round 2 (the chain's closing review)

Chain under review: show-build1d-20260907 (reviewed tree
4479b852da4e836f373f8e66e3e9d7cb6d032399, round 2, the same eight
files as round one). Critic: show-cc2b-20260907 (claude, Fable), zero
material findings, three low notes; the chain is closed on this
review. Orchestrator: m1d. The reviewer could not run the whole
fixture suite in its sandbox (bash 3.2 here-documents are denied) and
replayed the open-work verbs against the built binary instead; the
seat's outside-sandbox replay on the reviewed tree covers the gap:
build, vet and gofmt clean, go test of internal/report and
cmd/metasystem green, and the whole supervision-hook fixture suite
passed with the new once-only, malformed-state, deadline, open-chain,
running-job and placeholder scenarios.

All five round-one findings are closed: the verdict counts open
chains, a corrupt seen-state fails open with one reset line, the
deadline path reads and never writes, the marker hashes the full plan
line, and stale entries are pruned on write.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| SHO-06 | noted | True and unchanged from round one: the marker is written before the verdict is published, so a verdict that fails closed or a human-authorized stop marks a line without a refusal that names it. The line still renders on later verdicts, so nothing is hidden from the seat. Moving the write after a published verdict is a separate change. | none |
| SHO-08 | noted | True and adversarial: a directory deliberately created at the seen-state path makes the read fail open but the atomic rename fail, so the verdict exits 1. A write failure, outside the round-one mandate, and it needs a hand-made directory. Backlogged with SHO-06. | none |
| SHO-09 | noted | True as certification: the full-line digest is proven by unit tests with hand-built items and by the critic's own binary probe on a long Next step, but no fixture drives a long step through the scanner. The behaviour is proven, the fixture leg is not. | none |
