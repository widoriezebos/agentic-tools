package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
)

// plantedBatchCommit is the commit boundary of the in-process batch tests:
// the fixture's planted scripts/agents/commit.sh, run with the argv and
// environment the batch transport gave the shell boundary.
func plantedBatchCommit(request landpath.CommitRequest) (string, int) {
	script := filepath.Join(request.Root, "scripts", "agents", "commit.sh")
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
			return output.String(), exit.ExitCode()
		}
		return output.String() + err.Error(), 1
	}
	return output.String(), 0
}

// plantedOrLandingCommit commits through a planted scripts/agents/commit.sh
// when the checkout has one, otherwise through the landing path.
func plantedOrLandingCommit(request landpath.CommitRequest) (string, int) {
	script := filepath.Join(request.Root, "scripts", "agents", "commit.sh")
	if info, err := os.Stat(script); err == nil && info.Mode()&0o111 != 0 {
		return plantedBatchCommit(request)
	}
	return landingPathCommit(request)
}
