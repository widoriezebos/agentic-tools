package main

import "errors"

// The command's tests share one process-wide home, and a lane registered by
// one test would refuse the next test's landing checkout (U12). Every test
// here therefore runs without a host lane, on each seat's own
// landing.batch-root as before U12; a test of the lane passes its own home
// through landingLaneSeams or the helpers that take one.
func init() {
	landingLaneHome = func() (string, error) { return "", errors.New("the command's tests keep no host landing lane") }
}
