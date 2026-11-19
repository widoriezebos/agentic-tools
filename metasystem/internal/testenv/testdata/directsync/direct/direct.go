package direct

import (
	"os"

	"example.test/directsync/internal/durable"
)

// Call is a direct call.
func Call(f *os.File) error {
	return f.Sync()
}

// Value passes the method as a function.
func Value(f *os.File) func() error { return f.Sync }

// Expr is the method expression.
func Expr() func(*os.File) error {
	return (*os.File).Sync
}

type handle = os.File

// Alias calls through a local alias of the type.
func Alias(h *handle) error {
	return h.Sync()
}

type wrapped struct{ *os.File }

// Embedded calls the promoted method.
func Embedded(w wrapped) error {
	return w.Sync()
}

// Local calls through a copy of the variable, parenthesised.
func Local(f *os.File) error {
	g := f
	return (g.Sync)()
}

type logger struct{}

// Sync is a logger's own Sync, not the file's.
func (logger) Sync() error { return nil }

// Unrelated calls other Syncs: a logger's, an interface's, the adapter's.
func Unrelated(l logger, f *os.File) error {
	var syncer interface{ Sync() error } = f
	if err := syncer.Sync(); err != nil {
		return err
	}
	if err := l.Sync(); err != nil {
		return err
	}
	return durable.Sync(f)
}
