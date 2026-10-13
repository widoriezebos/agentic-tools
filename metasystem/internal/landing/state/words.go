package state

const (
	Running, Paused, Stopped                                              = "running", "paused", "stopped"
	Idle, Stepping, StepProcessGone, Ended                                = "idle", "stepping", "step-process-gone", "ended"
	Prepared, Closed, Open, Sealed                                        = "prepared", "closed", "open", "sealed"
	Waiting, Returned, Landed, Superseded                                 = "waiting", "returned", "landed", "superseded"
	Resolving, Resolved, Building, Reviewing, Done                        = "resolving", "resolved", "building", "reviewing", "done"
	Died, Pending, Failed, Green, Red, Unproven, Held, Skipped, Cancelled = "died", "pending", "failed", "green", "red", "unproven", "held", "skipped", "cancelled"
)
