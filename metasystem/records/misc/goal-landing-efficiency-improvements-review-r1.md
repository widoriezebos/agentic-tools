# Independent design critique — round 1

Verdict: **2 material findings** (1 HIGH, 1 MEDIUM). Neither requires expanding the design into a new framework. Both have bounded implementation fixtures. No product tests, builds, network access or repository edits were performed.

Reviewed the complete prepared design, SHA256 `63d47268d2c192fd25ca755c751620bbb3c52375f71110013c6144796bd770bb`, against the brief's threat model. Source checks used baseline `92743c95110f8682d84599fb88f9cdf156b9d3b0` in the prepared source directory. Evidence below is **read**; execution outcomes are reasoned counterexamples, not reproduced runtime failures.

## Material findings

### GLE-R1-01 — HIGH — Shared execution can create a circular wait between complete-plan attempts

**Paragraph:** Section 4, “Environment, observations and concurrency,” lines 125–127; Section 6, “Execute distinct missing evidence.”

The proposal combines per-identity reservation immediately before launch, attempts bound to a complete admitted plan, and consumption of another attempt's terminal observation. It records execute/reuse/wait roles but does not specify how those roles change live-observation scanning or prevent cycles while an outer attempt is still incomplete.

**Concrete counterexample:** Two concurrent proof requests both need X and Y. A reserves X; B reserves Y; A discovers B's reservation for Y and waits; B discovers A's reservation for X and waits. Even if both native commands finish, neither complete-plan attempt can terminate until it obtains the other's group. The existing source only exposes results after outer termination. Moreover, if a waiting group's membership in the complete plan still counts as a live reservation, it can hide the producer's completed observation.

**Source evidence:** `metasystem/internal/proofrun/test_result.go:315–351` merges `componentInputs(attempt.ProofIdentity.IdentityInputs)` with `PendingTestGroups`, treats any matching membership in an unterminated attempt as a live observation, and immediately continues without reading group results. The native/reused result scan starts only after the terminal test. This is a current consumer that must change alongside the proposed ownership roles.

**Minimal correction:** State that only the actual producer's reservation is a live execution observation; complete-plan membership and waiting/reused references never claim production. Specify one acyclic ownership rule. A small option is atomic allocation of a request's complete missing set with waits only on earlier owners, rather than acquiring mutually overlapping components incrementally. If dynamic preparation prevents that rule, the alternative must explicitly expose complete native component observations independently of the outer result. Do not silently rely on terminal-only scanning to resolve the cycle. Keep final complete-plan inventory validation separate from execution ownership.

**Acceptance fixture:** Two simultaneous overlapping plans are deliberately interleaved at reservation; each native identity executes once, both requests terminate, a waiting record cannot overshadow its producer, and a failed shared producer returns the same failure to both consumers without an implicit second execution.

**Artifact changed:** The reservation and observation paragraph, the executor implementation contract, and GLE-3's concurrency proof.

### GLE-R1-02 — MEDIUM — The freshness episode has no defined lifetime

**Paragraph:** Section 4, lines 127–129; Section 6, fresh requests and restart/reassembly/moved-trunk behavior.

The reservation key includes a freshness episode and mutable-service checks require a fresh observation for each certification episode, but the design does not define who creates an episode, which prefix decisions share it, or whether restart, reassembly and rebase preserve it. This is not merely naming: an implementer must choose when a retained external-state observation becomes ineligible.

**Concrete counterexample:** A batch obtains a fresh service-health pass, then restarts and is rebased onto a new destination while the service state changes. One conforming interpretation retains the batch ID as its episode and reuses the service observation for the new certification; another creates a new episode for every ensure/verify invocation and repeatedly executes the service check while trying to compose the same receipts. The text requires freshness and efficient restart but does not determine which event ends the original certification.

**Minimal correction:** Define the episode's binding and renewal events in the existing durable plan/attempt records. For example, one admitted certification request owns a durable episode shared by its declared prefix plan set; recovery of that identical request preserves it, while a new explicit forced diagnostic and a new certification after reassembly/rebase get a new episode. State whether unchanged prefix decisions may retain an episode when another member changes. Verification must validate the bound episode rather than mint one. Other choices are defensible, but one must be specified so freshness is a testable contract.

**Acceptance fixture:** A fresh request is interrupted after its group passes and resumed without duplicate execution; a new certification or forced diagnostic executes again despite identical static input hashes; a moved-trunk case follows the explicitly chosen episode-renewal rule.

**Artifact changed:** Freshness/recovery paragraphs and GLE-3/GLE-7 acceptance cases.

## Nonmaterial notes

- No additional prefix-coverage finding: the proposal explicitly recomputes and verifies every final rebased prefix, preserves admitted protection floors, and rejects reliance on the held-range ownership check as proof.
- No additional policy-floor finding: the design preserves the existing protected contract composition and separates reusable execution observations from the current delivery decision.
- A failed shared execution should be consumed as a failure by its waiters; that follows from the existing text's terminal-observation rule. The fixture above makes the rule observable without adding a new retry mechanism.

## Review limits

Only the exact source ranges needed for protection, identity reconstruction, newest-observation handling and rebased verification were read. No performance percentages or runtime correctness were independently measured. Uncited implementation owners and complete budget-accounting code were not audited; this was a bounded design review, not an implementation certification.
