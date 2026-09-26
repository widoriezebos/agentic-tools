# Intent workflow iteration log

Goal: verbs-match-intent. User authorization: Wido, 26 September 2026, design,
Fable critique, implementation, and repeated usability improvement; machinery
bypass and ignored stale stop hook persist. This record describes work, not policy.

## Baseline and first design

- Shared checkout: agentic-tools-m1e; preserve its uncommitted receipt/narrator logs.
- Isolated worktree: sibling agentic-tools-intent-workflows-20260926.
- Branch: codex/intent-workflows-20260926; source baseline c8ecd4bb62706c02331f12a4de4ae0025b634568.
- User requested pulling main; both checkouts pulled it. Incoming Partner sitting
  design does not change the CLI owners; its human authority boundaries are retained.
- Observed old executable: 527 help lines, no-argument exit 2. Observed first
  redesign executable: 72 help lines, 48 public commands, no-argument exit 2.
- Source audit found required manual review close/collect, non-idempotent repeated
  revision, internal question/recovery commands, and doubled Partner command names.
- Capability map: 48 current commands plus 13 existing advanced outcomes.
- Draft checkpoint: 98a632446d910b84308b983594322b370fe47346.
- Draft SHA256: 4382ffaea494dad8ae5c39eb76198bd922573a89946f017ac623facd47462bd9.
- Fable critique: launch 20260926t081353-83a8177af2, round 1, running at this entry.
- Design resolver and project check pass. No product implementation or runtime
  change is claimed by this draft checkpoint.

## Additional root questions for the design fold

1. Revision retries after completion must bind a stable prior work version; simply
   recomputing a key from the now-current result can accidentally authorize another
   round. Reusing identical text for a genuine later correction needs an explicit
   public version/decision, not a hidden timestamp or opaque chain id.
2. Compatibility-only flags such as fixture authority and internal session bindings
   must remain parseable for their existing callers while absent from public help
   and suggestions. Public agent recovery must establish its own session through
   the existing owner, never ask an agent to copy another session identity.
3. Review dispositions refer to an exact subject and findings, not just a mutable
   pathname. A retry cannot close a different newly current review with old decisions.
4. Keep ordinary reviewer/author decisions independent even while mechanical close
   and collection disappear from the caller's task sequence.

## Release completion test

All seven design obligations must have concrete owner tests and applicable runtime
proof. Run the complete public human/agent journeys again after implementation;
record each observed gap and its correction. A hidden help entry, renamed wrapper,
or retained internal escape hatch alone does not satisfy the contract. Preserve
an already verified candidate while making later improvements; do not repeat broad
checks for unchanged evidence inputs. Final judgment requires no unresolved material
findings and no demonstrated journey that still needs internal knowledge.

## Design accepted for implementation

Both checkouts pulled main again at 1226c71bf; incoming Partner changes merged
cleanly. Fable reviewed twice on one provider session: material findings 6 then 2.
Root folded all eight, refined C7 to retain authentic unresolved findings before
collection, and closed the final two bounded corrections as named fixture
obligations. Independent code critique remains mandatory. Main contract R126
now also records the user's explicit requirement that internal structure never
be required to complete a task. Product implementation is next.
