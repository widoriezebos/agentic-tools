// Package durable is the adapter, the one package that may call File.Sync.
package durable

import "os"

// Sync syncs f.
func Sync(f *os.File) error { return f.Sync() }
