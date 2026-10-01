package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

func TestResolveExplicitBatchLandingRejectsInvalidRoots(t *testing.T) {
	seat := t.TempDir()
	now := func() time.Time { return time.Unix(1, 0) }
	for _, test := range []struct {
		name string
		root string
		want string
	}{
		{name: "relative", root: "relative", want: "absolute"},
		{name: "missing checkout", root: filepath.Join(t.TempDir(), "missing"), want: "existing checkout"},
		{name: "seat checkout", root: seat, want: "non-seat checkout"},
		{name: "ordinary directory", root: t.TempDir(), want: "existing checkout"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := config.ResolveExplicitBatchLanding(test.root, seat, time.Minute, now); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("explicit root %q error=%v, want %q", test.root, err, test.want)
			}
		})
	}
}
