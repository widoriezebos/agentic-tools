package main

import (
	"errors"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
)

// The command's tests share one process-wide home, and a lane registered by
// one test would refuse the next test's landing checkout (U12). Every test
// here therefore runs without a host lane, on each seat's own
// landing.batch-root as before U12; a test of the lane passes its own home
// through the lane seams or the helpers that take one.
func init() {
	batchowner.LandingLaneHome = func() (string, error) {
		return "", errors.New("the command's tests keep no host landing lane")
	}
}
