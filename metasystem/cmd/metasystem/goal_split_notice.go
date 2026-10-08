package main

import (
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func configureSplitApproval(e *goal.Endpoint, dependencies syncRequestDependencies) {
	e.SplitConfirmed = func(endpoint goal.Endpoint, tip, parent string, now time.Time) {
		projection, err := goal.ProjectAtEndpoint(endpoint, tip, now)
		if err == nil {
			split := projection.Tree.Live[parent].Split
			err = phase.NotifySplitApproval(dependencies.readContext(), endpoint, tip, parent, split.Transaction, split.Children, now)
		}
		if err != nil {
			fmt.Fprintln(dependencies.errStream(), err)
		}
	}
}
