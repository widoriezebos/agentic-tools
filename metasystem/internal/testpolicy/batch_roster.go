package testpolicy

// BatchRosterIDs names ordinary acceptance groups selected only by the batch
// planner. The seat and merge gate continue to use the contract's always lists.
func BatchRosterIDs() []string {
	return []string{"batch-audits", "batch-verb-and-bed-coverage", "batch-testpolicy", "batch-source-audits"}
}
