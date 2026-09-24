package goal

import (
	"fmt"
	"testing"
)

// filesystemReconcileCalls declares the checkout calls a filesystem test permits.
// Calls are consumed in order, so a missing or extra checkout operation fails.
type filesystemReconcileCalls struct {
	t        *testing.T
	endpoint Endpoint
	want     []filesystemReconcileCall
}

type filesystemReconcileCall struct {
	kind, value string
}

func newFilesystemReconcileCalls(t *testing.T, endpoint Endpoint) *filesystemReconcileCalls {
	t.Helper()
	calls := &filesystemReconcileCalls{t: t, endpoint: endpoint}
	t.Cleanup(calls.assertConsumed)
	return calls
}

func (c *filesystemReconcileCalls) expectHead(tip string) {
	c.want = append(c.want, filesystemReconcileCall{"head", tip})
}

func (c *filesystemReconcileCalls) expectAnchor(commit string) {
	c.want = append(c.want, filesystemReconcileCall{"anchor", commit})
}

func (c *filesystemReconcileCalls) expectResolve() {
	c.want = append(c.want, filesystemReconcileCall{"resolve", ""})
}

func (c *filesystemReconcileCalls) consume(kind, root, value string) {
	c.t.Helper()
	if root != c.endpoint.Root || len(c.want) == 0 || c.want[0] != (filesystemReconcileCall{kind, value}) {
		c.t.Fatalf("unexpected %s callback: root=%q value=%q remaining=%v", kind, root, value, c.want)
	}
	c.want = c.want[1:]
}

func (c *filesystemReconcileCalls) head(root string) (string, error) {
	c.t.Helper()
	if root != c.endpoint.Root || len(c.want) == 0 || c.want[0].kind != "head" {
		c.t.Fatalf("unexpected head callback: root=%q remaining=%v", root, c.want)
	}
	tip := c.want[0].value
	c.consume("head", root, tip)
	return tip, nil
}

func (c *filesystemReconcileCalls) anchor(root, commit string) error {
	c.t.Helper()
	c.consume("anchor", root, commit)
	return nil
}

func (c *filesystemReconcileCalls) resolve(root string) (Endpoint, error) {
	c.t.Helper()
	c.consume("resolve", root, "")
	return c.endpoint, nil
}

func (c *filesystemReconcileCalls) assertConsumed() {
	c.t.Helper()
	if len(c.want) != 0 {
		c.t.Fatalf("checkout callbacks not consumed: %v", c.want)
	}
}

func nextFakeReconcileCommit(endpoint Endpoint) string {
	repo := endpoint.Repository.(*fakeGoalRepository)
	repo.store.mu.Lock()
	defer repo.store.mu.Unlock()
	return fmt.Sprintf("%040x", repo.store.serial+1)
}
