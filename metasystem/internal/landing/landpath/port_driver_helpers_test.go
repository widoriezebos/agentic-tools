package landpath

// Helpers of the land driver scenarios ported from the former Bash fixture
// bed (scripts/agents/land-fixtures.sh). Every helper works on one test's
// bed; nothing here is shared between tests.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

// driverInOrder fails unless every part occurs in text, each after the one
// before it.
func driverInOrder(t *testing.T, text string, parts ...string) {
	t.Helper()
	rest := text
	for _, part := range parts {
		index := strings.Index(rest, part)
		if index < 0 {
			t.Fatalf("%q is missing or out of order in:\n%s", part, text)
		}
		rest = rest[index+len(part):]
	}
}

// driverLogOrder fails unless the owner log holds calls starting with each
// prefix, in that order.
func driverLogOrder(t *testing.T, log *ownerLog, prefixes ...string) {
	t.Helper()
	position := 0
	for _, call := range log.calls {
		if position < len(prefixes) && strings.HasPrefix(call, prefixes[position]) {
			position++
		}
	}
	if position != len(prefixes) {
		t.Fatalf("owner order reached %d of %q in %q", position, prefixes, log.calls)
	}
}

// driverPathMode makes the bed's index empty until the landing stages its
// named paths, as a path-mode landing requires.
func driverPathMode(b *bed) {
	b.git.stagedEmpty = true
	b.git.on("add --", func(GitCall) GitResult { b.git.stagedEmpty = false; return ok("") })
}

// driverObserved records every observation request the live judge receives.
func driverObserved(b *bed) *[]ObserveRequest {
	var requests []ObserveRequest
	live := b.owners.Live
	b.owners.Live = func() Judge {
		judge := live()
		observe := judge.Observe
		judge.Observe = func(request ObserveRequest) (landing.Observation, int) {
			requests = append(requests, request)
			return observe(request)
		}
		return judge
	}
	return &requests
}

// driverWriteReceipt writes a landing receipt into the bed's root and
// returns its path.
func driverWriteReceipt(b *bed, name, body string) string {
	b.t.Helper()
	path := filepath.Join(b.root, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		b.t.Fatal(err)
	}
	return path
}

// driverEnvHas reports whether the environment holds entry exactly.
func driverEnvHas(env []string, entry string) bool {
	for _, candidate := range env {
		if candidate == entry {
			return true
		}
	}
	return false
}

// driverEnvNamed reports whether the environment holds any entry for name.
func driverEnvNamed(env []string, name string) []string {
	var entries []string
	for _, candidate := range env {
		if strings.HasPrefix(candidate, name+"=") {
			entries = append(entries, candidate)
		}
	}
	return entries
}
