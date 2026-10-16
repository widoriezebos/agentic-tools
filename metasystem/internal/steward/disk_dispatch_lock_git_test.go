package steward

// These adapter tests need Git's quarantine and linked-worktree semantics:
// a fake cannot establish whether a commit remains readable from the repository.
import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

func dispatchLockFixture(t *testing.T, b orphanGitBed, state string) string {
	t.Helper()
	path := filepath.Join(b.top, "artifacts", "agents", "locks", "orphan-1.d")
	if state == "absent" {
		return path
	}
	if state == "stat error" {
		orphanWrite(t, filepath.Dir(path), "not a directory")
		if _, err := os.Stat(path); err == nil || os.IsNotExist(err) || !strings.Contains(err.Error(), "not a directory") {
			t.Fatalf("fixture must force a non-absence stat error: %v", err)
		}
		return path
	}
	pid, tag := os.Getpid(), filepath.Base(os.Args[0])
	if state == "dead" {
		// Wait reaps the child, so its owner is dead before any observation.
		cmd := exec.Command("/usr/bin/true")
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
		pid = cmd.Process.Pid
	}
	owner := fmt.Sprintf(`{"pid":%d,"instanceTag":%q}`, pid, tag)
	if state == "unknown" {
		owner = "{unreadable owner identity"
	}
	orphanWrite(t, filepath.Join(path, "owner.json"), owner)
	return path
}

func dispatchObjectsSnapshot(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			files[path], err = os.ReadFile(path)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func TestDelegateDispatchLockThroughStewardPass(t *testing.T) {
	t.Parallel()
	for _, registered := range []bool{false, true} {
		for _, state := range []string{"live", "unknown", "stat error", "absent", "dead"} {
			t.Run(fmt.Sprintf("registered=%v/%s", registered, state), func(t *testing.T) {
				t.Parallel()
				b := newOrphanGitBed(t, registered)
				path := dispatchLockFixture(t, b, state)
				objects := dispatchObjectsSnapshot(t, filepath.Join(b.top, ".git", "objects"))
				quarantine := dispatchObjectsSnapshot(t, b.quarantine)
				var lockInfo os.FileInfo
				var owner []byte
				if state == "dead" {
					var err error
					lockInfo, err = os.Stat(path)
					if err != nil {
						t.Fatal(err)
					}
					owner, err = os.ReadFile(filepath.Join(path, "owner.json"))
					if err != nil {
						t.Fatal(err)
					}
				}
				for _, mode := range []diskstore.Mode{diskstore.ModePreview, diskstore.ModeReport, diskstore.ModeApply} {
					report := b.stewardPass(t, mode)
					absorb := mode == diskstore.ModeApply && (state == "dead" || state == "absent")
					b.borrowed(t, !absorb)
					if _, err := os.Stat(b.worktree); err != nil {
						t.Fatalf("workspace removed: %v", err)
					}
					if !reflect.DeepEqual(quarantine, dispatchObjectsSnapshot(t, b.quarantine)) {
						t.Fatal("quarantine objects changed")
					}
					if !absorb && !reflect.DeepEqual(objects, dispatchObjectsSnapshot(t, filepath.Join(b.top, ".git", "objects"))) {
						t.Fatal("objects or alternates changed during observation or a blocked apply")
					}
					if state == "dead" && mode != diskstore.ModeApply {
						after, err := os.Stat(path)
						if err != nil || !os.SameFile(lockInfo, after) {
							t.Fatalf("observation replaced the dead owner's lock: %v", err)
						}
						got, err := os.ReadFile(filepath.Join(path, "owner.json"))
						if err != nil || !bytes.Equal(owner, got) {
							t.Fatalf("observation rewrote the owner: %v", err)
						}
					}
					if state == "dead" && absorb {
						if _, err := os.Stat(path); !os.IsNotExist(err) {
							t.Fatalf("dead launch lock was not reclaimed: %v", err)
						}
					}
					if state == "live" || state == "unknown" || state == "stat error" {
						want := "a dispatch of this chain is in progress"
						if state == "stat error" {
							want = "not a directory"
						}
						if len(report.Pending) != 1 || !strings.Contains(report.Pending[0].Reason, want) || report.Pending[0].Item.Verdict.Absorbable || strings.Contains(report.Pending[0].Command, "--discard") {
							t.Fatalf("dispatch must stay pending without absorb or discard: %+v", report)
						}
					} else if absorb {
						if len(report.Kept) != 1 || !strings.Contains(report.Kept[0].Command, "--discard") {
							t.Fatalf("true orphan must name guarded discard: %+v", report)
						}
						orphanGit(t, b.top, nil, "log", "-1", "agent/orphan-1")
						if _, err := os.Stat(filepath.Join(b.top, ".git", "objects", b.commit[:2], b.commit[2:])); err != nil {
							t.Fatalf("orphan commit was not absorbed: %v", err)
						}
					}
				}
				if state == "live" {
					orphanWrite(t, filepath.Join(b.worktree, "after-apply"), "delegate continues\n")
					env := []string{"GIT_OBJECT_DIRECTORY=" + b.quarantine, "GIT_ALTERNATE_OBJECT_DIRECTORIES=" + filepath.Join(b.top, ".git", "objects")}
					orphanGit(t, b.worktree, env, "add", "after-apply")
					orphanGit(t, b.worktree, env, "commit", "-q", "-m", "dispatch still running")
					if got := orphanGit(t, b.top, nil, "log", "-1", "--format=%s", "agent/orphan-1"); got != "dispatch still running" {
						t.Fatalf("delegate commit unreadable from repository: %s", got)
					}
				}
			})
		}
	}
}

func TestDelegateDeadDispatchRechecksRecordsAndUse(t *testing.T) {
	t.Parallel()
	for _, registered := range []bool{false, true} {
		for _, scenario := range []string{"workspace in use", "surviving records"} {
			t.Run(fmt.Sprintf("registered=%v/%s", registered, scenario), func(t *testing.T) {
				t.Parallel()
				b := newOrphanGitBed(t, registered)
				path := dispatchLockFixture(t, b, "dead")
				var report diskstore.Report
				if scenario == "workspace in use" {
					report = b.censusPass(t)
					if len(report.Pending) != 1 || !strings.Contains(report.Pending[0].Reason, "in use by pid 42") {
						t.Fatalf("use census must still gate the orphan absorb: %+v", report)
					}
				} else {
					orphanWrite(t, filepath.Join(b.top, "artifacts", "agents", "jobs", "orphan-1.json"), `{"jobId":"orphan-1","chainClosed":false}`)
					report = b.stewardPass(t, diskstore.ModeApply)
					if len(report.Kept) != 1 || !strings.Contains(report.Kept[0].Reason, "the chain is open") || report.Kept[0].Item.Verdict.Absorbable {
						t.Fatalf("records must be rechecked after lock reclamation: %+v", report)
					}
				}
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("dead lock must be reclaimed even when absorption is kept: %v", err)
				}
				b.borrowed(t, true)
				if _, err := os.Stat(filepath.Join(b.top, ".git", "objects", b.commit[:2], b.commit[2:])); !os.IsNotExist(err) {
					t.Fatalf("absorbed despite fresh records or use proof: %v", err)
				}
			})
		}
	}
}
