package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// checkoutEngineRefusal is shared by dispatch and publication so a gate
// cannot replace the deployed engine by reaching a build without dispatch.
func checkoutEngineRefusal(root string, d deps) string {
	if d.getenv(checkoutBuildVariable) == "1" {
		return ""
	}
	if _, err := os.Stat(steward.RepoIdentityPath(root)); err != nil {
		return ""
	}
	delivery, _, err := config.Get(config.GetParams{Key: "metasystem.engine-delivery", ConfPath: filepath.Join(root, "metasystem.conf")})
	if err != nil || delivery != "central" {
		return ""
	}
	return fmt.Sprintf("go-build: this checkout takes its engine from the deploy (central delivery); nothing was built\n"+
		"the steward installs the deployed engine at its next tick while no launch runs, then repeat the command that sent you here; to deploy main's tip: metasystem deploy now\n"+
		"run: %s=1 go run ./cmd/devgate build  (only to build into this checkout anyway)\n", checkoutBuildVariable)
}

// publishEngine consumes a staged engine, preserving a serving checkout's
// deployed engine or atomically replacing the checkout-local engine.
func publishEngine(root, built string, d deps) (bool, error) {
	if checkoutEngineRefusal(root, d) != "" {
		if err := os.Remove(built); err != nil {
			return false, err
		}
		fmt.Fprintln(d.stdout, "bin/metasystem untouched: this checkout takes its engine from the deploy")
		return false, nil
	}
	err := os.Rename(built, filepath.Join(root, "bin", "metasystem"))
	return err == nil, err
}
