//go:build !darwin

package durable

import "os"

func kernelSync(f *os.File) error { return f.Sync() }
