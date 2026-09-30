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
	codeLandAuthorUnbound        = "BATCH_LAND_AUTHOR_UNBOUND"
	codeLandCleanupRefused       = "BATCH_LAND_CLEANUP_REFUSED"
	codeLandCommitRefused        = "BATCH_LAND_COMMIT_REFUSED"
	codeLandDelegationRefused    = "BATCH_LAND_DELEGATION_REFUSED"
	codeLandHeldRefused          = "BATCH_LAND_HELD_REFUSED"
	codeLandPushRefused          = "BATCH_LAND_PUSH_REFUSED"
	codeLandReceiptRefused       = "BATCH_LAND_RECEIPT_REFUSED"
	codeLandStateRefused         = "BATCH_LAND_STATE_REFUSED"
	codeLandTrunkMoved           = "BATCH_LAND_TRUNK_MOVED"
	codeLandUnwired              = "BATCH_LAND_UNWIRED"
	codeLandingBranchMoved       = "BATCH_LANDING_BRANCH_MOVED"
	codeLandingBranchPrepRefused = "BATCH_LANDING_BRANCH_PREP_REFUSED"
	codeP6Refused                = "BATCH_P6_REFUSED"
	codePrefixAuthorityRefused   = "BATCH_PREFIX_AUTHORITY_REFUSED"
	codePrefixDecisionMoved      = "BATCH_PREFIX_DECISION_MOVED"
	codePrefixProofRefused       = "BATCH_PREFIX_PROOF_REFUSED"
	codeProofInputMoved          = "BATCH_PROOF_INPUT_MOVED"
	codeProofNotAdmitted         = "BATCH_PROOF_NOT_ADMITTED"
	codeProofStaleCompletion     = "BATCH_PROOF_STALE_COMPLETION"
	codeProofStateRefused        = "BATCH_PROOF_STATE_REFUSED"
	codeProofUnionUncovered      = "BATCH_PROOF_UNION_UNCOVERED"
	codeFlakeAllowanceRefused    = "BATCH_FLAKE_ALLOWANCE_REFUSED"
	codeReassembleMoved          = "BATCH_REASSEMBLE_MOVED"
	codeRecoveryNotPushed        = "BATCH_RECOVERY_NOT_PUSHED"
	codeTrunkRedOwnerUnbound     = "TRUNK_RED_OWNER_UNBOUND"
	codeTrunkRedRecordPending    = "TRUNK_RED_RECORD_PENDING"
	codeWaitBound                = "BATCH_WAIT_BOUND"
)
