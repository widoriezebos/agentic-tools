//go:build !darwin && !linux

package proofrun

import "fmt"

func processStoppedForTest(int) (bool, error) {
	return false, fmt.Errorf("process task-state reading is unsupported on this platform")
}
