# Design brief: people shape goal splits in the app

Working Mode: Design. Proposed follow-up of `goals-are-shaped-small-with-a-person`; this brief opens no ledger goal. Revised 2026-10-08 after source critique round 1. Read the source draft and `goal-splits-apply-and-can-be-reversed-design-brief.md`. Depends on the source's working Markdown-file split, lineage and unstarted reverse effects. At most five units of at most 250 production lines; include TypeScript, styles, schemas and generated runtime assets, estimating bundle changes separately.

## Destination of the moved scope

**This brief is the destination for the versioned proposal store, proposal revision history, operation-id replay for proposal creation/editing, and the browser**, moved from the source's former S1/S2 under “Decided by m1e for Wido”, 2026-10-08, reversible. The source builds acceptance and lineage first. Before this follow-up, a person shows and edits the ordinary Markdown member file and applies it with `goal split G --plan FILE`; there is no required proposal store and no `propose` verb. Existing goal-publication recovery stays with the goal transaction, distinct from future proposal-edit replay.

Do not rebuild split state, approval notification, basic application or unstarted reversal here. No reverse dependency from the source: this follow-up consumes its goal state and effect verbs. The automation brief extends those effects for committed stops and work already started; browser controls for those extensions wait for their real verbs.

## Intent and first usable result

A person sees proposed children as a tree, shapes them with sizes and scope visible, and invokes the already-working split effect. Afterward the same view shows lineage, approval and dependencies. Design the smallest useful read-only tree first, from an ordinary member file and existing `goal show` projection. Decide separately whether the next editing slice needs the retained store below; storage is not an admission prerequisite for the source's plain-file command.

Baseline code to reread when designing: source origin/main `91f42e0aa3a3f8b78718e4b76cd3a78cf6dd2e29`, `internal/goal/split.go:24`, `:63`, `:158` own the member grammar; `cmd/metasystem/intent_planning.go:293`, `:2039` own its public form. Inspect current `internal/ui/project/`, `internal/ui/act/` and Partner proposal owners before choosing browser adapters. Use existing document/section rendering and signed-in person authority, not a second HTTP goal transaction or generic shell endpoint.

## Retained proposal requirements for the editing slice

The former draft proposed one `SplitProposal` owner under the resolved state root, with id/revision, parent goal/revision, author/time, origin (person, author, committed exit), exact retained source identities/bytes/hashes, stable child keys, proposed ids/intents, risk answers/basis, unit estimates, elapsed estimate, areas, assigned sections, prerequisite keys/goals, explicit exclusions/reasons and draft/withdrawn state. Preserve these requirements while challenging their minimal implementation; no approval derives from a planning record. Goal transactions remain the sole application owner.

- Keep a single current revision with inspectable prior edits and submitted conflicting changes. Operation-id replay of identical proposal edits returns the same result; conflicting contents under that id are explicit. No new general event store. Existing atomic-file and lock owners handle publication; failed writes never report success.
- Bind section identity to retained source bytes and occurrence/range, or a committed exit's existing stable ids. Every requirement is assigned once or visibly excluded; shared context is non-owning. Unknown estimates/source agreement are shown as unknown. Drafts may be incomplete or oversized while a person shapes them.
- Edit acts cover intent, rename, move section, merge children, prerequisites, drop with reason, full replacement and withdraw/restore. Merge remaps dependency edges; dropping retains excluded requirements; ordering is a dependency decision, not a display trick. Preserve both sides of a stale revision conflict and offer an explicit current-revision retry.
- A proven person's edit is retained despite unreadable advisory source data, with impact explained. An agent may not silently overwrite newer edits or rebind changed headings by guesswork. Retained pending intent on physical failure is not a second current-state selector or a false success.
- Reuse the Markdown `--plan` grammar at the effect boundary. If rich proposals require a separate machine representation, give it a distinct documented input rather than making the same flag mean JSON on propose and Markdown on split. Name public show/edit/accept forms in this follow-up's design only when they have production callers.

## User flow and public-act parity

1. The goal page opens a tree showing source, child intent, sizes/count against current caps, prerequisites, areas, unknowns and excluded scope. The server's existing size owner supplies observations; JavaScript does not recalculate admission. Nested display represents lineage, not scheduling authority.
2. Editing controls call the proposal owner's public counterparts once designed. Keyboard actions offer every drag-style operation. Preserve the displayed revision and pending edits on conflict. Partner text may prepare actions but cannot impersonate the person's press.
3. “Accept this split” shows the existing effect's impact and calls it through signed-in authority. Children remain unapproved and blocked by the source. Display the actual approve and person unblock actions; never suggest approval alone clears the block. Unknown advice does not disable a person's existing override, and physical failure remains pending with the real recovery action.
4. Reloaded lineage comes from current goal projection, not a cached proposal. Each child has its own brief, approval/budget and complete-goal landing. A browser-only lineage or approved badge is forbidden.
5. “Reverse split” initially maps to `goal split G --reverse --reason TEXT` while children are unstarted. Started-work reversal displays the retained decision/pending reconciliation from the automation owner only when implemented; it does not claim the bounded source verb has undone started work. A successor scope edit keeps original decisions inspectable.

Messages give the situation first and the next act second. Use existing Partner grants and accessible controls. No new graph library is authorized.

## Size, proof and boundaries

The former **450–650 production-line estimate covered only browser work**. It does not include the transferred proposal store/history/replay, formerly estimated as **480 production lines** in the source's old S1/S2. These historical estimates describe scope, not an approved combined allocation. Re-estimate the smallest useful tree from landed code, then split later storage/editing/interactive scope into further briefs before a unit exceeds 250 or the goal exceeds five. Include generated bundle changes honestly; do not hide them in tests.

Each eventual unit's scenario drives the real route and act owner with fake transport/time only. Prove fresh tree display, scope-preserving edits, stale conflict recovery, one committed split, separate approval/unblock, pending publication recovery and appropriate reversal. Mutations include direct browser writes, omitted revision, false approved state, hidden unknowns, disabled person override, duplicate children on replay and an unstarted-only reverse presented as undoing code. Interactive units require browser end-to-end proof; each unit runs new/changed tests and mutations, with one full suite at goal landing.

Answer the five questions for every new owner, read model and act, and exclude the four defect classes: ineffective remedy, borrowed authority, stale state and tests hiding the production boundary. Lineage/effects, budget derivation, split policy, approvals and convergence keep their domain owners; this goal presents and shapes their input.
