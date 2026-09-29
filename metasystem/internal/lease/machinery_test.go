package lease

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestMachineryAncestorWalksTheWholeChain: a supervision or adapter-supervisor
// ancestor anywhere above the caller is machinery, including above the
// caller's parent; none is not; an unreadable custody record is an error.
func TestMachineryAncestorWalksTheWholeChain(t *testing.T) {
	t.Parallel()
	self := int64(os.Getpid())
	parent, ok := ParentPid(self)
	if !ok {
		t.Skip("the test process's parent is unreadable")
	}
	parentStart, ok := StartedAt(parent, nil)
	if !ok {
		t.Skip("the test process's parent start is unreadable")
	}

	none := t.TempDir()
	if found, err := MachineryAncestor(none, childOf(t)); err != nil || found {
		t.Fatalf("no custody record: found=%t err=%v", found, err)
	}

	supervised := t.TempDir()
	writeJSON(t, filepath.Join(supervised, "artifacts/agents/supervision/state.json"),
		fmt.Sprintf(`{"owner":{"pid":%d,"pidStartedAt":%d,"instanceTag":"o"},"components":{}}`, self, selfStart(t)))
	if found, err := MachineryAncestor(supervised, childOf(t)); err != nil || !found {
		t.Fatalf("a supervision parent: found=%t err=%v", found, err)
	}

	// The adapter supervisor is the caller's grandparent: the walk goes past
	// the first ancestor, where ClassifyAt may already have answered.
	adapter := t.TempDir()
	writeJSON(t, filepath.Join(adapter, "artifacts/agents/jobs/job-1.json"),
		fmt.Sprintf(`{"jobId":"job-1","pid":%d,"pidStartedAt":%d}`, parent, parentStart))
	if found, err := MachineryAncestor(adapter, childOf(t)); err != nil || !found {
		t.Fatalf("an adapter-supervisor grandparent: found=%t err=%v", found, err)
	}

	corrupt := t.TempDir()
	writeJSON(t, filepath.Join(corrupt, "artifacts/agents/supervision/state.json"), "{")
	var failure *ClassificationFailure
	if found, err := MachineryAncestor(corrupt, childOf(t)); !errors.As(err, &failure) || found {
		t.Fatalf("a corrupt supervision state: found=%t err=%v", found, err)
	}
}
