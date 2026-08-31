Working Mode: design
Orchestrator Identity: m3 (lineage mac-m3, dispatch delegate under goal alert-escalation-channel, overnight envelope R-38-m3)
Date: 2026-08-31

# Goal

Revise plans/alert-channel-design.md to answer the Sol critique below
(job design-critic-2ea477763ee73bd1dbc0ddec, one round, all verdicts
failed, one critical). The critique is relayed whole per R-25b-m1 —
every fold or refusal is yours.

# New human words since your draft (bind the revision)

Wido, verbatim, in order given:
1. Telegram is CONFIRMED as the first example implementation — your
   flagged assumption resolved in your favor.
2. "We can use the same mechanism for the session bridge too, so there
   is a bit of reuse there" — the contract bears a second consumer,
   runtime-agnostic seat-to-seat messaging (goal seat-mutual-awareness).
3. "Another one would be slack, which has threaded messages. that also
   needs to fit the design of the alert channel and session bridge" —
   the contract must carry conversation/thread identity: a threaded
   adapter threads naturally (an episode's alert, updates, and
   acknowledgment one thread; a seat exchange one thread), flat
   adapters degrade honestly, no per-adapter leakage into call sites.

Note the convergence: the critique's critical finding (AC-CONTRACT-001)
and Wido's words 2-3 point at the same structural gap from two sides.

# Workspace

Your prior worktree. Revise exactly one file:
plans/alert-channel-design.md. A rejected finding needs a recorded
refutation with evidence in the design's text; an accepted one changes
the design.

# Constraints

- Wall-clock budget: 20 minutes (this claim's remaining pool). Fold in
  severity order: the critical contract finding with Wido's words 2-3
  first, then the two never-blocking findings, then state/receipt/
  dedup/credential, then slice-plan. If the budget cannot carry a
  sound complete fold, an HONEST PARTIAL beats a rushed whole: say in
  the design's status line exactly which findings remain unfolded; the
  orchestrator re-claims for the remainder.
- Keep the R-24-m1 self-grade current: fresh weakest claim and reject
  condition.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/alert-channel-design.md; whatWasDone names each
finding folded, refuted, or left for the next round.

# Gap Rule

stop and report a gap; never fill it silently.

## Verdicts and self-grade

### Threat-model verdicts and R-24-m1 self-grade
Contract soundness: fail—the contract needs changes, not merely a new seat caller. Never-blocking law: fail—the tick and alert lock wait synchronously, and the legacy gate split is unspecified. State honesty: fail—the pending queue and two digest cursors are independent delivery state. Deduplication: fail—the current episode update resolves unrelated findings and the four producer identities do not share a traced lifecycle. Credential and failure story: fail—secret-bearing transport errors are not required to be redacted, and the failure floor has no slice owner. Slice plan: fail—Telegram is the correct first adapter, but slice one omits required gate, floor, and enforcement work, while later adapter contracts remain deferred. Critic confidence is 0.94. The weakest finding is the breadth of the cross-provider message-size concern because Meta's WhatsApp documentation was unavailable; Telegram alone still proves the unresolved multipart outcome. Reject the second-consumer finding only if the human intended reuse of merely the registry factory while permitting a separate addressed seat contract, which conflicts with this round's explicit instruction that the adapter contract itself bear both consumers.

## Findings, verbatim

### AC-CONTRACT-001 (critical, material=True)

CLAIM: The two-consumer contract is structurally insufficient. This refutes “One interface, deliberately minimal” and “Call sites call Send and never name an adapter.” The only classes are alert and digest; Message contains human-alert fields but no sender, recipient seat, correlation identifier, reply route, or deadline; configuration fixes one destination per class. The seat-mutual-awareness consumer requires addressed, bidirectional questions for arbitrary runtimes. Using the same adapters therefore requires contract changes and receive/reply integration, not merely a new caller; otherwise callers must leak destination and adapter behavior.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 68–89 and 126–136 define the closed human-oriented shape. metasystem/plans/goals/seat-mutual-awareness.md lines 3–6 require direct questions in both directions, assertable seat identity, response commitments, deadlines, and runtime independence. metasystem/docs/seat-communication.md lines 1–8 explicitly exclude agent-to-agent traffic from the human message rules embedded in Message. Slack's official incoming-webhook scope also describes that adapter as one-way and tied to a specific channel.

### AC-BLOCK-001 (high, material=True)

CLAIM: The never-blocking law is false in the proposed control flow. This refutes “an adapter that cannot complete within the existing 15-second notify timeout is a failed attempt, never a wedged tick” and “no tick ... waits on a send.” A 15-second timeout bounds waiting; it does not remove it. Retaining the synchronous Send signature and replacing Deliver internally preserves a path where the steward tick and every contender for the alert lock wait on network delivery.

EVIDENCE: metasystem/internal/steward/alert_episode.go lines 231–360 acquire the exclusive alert lock, invoke Deliver at line 341, and release the lock only after completion is journaled. metasystem/internal/steward/notify.go lines 20–58 execute CombinedOutput synchronously under a 15-second context. metasystem/internal/steward/tick.go lines 240–266 synchronously calls UpdateAlertEpisodes before completing the tick. AcknowledgeAlert also needs the same exclusive lock at metasystem/internal/steward/alert_episode.go lines 364–391.

### AC-BLOCK-002 (high, material=True)

CLAIM: The claimed split preserving the legacy delivery-gated launch behavior is not representable by the proposed interface. This refutes “the one existing delivery-gated behavior ... is preserved for its current callers and NOT extended to any new alert class.” The existing gate asks whether NotifyCommand is available before launch; Channel exposes only a side-effecting Send. Keeping the old check makes a configured Telegram channel fail the Linux launch gate unless the legacy command is also configured. Replacing it with credential validation makes the design choose, but never specifies, whether an unconfigured external adapter refuses launch or degrades harmlessly. Probing with Send would itself wait and emit a message.

EVIDENCE: metasystem/internal/steward/runner.go lines 213–222 and 375–388 call NotifyCommand before EnsureRunner and arm. metasystem/plans/alert-channel-design.md lines 161–166 require missing credentials not to stop a launch, while lines 301–305 promise preservation of the old refusal. Channel at lines 71–90 has no readiness or non-side-effecting availability operation and no caller policy that separates legacy gating from new alerts.

### AC-STATE-001 (high, material=True)

CLAIM: The design leaves a durable retry queue and receipt path outside the episode store. This refutes “Adapters hold NO state, do NO retries, keep NO queue” and “The alert episode store is authoritative.” Existing revival failures, reaping notices, and other retained DeliverPending callers continue to use PendingNotification files; slice two redirects only narrator noticings. Those queue entries disappear on success and have no episode attempt receipt, so an implementer following the stated preservation rule leaves two delivery-state owners.

EVIDENCE: metasystem/internal/steward/intervene.go lines 281–348 defines artifacts/agents/steward/pending as a durable queue and deletes entries after delivery. metasystem/internal/steward/runner.go lines 113–133 queues revival failures and retries that queue. metasystem/internal/steward/reap.go and metasystem/internal/steward/tick.go also call QueueNotification. metasystem/plans/alert-channel-design.md lines 323–326 migrates only narrator noticings, while lines 37–46 and 301–305 preserve current Deliver callers.

### AC-RECEIPT-001 (high, material=True)

CLAIM: Primary-plus-fallback receipts cannot be represented honestly by the stated result and schema. This refutes “The fallback attempt is journaled with its own Channel name.” Send returns only nil or one error, and AlertAttempt gains only one Channel field beside one result and one problem. A primary failure followed by fallback success has two transport outcomes. Recording only the fallback loses the primary failure; recording one failed attempt makes the episode look undelivered; appending two attempts changes sequence, pending-recovery, and retry semantics that the design does not specify.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 71–90 defines the single error result; lines 225–233 adds only one Channel field; lines 274–285 requires separate primary and fallback journaling. The shipped path at metasystem/internal/steward/alert_episode.go lines 324–355 creates one pending AlertAttempt around one Deliver call and completes that single record with one result and problem.

### AC-DEDUP-001 (high, material=True)

CLAIM: A stable class-and-subject digest is not enough to support the four blocked-state producers, and the claimed shared store law would prematurely resolve live alerts. This refutes “one actionable state, one episode” and “the state clearing ... resolves the episode through the same store law that health verdicts use today.” The current health-specific update marks every different open digest resolved when a new finding arrives, so two simultaneous blocked states cannot coexist. Producer tracing also shows no uniform lifecycle: stopped goals have stop identifiers and fence coordinates; mission asks have ask identifiers, but free-form runtime dialogs do not; enrollment drift is an ephemeral up result with no clear event; and the example goal approve/reject commands do not exist.

EVIDENCE: metasystem/internal/steward/alert_episode.go lines 270–279 resolves every nonmatching open episode. metasystem/internal/goal/stop.go lines 29–50 and 345–397 supplies stop, revision, fence, and human-only resume coordinates. metasystem/internal/missionrunner/answer.go lines 14–65 uses durable ask identifiers. metasystem/internal/up/up.go lines 391–420 returns ENROLLMENT_DRIFT without a durable lifecycle. Searches under metasystem/cmd/metasystem and metasystem/internal/goal found no goal approve or goal reject verb, contradicting the exact answering act shown at metasystem/plans/alert-channel-design.md lines 208–214.

### AC-STATE-002 (high, material=True)

CLAIM: The proposed digest cursor is irreducible second delivery state and conflicts with an existing consumer cursor. This refutes “The cursor is bookkeeping, not a second state: it is rebuildable, and losing it costs at worst one repeated batch.” The register cannot reveal which prefix an external transport submitted; losing that fact cannot be rebuilt from the register. Moreover, the Stop hook already advances a different cursor after showing the same register to a human. Two cursors create two independent definitions of delivered, while reusing one would let either consumer consume entries before the other sees them.

EVIDENCE: metasystem/internal/narratordigest/digest.go lines 39–49 and 201–282 defines the existing byte-offset and prefix-hash cursor. metasystem/scripts/agents/supervision-hook.sh lines 165–176 and 203–207 reads and advances it after Stop payload emission. metasystem/plans/alert-channel-design.md lines 179–188 proposes a new timestamp cursor at a different path, without a reconciliation or multi-consumer receipt law. With no cursor, the existing implementation starts at byte zero, which can repeat the entire retained register rather than one batch.

### AC-CREDENTIAL-001 (high, material=True)

CLAIM: The credential story can persist secrets through failure text. This refutes “credentials live ONLY in metasystem.conf.local or the environment” as a complete enforceable story. Telegram places the bot token in the request URL, Slack defines the webhook URL itself as secret, and the design journals each adapter's problem string without requiring redaction. A routine HTTP or command error that includes its URL or output can therefore copy credentials into episode JSON, health output, or logs even when configuration storage is correct.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 143–159 protects only configuration placement, while lines 161–166 and 274–278 persist adapter problem strings. Telegram's official Bot API documents URLs of the form https://api.telegram.org/bot<token>/METHOD. Slack's official incoming-webhook guide says the webhook URL contains a secret and must not be shared. The current notifier at metasystem/internal/steward/notify.go lines 55–57 already embeds command output in its returned problem, demonstrating why the adapter contract needs an explicit sanitized-error invariant.

### AC-SLICE-001 (medium, material=True)

CLAIM: The loudly-but-harmless failure floor has no implementation owner in the slice plan. This refutes the complete failure story and the claim that the slices are independently deployable. The design promises new health and Stop-hook lines, but no slice includes those changes or their acceptance tests. The committed-secret rule is deferred until slice five and enters only marking mode, with no activation slice or criterion. Slice one can therefore ship a live Telegram token and failed external sends while the promised floor and refusal remain absent.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 286–299 specifies health and Stop-hook surfaces. Lines 312–338 enumerate five slices but name neither surface in any slice; the secret rule appears last and only in marking mode. The current health verb at metasystem/cmd/metasystem/steward_verbs.go lines 39–74 prints only the health verdict, and the current Stop hook at metasystem/scripts/agents/supervision-hook.sh lines 160–176 composes health plus narrator digest, not failed episode counts. Telegram as the first adapter is confirmed and is not disputed.

### AC-CONTRACT-002 (medium, material=True)

CLAIM: The one-message contract does not yet support even the confirmed Telegram digest path, and the later adapters are not proven configuration-only. This refutes “send one composed message” and the fake-endpoint acceptance test as sufficient for all named targets. Telegram limits sendMessage text to 4096 characters, while the digest register and four-hour batch have no size bound. The design does not choose rejection, truncation, or multipart submission; multipart partial success would again exceed the single success-or-error result. Exact email, Slack, and WhatsApp settings and behavior are deferred, and slice four explicitly requires new engine code, so a fake endpoint with no call-site diff cannot prove provider constraints or configuration completeness.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 68–90 defines one composed message and one result; lines 179–188 permits an unbounded batch; lines 132–136 defers exact adapter settings; lines 331–335 requires three later registry implementations. Telegram's official Bot API specifies 1–4096 characters after entity parsing. metasystem/internal/narratordigest/digest.go lines 109–148 appends without a total-size ceiling. This forces an outcome and retry decision before Telegram plus digest is deployable.