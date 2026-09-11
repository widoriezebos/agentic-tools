//go:build !darwin && !linux

package proofrun

import "fmt"

type unsupportedProcessTreeReader struct{}

func newProcessTreeReader() processTreeReader { return unsupportedProcessTreeReader{} }

func (unsupportedProcessTreeReader) ProgressRuleSuffix() string { return "" }

func (unsupportedProcessTreeReader) Sample(int) (processTreeSample, error) {
	return processTreeSample{}, fmt.Errorf("process-tree supervision is unsupported on this platform")
}
