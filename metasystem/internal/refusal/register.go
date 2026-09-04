// Package refusal holds the refusal register defined by the human-carried-landing design at HCL-AUDIT-03; shell refusals are recorded by hand, and no test proves that list complete.
package refusal

type Shape string

const (
	Identity Shape = "identity" // The machine establishes that the word is the human's.
	Warning  Shape = "warning"  // The machine prints and records the warning, then complies.
	Question Shape = "question" // The word is stale or names nothing, so the machine asks.
	Agent    Shape = "agent"    // The refusal binds an agent's act only; Override is the human verb past it.
)

type Row struct {
	Code     string // The token exactly as the walk collects it.
	Owner    string // The package or script that emits it.
	Site     string // The file and line of one emitting site.
	Shape    Shape
	Override string // The human verb that carries past it; empty only for Identity and Question.
	Commands int    // Commands between the human's intent and the effect; zero when Override is empty.
	Pending  string // "human-carried-landing" while the Override is not yet in the tree.
}

type Exclusion struct {
	Pattern string // An exact token, or "PREFIX_*" for a prefix.
	Reason  string
}

type Shell struct{ Script, Line, Prose, Override string }

type Defect struct{ Code, Why string }

var Rows = []Row{
	{Code: "AGENT_IN_AUTHORITY_CHAIN", Owner: "internal/humanauthority", Site: "authority.go:29", Shape: Identity},
	{Code: "ANCESTRY_CHANGED", Owner: "internal/humanauthority", Site: "authority.go:32", Shape: Identity},
	{Code: "ANCESTRY_CYCLE", Owner: "internal/humanauthority", Site: "authority.go:35", Shape: Identity},
	{Code: "ANCESTRY_UNREADABLE", Owner: "internal/humanauthority", Site: "authority.go:31", Shape: Identity},
	{Code: "ARGV_UNREADABLE", Owner: "internal/humanauthority", Site: "authority.go:33", Shape: Identity},
	{Code: "PROCESS_REUSED", Owner: "internal/humanauthority", Site: "authority.go:34", Shape: Identity},
	{Code: "TERMINAL_NOT_REACHED", Owner: "internal/humanauthority", Site: "authority.go:30", Shape: Identity},

	{Code: "APPROVAL_REQUIRED", Owner: "internal/goal", Site: "approval.go:266", Shape: Agent, Override: "goal approve", Commands: 1},
	{Code: "APPROVAL_EXPIRED", Owner: "internal/goal", Site: "approval.go:301", Shape: Agent, Override: "goal approve", Commands: 1},
	{Code: "RELAY_AFTER_ENROLLMENT", Owner: "internal/goal", Site: "approval.go:329", Shape: Identity},
	{Code: "GOAL_NORM_REFUSED", Owner: "internal/goal", Site: "norm.go:95", Shape: Agent, Override: "goal set-budget then --approved-ref", Commands: 2},
	{Code: "GOAL_SPLIT_REFUSED", Owner: "internal/goal", Site: "split.go:240", Shape: Agent},
	{Code: "SPLIT_RATIFY_REFUSED", Owner: "internal/goal", Site: "split.go:334", Shape: Question},
	{Code: "SWEEP_DUPLICATE_GOAL", Owner: "internal/goal", Site: "approval.go:85", Shape: Question},
	{Code: "SWEEP_INCOMPLETE", Owner: "internal/goal", Site: "approval.go:108", Shape: Question},
	{Code: "SWEEP_LISTING_CHANGED", Owner: "internal/goal", Site: "approval.go:169", Shape: Question},
	{Code: "SWEEP_MALFORMED_ROW", Owner: "internal/goal", Site: "approval.go:45", Shape: Question},
	{Code: "SWEEP_UNKNOWN_GOAL", Owner: "internal/goal", Site: "approval.go:89", Shape: Question},
	{Code: "ELAPSED_LIMIT", Owner: "internal/goal", Site: "file.go:264", Shape: Agent, Override: "goal resume", Commands: 1},
	{Code: "CORRUPT_OVER_LIMIT", Owner: "internal/goal", Site: "file.go:265", Shape: Agent, Override: "goal resume", Commands: 1},

	{Code: "BUDGET_UNKNOWN", Owner: "internal/dispatch", Site: "admission.go:102", Shape: Agent, Override: "goal set-budget", Commands: 1},
	{Code: "BUDGET_REFUSED", Owner: "internal/dispatch", Site: "admission.go:300", Shape: Agent, Override: "goal set-budget", Commands: 1},
	{Code: "HAZARD_REFUSED", Owner: "internal/dispatch", Site: "admission.go:211", Shape: Agent, Override: "goal edit --tier 2 --why", Commands: 3},
	{Code: "RISK_UNANSWERED", Owner: "internal/dispatch", Site: "admission.go:199", Shape: Agent, Override: "goal classify-sweep or goal unapprove, goal edit --risk, goal approve", Commands: 3},
	{Code: "CLOCK_REGRESSED", Owner: "internal/dispatch", Site: "budget.go:275", Shape: Agent, Override: "goal set-budget", Commands: 1},
	{Code: "ADMISSION_CLOSED_ELAPSED", Owner: "internal/dispatch", Site: "budget.go:34", Shape: Agent, Override: "goal resume", Commands: 1},
	{Code: "ELAPSED_BREACH", Owner: "internal/dispatch", Site: "budget.go:35", Shape: Agent, Override: "goal resume", Commands: 1},
	{Code: "BRIEF_AUTHORITY_REFUSED", Owner: "internal/dispatch", Site: "brief.go:34", Shape: Question},
	{Code: "OBLIGATION_REFUSED", Owner: "internal/dispatch", Site: "governed.go:89", Shape: Agent, Override: "goal set-obligation", Commands: 1},
	{Code: "SLICE_CAP_REFUSED", Owner: "internal/dispatch", Site: "slice.go:63", Shape: Agent, Override: "goal approve or goal set-budget, then --approved-ref", Commands: 2},
	{Code: "SLICE_START_UNRECORDED", Owner: "internal/dispatch", Site: "claim.go:366", Shape: Agent, Override: "goal recover", Commands: 1},
	{Code: "slice-approval-refused", Owner: "internal/dispatch", Site: "claim.go:251", Shape: Agent, Override: "goal approve", Commands: 1},

	{Code: "malformed-candidate-tree", Owner: "internal/landing", Site: "observe.go:76", Shape: Question},
	{Code: "candidate-tree-unreadable", Owner: "internal/landing", Site: "observe.go:80", Shape: Question},
	{Code: "missing-declaration", Owner: "internal/landing", Site: "observe.go:89", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "conflicting-declarations", Owner: "internal/landing", Site: "observe.go:83", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "path-unclassified", Owner: "internal/landing", Site: "observe.go:602", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "ledger-path-not-goal-verb", Owner: "internal/landing", Site: "observe.go:586", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "runtime-path-refused", Owner: "internal/landing", Site: "observe.go:591", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "exact-revert-record-refused", Owner: "internal/landing", Site: "observe.go:1082", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "goal-item-not-held", Owner: "internal/landing", Site: "observe.go:617", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "record-not-owned", Owner: "internal/landing", Site: "observe.go:660", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "malformed-chain-id", Owner: "internal/landing", Site: "observe.go:123", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "chain-record-unreadable", Owner: "internal/landing", Site: "observe.go:129", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "chain-record-malformed", Owner: "internal/landing", Site: "observe.go:133", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "chain-not-implementation", Owner: "internal/landing", Site: "observe.go:136", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "chain-not-design-bearing", Owner: "internal/landing", Site: "observe.go:154", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "chain-open", Owner: "internal/landing", Site: "observe.go:157", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "chain-output-unreadable", Owner: "internal/landing", Site: "observe.go:161", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "chain-output-mismatch", Owner: "internal/landing", Site: "observe.go:165", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "chain-has-uncarried-paths", Owner: "internal/landing", Site: "observe.go:199", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "register-carriage-policy-unreadable", Owner: "internal/landing", Site: "observe.go:171", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "register-carriage-path-refused", Owner: "internal/landing", Site: "observe.go:607", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "register-carriage-not-append-only", Owner: "internal/landing", Site: "observe.go:648", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "malformed-revert-commit", Owner: "internal/landing", Site: "observe.go:924", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "direct-fix-policy-unreadable", Owner: "internal/landing", Site: "observe.go:930", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "not-exact-revert", Owner: "internal/landing", Site: "observe.go:971", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "direct-fix-floor-refused", Owner: "internal/landing", Site: "observe.go:577", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "unknown-direct-fix-class", Owner: "internal/landing", Site: "observe.go:940", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "chain-full-gate-refused", Owner: "internal/landing", Site: "observe.go:149", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "promotion-record-malformed", Owner: "internal/landing", Site: "promotion.go:28", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "promotion-base-unreadable", Owner: "internal/landing", Site: "promotion.go:30", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "tier1-declaration-refused", Owner: "internal/landing", Site: "tierone.go:22", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "tier1-policy-unreadable", Owner: "internal/landing", Site: "tierone.go:27", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "tier1-root-refused", Owner: "internal/landing", Site: "tierone.go:42", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "tier1-floor-refused", Owner: "internal/landing", Site: "tierone.go:51", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "tier1-diff-shape-refused", Owner: "internal/landing", Site: "tierone.go:63", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "tier1-file-bound-refused", Owner: "internal/landing", Site: "tierone.go:66", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "tier1-line-bound-refused", Owner: "internal/landing", Site: "tierone.go:69", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "tier1-receipt-refused", Owner: "internal/landing", Site: "tierone.go:73", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
	{Code: "tier1-full-gate-refused", Owner: "internal/landing", Site: "tierone.go:76", Shape: Agent, Override: "land.sh --carried", Commands: 1, Pending: "human-carried-landing"},
}

var Exclusions = []Exclusion{
	{Pattern: "GIT_*", Reason: "git environment variable names"},
	{Pattern: "METASYSTEM_*", Reason: "this tree's environment variable names"},
	{Pattern: "BASH_SOURCE", Reason: "an environment name"},
	{Pattern: "LC_ALL", Reason: "an environment name"},
	{Pattern: "STEWARD_MESSAGE", Reason: "an environment name"},
	{Pattern: "HUMAN_AUTHORITY_PROVEN", Reason: "an authority outcome that admits rather than refuses"},
	{Pattern: "TEMPORARY_HUMAN_WORD", Reason: "an authority outcome that admits rather than refuses"},
	{Pattern: "AUTHENTICATED_CHANNEL_WORD", Reason: "an authority outcome that admits rather than refuses"},
	{Pattern: "AUTO_HEAL_ELIGIBLE", Reason: "a steward health observation that refuses nothing"},
	{Pattern: "AUTO_HEAL_ENDED", Reason: "a steward health observation that refuses nothing"},
	{Pattern: "HEALING_FLAPPING", Reason: "a steward health observation that refuses nothing"},
	{Pattern: "NO_LAWFUL_REMEDY", Reason: "a steward health observation that refuses nothing"},
	{Pattern: "BREACH_STOP_COMPLETE", Reason: "a steward component observation that refuses nothing"},
	{Pattern: "BREACH_STOP_INDETERMINATE", Reason: "a steward component observation that refuses nothing"},
	{Pattern: "BREACH_STOP_OPEN", Reason: "a steward component observation that refuses nothing"},
	{Pattern: "ASSUMPTION_DRIFT", Reason: "a steward component observation that refuses nothing"},
	{Pattern: "BUDGET_MISSING", Reason: "a steward component observation that refuses nothing"},
	{Pattern: "DURABILITY_PENDING", Reason: "a steward component observation that refuses nothing"},
	{Pattern: "INTERRUPTED_BY_NEXT_TURN", Reason: "a steward component observation that refuses nothing"},
	{Pattern: "ENROLLMENT_CHANGED", Reason: "a steward component observation that refuses nothing"},
	{Pattern: "ENROLLMENT_DRIFT", Reason: "a steward component observation that refuses nothing"},
	{Pattern: "NOT_ENROLLED", Reason: "a steward component observation that refuses nothing"},
	{Pattern: "NO_ADAPTER", Reason: "a steward component observation that refuses nothing"},
	{Pattern: "FETCH_FAILED", Reason: "a steward component observation that refuses nothing"},
	{Pattern: "STATE_WRITE_FAILED", Reason: "a steward component observation that refuses nothing"},
	{Pattern: "RENDER_FAILED", Reason: "a steward component observation that refuses nothing"},
	{Pattern: "TICK_FAILED", Reason: "a steward component observation that refuses nothing"},
	{Pattern: "WRITE_FAILED", Reason: "a steward component observation that refuses nothing"},
	{Pattern: "PASS_COMPLETE", Reason: "a steward component observation that refuses nothing"},
	{Pattern: "PASS_FAILED", Reason: "a steward component observation that refuses nothing"},
	{Pattern: "TRANSPORT_FAILED", Reason: "a steward component observation that refuses nothing"},
	{Pattern: "TRANSPORT_SUBMITTED", Reason: "a steward component observation that refuses nothing"},
	{Pattern: "ledger-unavailable", Reason: "a steward component observation that refuses nothing"},
	{Pattern: "LOCAL_PENDING", Reason: "a stop-verdict state after a stop was ordered"},
	{Pattern: "LOCAL_TERMINAL", Reason: "a stop-verdict state after a stop was ordered"},
	{Pattern: "FOREIGN_REPORT_ONLY", Reason: "a stop-verdict state after a stop was ordered"},
	{Pattern: "ALREADY_TERMINAL", Reason: "a stop-verdict state after a stop was ordered"},
	{Pattern: "TERMINAL_DURING_STOP", Reason: "a stop-verdict state after a stop was ordered"},
	{Pattern: "MIGRATION_EPOCH", Reason: "a manifest key"},
	{Pattern: "REVIEWED_SOURCE_SHA256", Reason: "a manifest key"},
	{Pattern: "NON_NEGATIVE_INTEGER", Reason: "a usage-line placeholder"},
	{Pattern: "POSITIVE_INTEGER", Reason: "a usage-line placeholder"},
	{Pattern: "usage-unavailable", Reason: "an adapter verb name rather than a refusal"},
	{Pattern: "READY_FOR_RUNTIME", Reason: "a design-obligation status rather than a refusal"},
	{Pattern: "creator-identity-unreadable", Reason: "a census or custody evidence code rather than a refusal returned to a caller"},
	{Pattern: "custody-entry-malformed", Reason: "a census or custody evidence code rather than a refusal returned to a caller"},
	{Pattern: "group-membership-unreadable", Reason: "a census or custody evidence code rather than a refusal returned to a caller"},
	{Pattern: "index-unreadable", Reason: "a census or custody evidence code rather than a refusal returned to a caller"},
	{Pattern: "indexed-record-unreadable", Reason: "a census or custody evidence code rather than a refusal returned to a caller"},
	{Pattern: "prefork-group-membership-unreadable", Reason: "a census or custody evidence code rather than a refusal returned to a caller"},
	{Pattern: "prefork-marker-unreadable", Reason: "a census or custody evidence code rather than a refusal returned to a caller"},
	{Pattern: "prefork-process-table-unreadable", Reason: "a census or custody evidence code rather than a refusal returned to a caller"},
	{Pattern: "prefork-supervisor-unreadable", Reason: "a census or custody evidence code rather than a refusal returned to a caller"},
	{Pattern: "process-enumeration-unavailable", Reason: "a census or custody evidence code rather than a refusal returned to a caller"},
	{Pattern: "process-table-unreadable", Reason: "a census or custody evidence code rather than a refusal returned to a caller"},
	{Pattern: "recorded-identity-unreadable", Reason: "a census or custody evidence code rather than a refusal returned to a caller"},
	{Pattern: "registry-unreadable", Reason: "a census or custody evidence code rather than a refusal returned to a caller"},
	{Pattern: "tag-position-proof-unavailable", Reason: "a census or custody evidence code rather than a refusal returned to a caller"},
}

var ShellRows = []Shell{
	{Script: "land.sh", Line: "94", Prose: "land refused: unknown option: $1", Override: "land.sh --carried"},
	{Script: "land.sh", Line: "105", Prose: "Usage: scripts/agents/land.sh -m <message-file-or-heredoc> [--goal <id>] [--chain <root-job> [--direct-fix register-carriage] | --direct-fix register-carriage | --direct-fix exact-revert --revert-of <commit> | --direct-fix tier-1 --root-job <job-id> --tests <command>] [--staged-only | <pathspec>...] [--ratchet <path>] [--allow-new-plan] [--skip-transport]", Override: "land.sh --carried"},
	{Script: "land.sh", Line: "107", Prose: "land refused: --staged-only cannot be combined with pathspecs", Override: "land.sh --carried"},
	{Script: "land.sh", Line: "111", Prose: "land refused: name pathspecs or choose --staged-only", Override: "land.sh --carried"},
	{Script: "commit.sh", Line: "234", Prose: "agent commit refused: staged Go package coverage check failed", Override: "land.sh --carried"},
	{Script: "commit.sh", Line: "271", Prose: "agent commit refused: the static re-proof failed (go-gate.sh --fast)", Override: "land.sh --carried"},
	{Script: "commit.sh", Line: "386", Prose: "agent commit refused: the static re-proof failed (audit-metasystem.sh)", Override: "land.sh --carried"},
	{Script: "commit.sh", Line: "450", Prose: "agent commit refused: the landing evaluator failed or returned an incomplete decision ($landing_verdict)", Override: "land.sh --carried"},
	{Script: "commit.sh", Line: "542", Prose: "agent commit refused: the final commit message did not contain exactly one byte-exact Goal-Item stamped by --goal; the commit was rolled back", Override: "land.sh --carried"},
}

var Defects = []Defect{
	{Code: "GOAL_SPLIT_REFUSED", Why: "no human verb clears the sliced mark; the only path is park plus new goals"},
	{Code: "chain-full-gate-refused", Why: "emitted by observe.go:149 and absent from knownRefusalCode (promotion.go:90-122); the promotion reader does not know it"},
}

var Slow = []string{
	"goal edit --tier 2 --why: 3 commands",
	"goal classify-sweep or goal unapprove, goal edit --risk, goal approve: 3 commands",
}
