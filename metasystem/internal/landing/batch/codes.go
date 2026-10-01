package batch

// The refusal codes this package's errors lead with ("CODE: words"): the
// token machines, records and the refusal register match. A person's
// command strips the code from the words it prints and shows it only with
// --verbose ("Messages a Person Reads").
const (
	codeChangeUnreadable         = "BATCH_CHANGE_UNREADABLE"
	codeCostInputMoved           = "BATCH_COST_INPUT_MOVED"
	codeJoinPending              = "BATCH_JOIN_PENDING"
	codeJoinTestDropped          = "BATCH_JOIN_TEST_DROPPED"
	codeLandCleanupRefused       = "BATCH_LAND_CLEANUP_REFUSED"
	codeLandTrunkMoved           = "BATCH_LAND_TRUNK_MOVED"
	codeLandingBranchMoved       = "BATCH_LANDING_BRANCH_MOVED"
	codeLandingBranchPrepRefused = "BATCH_LANDING_BRANCH_PREP_REFUSED"
	codeP6Refused                = "BATCH_P6_REFUSED"
	codePrefixAuthorityRefused   = "BATCH_PREFIX_AUTHORITY_REFUSED"
	codeReassembleMoved          = "BATCH_REASSEMBLE_MOVED"
	codeRecoveryNotPushed        = "BATCH_RECOVERY_NOT_PUSHED"
)
