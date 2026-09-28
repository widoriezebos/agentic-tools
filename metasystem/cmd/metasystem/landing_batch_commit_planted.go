//go:build plantedcommit

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
)

// A plantedcommit engine (the batch beds build their fixture engine with this
// tag) commits each unit through the fixture's planted commit
// script when the landing checkout has one, as the batch beds did while the
// commit boundary was scripts/agents/commit.sh; without one it commits through
// the landing path.
func init() {
	batchCommitBoundary = func(request landpath.CommitRequest) (string, int) {
		if output, status, planted := runPlantedBatchCommit(request); planted {
			return output, status
		}
		return landingPathCommit(request)
	}
}

// runPlantedBatchCommit runs root/scripts/agents/commit.sh with the argv and
// environment the batch transport gave the shell boundary.
func runPlantedBatchCommit(request landpath.CommitRequest) (string, int, bool) {
	script := filepath.Join(request.Root, "scripts", "agents", "commit.sh")
	if info, err := os.Stat(script); err != nil || info.Mode()&0o111 == 0 {
		return "", 0, false
	}
	var args []string
	if request.Chain != "" {
		args = append(args, "--chain", request.Chain)
	}
	if request.Attested != "" {
		args = append(args, "--attested", request.Attested, "--attested-snapshot", request.AttestedSnapshot, "--attested-base", request.AttestedBase)
	}
	args = append(args, "--goal", request.Goal, "--test-receipt", request.TestReceipt, "-F", request.MessageFile)
	command := exec.Command(script, args...)
	command.Dir = request.Root
	command.Env = append(append(os.Environ(), request.Env...), "METASYSTEM_LANDED_BY="+request.LandedBy)
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	if err := command.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return output.String(), exit.ExitCode(), true
		}
		return output.String() + err.Error(), 1, true
	}
	return output.String(), 0, true
}
