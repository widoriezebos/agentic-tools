package main

import (
	"errors"
	"io"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

type completeOwners struct {
	cwd      string
	resolver stateroot.Resolver
}

func runComplete(args []string, stdout, stderr io.Writer, owners completeOwners) int { return 0 }

func completionGoalDir(owners completeOwners, repo string) (string, bool) { return "", false }

func nearestGitTop(path string) (string, error) { return "", errors.New("skeleton") }
