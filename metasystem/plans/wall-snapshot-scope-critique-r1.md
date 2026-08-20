# Snapshot-scope critique, round 1 dispositions

Chain: codex gpt-5.6-sol xhigh, read-only, both-must-agree. Failsafe
round declared at loop start: round 4. Round 1 verdict: 12 material
findings; every one adjudicated below (accept amends the design;
refutation carries its evidence).

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| WSS-R1-01 | ACCEPTED (resolved differently) | Confirmed: composition iterates adjudicated `certified[]` (wall.go:186); no pre-integration order artifact exists. But the ordered-prefix lane never needed the order: consumed patches are pairwise disjoint by the parent's overlap rule, so any SUBSET composes deterministically | Membership is subset-decomposition, order-free; the recorded order stays the parent's replay concern |
| WSS-R1-02 | ACCEPTED | An ours-merge burying an illicit second parent passes result-only checks | Rule 3: every non-first parent's tree must equal a consumed authorization's reviewed tree or be accounted; interior side commits stay the stated boundary, now bounded by their tip |
| WSS-R1-03 | ACCEPTED | Commit-tag-reset retains shipped bytes under a live ref with an empty final chain; refs are live state, not archaeology | Rule 5 + openTurn refMap anchor: created/moved refs/heads//refs/tags tips must be accounted-or-reviewed; live agent/* delegate branches (dispatch.sh:933) and refs/metasystem/* exempt; deletions pass. New row WSS-11 |
| WSS-R1-04 | ACCEPTED | Confirmed: configPins (gittree.go:43) never disable object replacement; a replace ref rewrites every comparison | core.useReplaceRefs=false joins the pins; new row WSS-10 |
| WSS-R1-05 | ACCEPTED | The real index at repository scope was unwatched; a staged sibling entry with a restored worktree escaped both obligations | Rule 6 gains the toplevel real-index check: prefix-only deltas against toplevel HEAD |
| WSS-R1-06 | ACCEPTED (kills the design's own WSS-6) | Freezing the seed at the open commit falsely rejects an authorized commit adding a tracked-and-ignored path — read-tree from the open commit omits what the expected tree carries | Live-HEAD seeding restored; soundness from rule ordering (HEAD proven accounted before the equation is read); conservative intermediate-subset edge stated and fixtured |
| WSS-R1-07 | ACCEPTED | Refusing an index equal to the human-sealed baseline contradicts the parent's clean-or-sealed admission (host-implementer-wall-design.md §tree composition); the sealed tree is reviewable by equality | Mission-start staged admission accepts HEAD's tree OR the admitted baseline |
| WSS-R1-08 | ACCEPTED IN PART, REFUTED IN PART | Accepted: the design must state its reading explicitly and reconcile the O15 row text. Refuted: refusing empty commits, E-point commits, and returns to accounted trees guards no invariant — no unreviewed byte moves — and the strictness rule (AGENTS.md work contract) forbids encoding a check that breaks benign variation; the wall proves BYTES, not ceremony | The accounted-set paragraph states content-accounting and the row-text reconciliation at landing |
| WSS-R1-09 | ACCEPTED | Plain ancestry admits an open commit reachable only via a second parent, leaving the first-parent walk without a terminal | Rule 1 requires first-parent reachability |
| WSS-R1-10 | ACCEPTED | inspectWall's error/violation split (wall.go:122-124) would classify a host-destroyed HEAD as a runner error and loop | The outcome split is by failure class: repository-state failures violate; environment failures stay errors |
| WSS-R1-11 | ACCEPTED | Between-turns checks reading wall.json would trust rewritable evidence; the hash-chained acceptance payload is the only forward authority | Acceptance payload gains headCommitPost, topTreePost, staged verdict (additive, schema 2); WSS-1/WSS-7 updated |
| WSS-R1-12 | ACCEPTED | Confirmed: only `wall-violation` is registered (wall.go:690, event-registry.json:783); no pass event exists | Design names the real event and extends its payload only |
