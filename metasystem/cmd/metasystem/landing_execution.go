package main

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

func landingExecutionSeams(seams plain.ProveSeams) plain.ProveSeams {
	if seams.Executions == nil {
		seams.Executions = func(root string) map[string]enginepin.Execution {
			roles := supervise.ReadExecutions(root, nil)
			roles["runner"] = steward.ReadRunnerExecution(root, nil)
			return roles
		}
	}
	return seams
}
