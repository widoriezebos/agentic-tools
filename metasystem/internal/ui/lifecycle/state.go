// Package lifecycle owns the checkout's interface server independently of the engine.
package lifecycle

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Dir is the lifecycle state directory beneath the state root, beside the
// steward's and the supervision's.
func Dir(stateRoot string) string { return filepath.Join(stateRoot, "artifacts", "agents", "ui") }

func recordPath(stateRoot string) string { return filepath.Join(Dir(stateRoot), "server.json") }
func lockPath(stateRoot string) string   { return filepath.Join(Dir(stateRoot), "server.flock") }

type Record struct {
	SchemaVersion    int    `json:"schemaVersion"`
	Process          string `json:"process"`
	Address          string `json:"address"`
	Checkout         string `json:"checkout"`
	Installation     string `json:"installation"`
	StartedAt        string `json:"startedAt"`
	EngineBuild      string `json:"engineBuild"`
	ExecutableDigest string `json:"executableDigest"`
	// Authority is the one line this server's boot-time human-authority
	// observation produced. It is written here and nowhere else because
	// `ui status` runs in another process entirely and has no way to ask the
	// server: the observation is a fact of this run, so this run records it.
	// It is evidence, never a credential — nothing reads it back as
	// authority, and an older reader that does not know the key ignores it,
	// which is why the schema stays 1.
	Authority string `json:"authority,omitempty"`
	// Sessions is one line per browser signed into this run, for `ui status`
	// to print for the reason Authority is here: status runs in another
	// process and cannot ask the server. It lives and dies with this record,
	// which is right — a session is one process's observation. What must
	// outlive the run is the one-time-code floor, and that has a file of its
	// own; see sessions.go. An older reader ignores the key.
	Sessions []string `json:"sessions,omitempty"`
}

// updating serializes this process's rewrites of its own record. One process
// owns one state root for its whole life — Serve holds the lock until it
// returns — so a mutex is the whole of the exclusion needed here.
var updating sync.Mutex

// Update rewrites the running server's record in place. A record that is not
// there is a server that has not written one yet or has stopped; neither is an
// error, and neither is recreated here.
func Update(stateRoot string, apply func(*Record)) error {
	updating.Lock()
	defer updating.Unlock()
	rec, err := readRecord(stateRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	apply(rec)
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(recordPath(stateRoot), string(data)+"\n", stateRoot)
	return err
}

type State string

const (
	Running       State = "running"
	Stopped       State = "stopped"
	Stale         State = "stale"
	Uninspectable State = "uninspectable"
	Unreadable    State = "unreadable"
	Busy          State = "busy"
)

type Status struct {
	State  State
	Record *Record
}

func readRecord(stateRoot string) (*Record, error) {
	data, err := os.ReadFile(recordPath(stateRoot))
	if err != nil {
		return nil, err
	}
	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}
	if rec.SchemaVersion != 1 {
		return nil, errors.New("unsupported interface record schema")
	}
	if _, err := identity.ParseRef(rec.Process); err != nil {
		return nil, err
	}
	return &rec, nil
}

func Read(stateRoot string, prober identity.Prober) (Status, error) {
	rec, err := readRecord(stateRoot)
	if err == nil {
		ref, _ := identity.ParseRef(rec.Process)
		switch identity.AliveRef(prober, ref) {
		case identity.Alive:
			return Status{Running, rec}, nil
		case identity.Unknown:
			return Status{Uninspectable, rec}, nil
		}
	}
	return readInactive(stateRoot, err)
}

func readInactive(stateRoot string, recordErr error) (Status, error) {
	f, won, err := probeLock(stateRoot)
	if errors.Is(err, os.ErrNotExist) {
		return Status{State: Stopped}, nil
	}
	if err != nil {
		return Status{}, err
	}
	if !won {
		if recordErr != nil && !errors.Is(recordErr, os.ErrNotExist) {
			return Status{State: Unreadable}, nil
		}
		return Status{State: Busy}, nil
	}
	defer releaseLock(f)
	// The record may have changed while we waited; only the lock authorizes removal.
	rec, _ := readRecord(stateRoot)
	if err := removeRecord(stateRoot); err != nil {
		return Status{}, err
	}
	if rec != nil {
		return Status{Stale, rec}, nil
	}
	return Status{State: Stopped}, nil
}

func removeRecord(stateRoot string) error {
	err := os.Remove(recordPath(stateRoot))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func probeLock(stateRoot string) (*os.File, bool, error) {
	f, err := os.OpenFile(lockPath(stateRoot), os.O_RDWR, 0)
	if err != nil {
		return nil, false, err
	}
	err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err == nil {
		return f, true, nil
	}
	_ = f.Close()
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return nil, false, nil
	}
	return nil, false, err
}

func releaseLock(f *os.File) {
	_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
	_ = f.Close()
}

// Each waiter owns its open file description. A timed-out waiter must release
// a later acquisition rather than leave a server blocked behind it.
func waitLock(stateRoot string, create bool, wait time.Duration, after func(time.Duration) <-chan time.Time) (*os.File, bool, <-chan struct{}, error) {
	done := make(chan struct{})
	flags := os.O_RDWR
	if create {
		flags |= os.O_CREATE
	}
	f, err := os.OpenFile(lockPath(stateRoot), flags, 0o644)
	if err != nil {
		close(done)
		return nil, false, done, err
	}
	if after == nil {
		after = time.After
	}
	result := make(chan error)
	abandoned := make(chan struct{})
	go func() {
		defer close(done)
		err := unix.Flock(int(f.Fd()), unix.LOCK_EX)
		select {
		case result <- err:
			if err != nil {
				_ = f.Close()
			}
		case <-abandoned:
			if err == nil {
				releaseLock(f)
			} else {
				_ = f.Close()
			}
		}
	}()
	select {
	case err := <-result:
		if err != nil {
			return nil, false, done, err
		}
		return f, true, done, nil
	case <-after(wait):
		close(abandoned)
		return nil, false, done, nil
	}
}
