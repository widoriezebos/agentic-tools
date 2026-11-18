package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

// TestStatusAndWorkStatusHaveNoPathToTheSocket (R25, U10c-2): the one-shot
// views read the board directly; none of their files dials, subscribes or
// names the bridge's socket.
func TestStatusAndWorkStatusHaveNoPathToTheSocket(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"intent_table.go", "intent_selection.go", "intent_process.go"} {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		source, _ := os.ReadFile(file)
		for _, forbidden := range []string{"board.Dial", "board.Subscribe", "bridgeNudges", "bridge.sock", "net.Dial"} {
			if strings.Contains(string(source), forbidden) {
				t.Errorf("%s names %s: a one-shot view never uses the bridge", file, forbidden)
			}
		}
		for _, spec := range parsed.Imports {
			if path, _ := strconv.Unquote(spec.Path.Value); path == "net" {
				t.Errorf("%s imports net", file)
			}
		}
	}
}

func TestTheWatcherPassReturnsWhileALockIsHeld(t *testing.T) {
	t.Parallel()
	for _, held := range []string{"arm.flock", "engine-pins/.prepare.flock"} {
		t.Run(held, func(t *testing.T) {
			t.Parallel()
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			now := time.Unix(1800000000, 0)
			enrollReviveRunner(t, root, now)
			directory := filepath.Join(root, "artifacts", "agents", "steward")
			if err := os.Remove(filepath.Join(directory, "runner.json")); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(directory, held)
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			heldLock, err := lock.File(path, 0o600, lock.Exclusive)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = heldLock.Release() })
			cfg := supervise.WatcherConfig{
				SupervisionDir: supervise.SupervisionDir(root), Interval: 60, IntervalMS: 60000, BudgetPercent: 100,
				Fingerprint: func() (string, error) { return "repair-step", nil },
				Census: func(fp string, at time.Time) (census.Verdict, error) {
					generation := int64(1)
					return census.Verdict{Verdict: "SUCCESS", Generation: &generation, Fingerprint: fp}, nil
				},
				Now: func() time.Time { return now },
			}
			pass := watcherPass(io.Discard, root, identity.Ref{Pid: 75, StartedAtSec: 100}, 1, cfg)
			done := make(chan error, 1)
			go func() { done <- pass() }()
			var passErr error
			testenv.Await(t, "watcher pass to finish while "+held+" is held", func() bool {
				select {
				case passErr = <-done:
					return true
				default:
					return false
				}
			})
			if passErr != nil {
				t.Fatal(passErr)
			}
			data, err := os.ReadFile(steward.ComponentEvidencePath(root, "repo-watcher"))
			if err != nil {
				t.Fatal(err)
			}
			var evidence steward.ComponentEvidence
			if err := json.Unmarshal(data, &evidence); err != nil {
				t.Fatal(err)
			}
			wantDigest := fmt.Sprintf("%x", sha256.Sum256([]byte("census, run assessment, and steward check completed (BUSY)")))
			if evidence.Result != steward.ComponentOK || evidence.Outcome != "PASS_COMPLETE" || evidence.EvidenceDigest != wantDigest {
				t.Fatalf("watcher did not complete a busy pass: %s", data)
			}
			if _, err := os.Stat(filepath.Join(directory, "runner.json")); !os.IsNotExist(err) {
				t.Fatalf("busy watcher wrote a runner record: %v", err)
			}
		})
	}
}
