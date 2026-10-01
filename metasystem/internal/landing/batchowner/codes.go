package batchowner

// The refusal codes this package's errors lead with ("CODE: words"): the
// token machines, records and the refusal register match. A person's
// command strips the code from the words it prints and shows it only with
// --verbose ("Messages a Person Reads").
const (
	codeChangeParentUnknown      = "BATCH_CHANGE_PARENT_UNKNOWN"
	codeChangeUnreadable         = "BATCH_CHANGE_UNREADABLE"
	codeCostAuthorityRefused     = "BATCH_COST_AUTHORITY_REFUSED"
	codeCostHeadroomRefused      = "BATCH_COST_HEADROOM_REFUSED"
	codeCostInputMoved           = "BATCH_COST_INPUT_MOVED"
	codeJoinAdmissionUnavailable = "BATCH_JOIN_ADMISSION_UNAVAILABLE"
	codeJoinAuthorUnbound        = "BATCH_JOIN_AUTHOR_UNBOUND"
	codeJoinRevisionMoved        = "BATCH_JOIN_REVISION_MOVED"
	codeJoinTestDropped          = "BATCH_JOIN_TEST_DROPPED"
	codeJoinUnread               = "BATCH_JOIN_UNREAD"
	codePrefixAuthorityRefused   = "BATCH_PREFIX_AUTHORITY_REFUSED"
	codePrefixPlanRefused        = "BATCH_PREFIX_PLAN_REFUSED"
)
