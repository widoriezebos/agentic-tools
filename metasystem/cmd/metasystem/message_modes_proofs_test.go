package main

// Group 4 of the message rewrite: proofs, tests and gates. Each path below
// speaks in the two lines of "Messages a Person Reads". The files left out
// hold machine protocol a parent process reads by its leading code (for
// example internal/testrun/protocol.go, read by the landing owner); they
// say so at their top.
var _ = enforceMessages(
	"internal/candidateengine",
	"internal/enginecause",
	"internal/testenv",
	"internal/testpolicy",
	"internal/testrun/contract.go",
	"internal/testrun/forecast.go",
	"internal/testrun/prepare.go",
	"internal/testrun/rearm.go",
	"internal/testrun/refusal.go",
	"internal/testrun/verify.go",
	"internal/testrun/worker.go",
)
