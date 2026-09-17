# Delivery process reset (2026-09-17)

Wido, 2026-09-17 08:45 CEST: "It seems to me like we have burned a shitload of tokens for no
reason at all because we were doing something very stupid. Carefully consider what we should
change, and then change it." This record holds the numbers, the causes, the decisions and the
re-cut of the work in flight. Seat m1e wrote it and made the changes in his name.

## 1. What was spent

Method: every assistant message in the Claude transcripts of this Mac (three seats, their
subagents, the headless design and critique delegates) from 2026-09-15 00:00Z to 2026-09-17
06:40Z, deduplicated by message id, weighted input 1, cache write 1.25, cache read 0.1,
output 5 (input-token equivalents, not money). Codex builds and Codex critiques are billed on
the OpenAI side and are not in these numbers.

| Measure | Value |
|---|---|
| Weighted total | 514M |
| Output tokens | 12.3M (2.4% of raw tokens, 12% of weighted) |
| Cache-read tokens | 3,143M raw, 61% of weighted |
| Cache-write tokens | 110M raw, 27% of weighted |
| Seat main sessions (m1b, m1c, m1e) | 319.5M, 62% |
| Seat subagents (lanes, build chains, reads, designs) | 163.8M, 32% |
| Headless design and critique delegates | 20.8M, 4% |
| Per day | 09-15: 171M; 09-16: 311M; 09-17 to 06:40Z: 31M |
| Seat context per call | 09-15: 338K to 490K; 09-16: 140K to 225K; 09-17: 112K to 116K |
| m1e subagents by role | lanes 33M, build chains 28.5M, chains and lanes by brief file 12.8M, fixes 3.6M, designs 11.5M, reads 11.4M (37 reads, 0.31M each), critiques 7M |
| Overnight window 09-16 22Z to 09-17 08Z | 46M for 22 landed units (2.1M per unit); 4.6M per hour for the fleet against 13M per hour on 09-16 daytime |

Eighty-eight percent of the spend is carrying context, not producing anything. A seat call at
250K context costs the same as writing 5,000 tokens of text; at 115K it costs 2,300.

## 2. Where it went wrong, ranked by cost

1. **Seats far above 200K context.** On 09-15 the three seats averaged 338K to 490K per call
   (2,876 calls above 200K over two days, premium priced). Fixed on 09-16 14:25Z by the 200K
   compaction window and the 105K handoff rule; today's seats run at 115K. Roughly 200M of the
   514M is this cause.
2. **Waiting inside Claude contexts.** Landing lanes and build chains were Claude subagents that
   launched a Codex job or a proof and polled it: 70 output tokens per call, about 78M on m1e
   plus about 30M on m1b and m1c. Builds now launch from a shell script that polls outside any
   model context (17 Sep); landings still run in Claude subagents until the batch verbs and
   branch landing are live.
3. **Relay count.** The 300-line cap in docs/design/design-principles.md (landed 97834d93c on
   09-15 for goal design-allocations-leave-ceiling-margin, opened by seat m1e under the
   authority grant to stop overflows of a 400-line brief ceiling) turned a 3,000-line goal into
   24 units. Each unit is one relay through the coordinator: a brief with seat rulings (BA9a1's
   alone were 80 lines), a build, an independent read, a fix round, a stack, a proof and a
   landing; about 80 seat calls and two subagents, about 2M weighted per unit at today's context
   size. The token cost of the relays is 25M to 35M per goal; the wall-clock cost is 24 serial
   handoffs through one seat: 16 hours for the first eleven units of batch landing.
4. **Design rounds that cannot converge.** A critic briefed to attack a design returns "material"
   findings every round: 16, 13 and 10 for batch landing, 16 and 10 for goal branches. The rule
   "rounds continue until a round has zero material findings" never terminates. Of the ten
   round-2 findings on goal branches, four change what gets built, one is a process artifact, one
   is a known defect with its own goal, four are witness bookkeeping the builder does anyway
   (builder-proves-each-rule-by-mutation). The seven hours of design for batch landing were 2.5
   hours of model time, 1.3 hours of a session-limit outage (ten parallel folds) and 2 hours of
   the seat writing briefs by hand.

What was not the cost: Fable designs (24M, 5%), Opus reads (23M, 4.5%), the Claude side of
critiques (10M, 2%). Cutting those saves nothing. Cutting relays and waits saves most of what is
left, and the context fix already took out the largest part.

Every rule in causes 3 and 4 was opened by the seats as an efficiency measure between 09-13 and
09-15 (design-allocations-leave-ceiling-margin, brief-declares-the-round-boundary,
builder-proves-each-rule-by-mutation, critique-always). Each was a defensible reaction to one
failure; nobody totalled them, and together they run on every unit.

## 3. Decisions

D1. **A unit is one design section.** Normally 600 to 1,500 changed lines, the files, behaviour and
tests one builder produces in one job. The allocation on the brief is an estimate, not a cap. A
builder that finds the unit does not hold together returns a split proposal (a gap-stop) instead
of trimming tests; the seat never pre-splits a unit to fit a number. Written into
docs/design/design-principles.md (Implementation Slicing) and scripts/agents/templates/brief.md.

D2. **The design page is the brief.** A unit brief adds workspace, inputs, return shape and proof
group, and restates no rule of the page. If the page needs seat rulings to be buildable, the page
is not done: one fold, then build. Written into the brief template.

D3. **One design critique round.** A material finding shows that the design cannot be built as
written or would build the wrong behaviour, and names the artifact. Witness bookkeeping, naming
and record-format findings are notes for the builder's brief, not rounds. A second round runs only
after a fold that changed a rule; there is never a third. An amendment that records a human
ruling gets no round: the ruling is the authority and the builder proves the fold. Written into
docs/orchestration.md.

D4. **One independent read per build,** in a fresh session of about 0.3M weighted, of the whole
build; a fix round re-reads the fix only. The proof gates main. Written into docs/orchestration.md.

D5. **No Claude context waits.** A delegate that launches a job, a proof or a landing returns at
once; a shell or Go waiter re-invokes the seat when the record is durable. Written into
docs/orchestration.md; the Go form is goal waits-run-outside-model-contexts.

D6. **Seats keep the 200K window and the 105K handoff.** Unchanged; it is the fix that worked.

D7. **Lane rules amended.** A lane opens at two ready builds or 90 minutes since the first went
ready; the number held is changed lines per proof (target 2,000 or more) until goal branches
land, then goals per proof.

D8. **The work in flight is re-cut** (section 4). Nothing built is discarded; the units in flight
finish as briefed.

## 4. The re-cut

| Goal | Landed or in flight | Builds from here |
|---|---|---|
| goals-live-on-branches (m1c) | unit 1 building (Codex) | A = units 2, 3, 4 (about 830 lines). B = units 5, 6, 7a, 7b (about 910). C = units 8, 9, 10a, 10b, 11 (about 750, after batch build C). Builder reads the page; the round-2 critique dispositions are on the page. |
| units-land-in-batches-under-one-proof (m1e) | BA0 to BA8, BA10, BA16 landed; BA9a1 and BA5c building | B = BA9a2, BA9b, BA11a, BA11b (about 1,000). C = BA12a, BA12b, BA17, BA13 (about 870). D = BA14a, BA14b, BA15 (about 780). |
| red-on-main-gets-an-owner-on-the-ledger (m1b) | U0 landed; U2a built, read running | m1b re-cuts the remaining units into section builds the same way. |
| fixture-children-cannot-outlive-their-test (m1c) | 5b0 to 5e landed; 5f built | parks after 5f with trigger "goals-live-on-branches unit 3 landed"; 5g and units 6 to 12 become two builds on goal/fixture-children. |

## 5. What is measured from here

- Hours from goal open to goal landed (target under 8 for a 3,000-line goal; batch landing was 16
  for its first third).
- Weighted tokens per landed changed line (overnight: about 7,000; target under 2,000).
- Seat calls per landed build.

The durable form of this report is goal spend-fence-reports-tokens-per-model-and-cause (priority
1). The scan scripts live in seat m1e's scratchpad until then.
