package main

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func landingExecutionSeams(seams plain.ProveSeams) plain.ProveSeams {
	if seams.Executions == nil {
		seams.Executions = func(root string) map[string]enginepin.Execution {
			return steward.ReadExecutions(root, nil)
		}
	}
	return seams
}
