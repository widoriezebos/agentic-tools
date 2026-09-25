package goal

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// An absent accepted reference keeps the verdict in the on-disk legacy world.
// Every other repository operation is unexpected in that world.
type strictAbsentRepository struct {
	t        *testing.T
	accepted atomic.Int64
}

func (r *strictAbsentRepository) reject(method string) error {
	err := fmt.Errorf("unexpected repository %s in the legacy world", method)
	r.t.Error(err)
	return err
}

func (r *strictAbsentRepository) Accepted() (string, bool, error) {
	r.accepted.Add(1)
	return "", false, nil
}
func (r *strictAbsentRepository) Capture(string) (string, error) {
	return "", r.reject("Capture")
}
func (r *strictAbsentRepository) Files(string, ...string) (map[string][]byte, error) {
	return nil, r.reject("Files")
}
func (r *strictAbsentRepository) Build(string, string, []Change, string) (string, error) {
	return "", r.reject("Build")
}
func (r *strictAbsentRepository) Publish(string, string) (CASOutcome, error) {
	return "", r.reject("Publish")
}
func (r *strictAbsentRepository) AcceptedCAS(string, string) error {
	return r.reject("AcceptedCAS")
}
func (r *strictAbsentRepository) IsAncestor(string, string) (bool, error) {
	return false, r.reject("IsAncestor")
}
func (r *strictAbsentRepository) TrailerPresent(string, string) (bool, error) {
	return false, r.reject("TrailerPresent")
}
func (r *strictAbsentRepository) CommitWithTrailer(string, string, string) (string, error) {
	return "", r.reject("CommitWithTrailer")
}
func (r *strictAbsentRepository) CommitTime(string) (time.Time, error) {
	return time.Time{}, r.reject("CommitTime")
}
func (r *strictAbsentRepository) Release(string) error {
	return r.reject("Release")
}

func (r *strictAbsentRepository) requireAccepted(t *testing.T) {
	t.Helper()
	if r.accepted.Load() == 0 {
		t.Error("the accepted-reference absence path was not exercised")
	}
}

func legacyVerdictStore(t *testing.T, expectProbe bool) *Store {
	t.Helper()
	store := testStore(t)
	repository := &strictAbsentRepository{t: t}
	store.projectionDeps.source = &projectionSource{
		endpoint: Endpoint{Root: store.Root, Repository: repository},
		machine:  "mac-a",
	}
	if expectProbe {
		t.Cleanup(func() { repository.requireAccepted(t) })
	}
	return store
}

func legacyTurnVerdict(store *Store, scan ScanResult, sessionID, watchdogDigest, mainID string, option ...TurnVerdictOptions) (Verdict, error) {
	if len(option) == 0 {
		option = []TurnVerdictOptions{{}}
	}
	if option[0].SeatActor.Machine == "" {
		option[0].SeatActor.Machine = "mac-a"
	}
	return store.TurnVerdict(scan, sessionID, watchdogDigest, mainID, option...)
}
