package durable

import "os"

type TestPolicy = policy

func (p *policy) TestOptIn(isTest bool) error    { return p.useKernel(isTest) }
func (p *policy) TestCurrent(freeze bool) string { return p.current(freeze) }
func SyncAtForTest(level string, f *os.File, full, kernel func(*os.File) error) error {
	return syncAt(level, f, full, kernel)
}

var TestKernelSync = kernelSync
