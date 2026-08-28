package census

import (
	"errors"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"
)

type taggedTestReader struct {
	starts map[int64]identity.Exact
	argv   map[int64][]string
	known  map[int64]bool
}

type unreadableTaggedTestReader struct{}

func (unreadableTaggedTestReader) ReadStart(int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{}, identity.Unknown, errors.New("identity unreadable")
}

func (unreadableTaggedTestReader) ReadArgv(int64) ([]string, bool) {
	return nil, false
}

func (r taggedTestReader) ReadStart(pid int64) (identity.Exact, identity.Liveness, error) {
	exact, ok := r.starts[pid]
	if !ok {
		return identity.Exact{}, identity.Dead, nil
	}
	return exact, identity.Alive, nil
}

func (r taggedTestReader) ReadArgv(pid int64) ([]string, bool) {
	return r.argv[pid], r.known[pid]
}

func TestTaggedProcessCensusPreservesUnreadableArgv(t *testing.T) {
	tag := "metasystem-job-unreadable-nonce"
	reader := taggedTestReader{
		starts: map[int64]identity.Exact{
			41: {Pid: 41, StartedAt: time.UnixMicro(100_000_001)},
		},
		argv:  map[int64][]string{},
		known: map[int64]bool{41: false},
	}
	result := ScanTaggedProcesses(tag, TaggedScanDependencies{
		PIDs:   func() ([]int64, error) { return []int64{41}, nil },
		Signal: func(int64) error { return nil },
		PGID:   func(int64) (int64, error) { return 41, nil },
		Reader: reader,
		MatchesTag: func(argv []string, wanted string) bool {
			return len(argv) == 2 && argv[1] == wanted
		},
	})
	if result.Complete() {
		t.Fatal("an unreadable argv observation cannot prove complete-census absence")
	}
	if len(result.Tagged) != 0 || len(result.Indeterminate) != 1 || result.Indeterminate[0].PID != 41 {
		t.Fatalf("unknown observation was dropped: %+v", result)
	}
	if result.Indeterminate[0].Universe != ProcessUniverseSignalable || result.UnknownWithinUniverse() != 1 {
		t.Fatalf("same-user unreadable observation left the candidate universe: %+v", result)
	}
}

func TestTaggedProcessCensusExcludesOldSignalableUnknownByReservationAge(t *testing.T) {
	createdAt := time.Unix(200, 0).UTC()
	reader := taggedTestReader{
		starts: map[int64]identity.Exact{
			43: {Pid: 43, StartedAt: createdAt.Add(-ReservationStartTimeSlack - time.Second)},
		},
		argv:  map[int64][]string{},
		known: map[int64]bool{43: false},
	}
	result := ScanTaggedProcesses("metasystem-job-old-daemon-nonce", TaggedScanDependencies{
		PIDs:   func() ([]int64, error) { return []int64{43}, nil },
		Signal: func(int64) error { return nil },
		Reader: reader,
		MatchesTag: func(argv []string, wanted string) bool {
			return false
		},
		ReservationCreatedAt: createdAt,
	})
	if !result.Complete() || result.UnknownWithinUniverse() != 0 || result.ExcludedByAgeCount() != 1 {
		t.Fatalf("old same-user unknown blocked reservation absence: %+v", result)
	}
	if len(result.Indeterminate) != 1 || result.Indeterminate[0].Universe != ProcessUniverseExcludedByAge {
		t.Fatalf("old observation lost its age classification: %+v", result)
	}
}

func TestTaggedProcessCensusKeepsPostReservationUnknownWithinUniverse(t *testing.T) {
	createdAt := time.Unix(200, 0).UTC()
	reader := taggedTestReader{
		starts: map[int64]identity.Exact{
			44: {Pid: 44, StartedAt: createdAt.Add(time.Second)},
		},
		argv:  map[int64][]string{},
		known: map[int64]bool{44: false},
	}
	result := ScanTaggedProcesses("metasystem-job-new-worker-nonce", TaggedScanDependencies{
		PIDs:   func() ([]int64, error) { return []int64{44}, nil },
		Signal: func(int64) error { return nil },
		Reader: reader,
		MatchesTag: func(argv []string, wanted string) bool {
			return false
		},
		ReservationCreatedAt: createdAt,
	})
	if result.Complete() || result.UnknownWithinUniverse() != 1 || result.ExcludedByAgeCount() != 0 {
		t.Fatalf("post-reservation unknown left the candidate universe: %+v", result)
	}
}

func TestTaggedProcessCensusPreservesButExcludesForeignUnknowns(t *testing.T) {
	reader := taggedTestReader{}
	result := ScanTaggedProcesses("metasystem-job-foreign-nonce", TaggedScanDependencies{
		PIDs:   func() ([]int64, error) { return []int64{302}, nil },
		Signal: func(int64) error { return unix.EPERM },
		Reader: reader,
		MatchesTag: func(argv []string, wanted string) bool {
			return false
		},
	})
	if !result.Complete() || result.UnknownWithinUniverse() != 0 {
		t.Fatalf("foreign process blocked complete candidate-universe absence: %+v", result)
	}
	if len(result.Indeterminate) != 1 || result.Indeterminate[0].Universe != ProcessUniverseForeign {
		t.Fatalf("foreign observation lost its universe classification: %+v", result)
	}
}

func TestTaggedProcessCensusKeepsSignalableUnreadableIdentityUnknown(t *testing.T) {
	result := ScanTaggedProcesses("metasystem-job-identity-nonce", TaggedScanDependencies{
		PIDs:   func() ([]int64, error) { return []int64{42}, nil },
		Signal: func(int64) error { return nil },
		Reader: unreadableTaggedTestReader{},
		MatchesTag: func(argv []string, wanted string) bool {
			return false
		},
		ReservationCreatedAt: time.Unix(200, 0).UTC(),
	})
	if result.Complete() || result.UnknownWithinUniverse() != 1 {
		t.Fatalf("signalable unreadable identity did not block absence: %+v", result)
	}
}

func TestTaggedProcessCensusPreservesEnumerationFailure(t *testing.T) {
	result := ScanTaggedProcesses("metasystem-job-enumeration-nonce", TaggedScanDependencies{
		PIDs: func() ([]int64, error) { return nil, errors.New("process table denied") },
	})
	if result.Complete() || result.EnumerationError == "" {
		t.Fatalf("enumeration failure was normalized to absence: %+v", result)
	}
}
