package batchowner

// The refusal codes this package's errors lead with ("CODE: words"): the
// token machines, records and the refusal register match. A person's
// command strips the code from the words it prints and shows it only with
// --verbose ("Messages a Person Reads").
const (
	codeBaseMoved                = "BATCH_BASE_MOVED"
	codeChangeParentUnknown      = "BATCH_CHANGE_PARENT_UNKNOWN"
	codeChangeUnreadable         = "BATCH_CHANGE_UNREADABLE"
	codeCostAuthorityRefused     = "BATCH_COST_AUTHORITY_REFUSED"
	codeCostHeadroomRefused      = "BATCH_COST_HEADROOM_REFUSED"
	codeCostInputMoved           = "BATCH_COST_INPUT_MOVED"
	codeJoinAdmissionUnavailable = "BATCH_JOIN_ADMISSION_UNAVAILABLE"
	codeJoinAuthorUnbound        = "BATCH_JOIN_AUTHOR_UNBOUND"
	codeJoinPending              = "BATCH_JOIN_PENDING"
	codeJoinRevisionMoved        = "BATCH_JOIN_REVISION_MOVED"
	codeJoinTestDropped          = "BATCH_JOIN_TEST_DROPPED"
	codeJoinUnread               = "BATCH_JOIN_UNREAD"
	codeLandPushRefused          = "BATCH_LAND_PUSH_REFUSED"
	codeLandTrunkMoved           = "BATCH_LAND_TRUNK_MOVED"
	codeOwnerIdentityUnknown     = "BATCH_OWNER_IDENTITY_UNKNOWN"
	codeOwnerIndeterminate       = "BATCH_OWNER_INDETERMINATE"
	codeOwnerOwnedElsewhere      = "BATCH_OWNER_OWNED_ELSEWHERE"
	codePrefixAuthorityRefused   = "BATCH_PREFIX_AUTHORITY_REFUSED"
	codePrefixDecisionMoved      = "BATCH_PREFIX_DECISION_MOVED"
	codePrefixPlanRefused        = "BATCH_PREFIX_PLAN_REFUSED"
	codePrefixProofRefused       = "BATCH_PREFIX_PROOF_REFUSED"
	codeProofStateMoved          = "BATCH_PROOF_STATE_MOVED"
	codeProofStateRefused        = "BATCH_PROOF_STATE_REFUSED"
	codeProofTreeMoved           = "BATCH_PROOF_TREE_MOVED"
)
