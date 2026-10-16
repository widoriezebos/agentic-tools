package steward

// These adapter tests exercise linked worktrees, quarantine objects and the
// steward's applying and reporting passes in isolated repositories.
import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type orphanGitBed struct {
	top, worktree, quarantine, commit string
	record                            diskstore.Record
}

func orphanWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
func orphanGit(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(gittree.ScrubbedEnviron("GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com"), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func newOrphanGitBed(t *testing.T, registered bool) orphanGitBed {
	t.Helper()
	top := canonicalPath(t.TempDir())
	orphanGit(t, top, nil, "init", "-q", "-b", "main")
	orphanWrite(t, filepath.Join(top, "README"), "base\n")
	orphanWrite(t, filepath.Join(top, "metasystem.conf"), "metasystem.template=false\n")
	orphanWrite(t, filepath.Join(top, ".gitignore"), "artifacts/\n")
	orphanGit(t, top, nil, "add", ".")
	orphanGit(t, top, nil, "commit", "-q", "-m", "base")
	bed := orphanGitBed{top: top, worktree: filepath.Join(top, "artifacts", "agents", "worktrees", "orphan-1")}
	orphanGit(t, top, nil, "worktree", "add", "-q", "-b", "agent/orphan-1", bed.worktree)
	identityOf, err := diskstore.ReadGitIdentity(bed.worktree)
	if err != nil {
		t.Fatal(err)
	}
	bed.quarantine = filepath.Join(identityOf.Gitdir, diskstore.QuarantineName)
	if err := os.MkdirAll(bed.quarantine, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := diskstore.AddAlternate(filepath.Join(top, ".git", "objects"), bed.quarantine); err != nil {
		t.Fatal(err)
	}
	orphanWrite(t, filepath.Join(bed.worktree, "work.txt"), "orphan work\n")
	env := []string{"GIT_OBJECT_DIRECTORY=" + bed.quarantine, "GIT_ALTERNATE_OBJECT_DIRECTORIES=" + filepath.Join(top, ".git", "objects")}
	orphanGit(t, bed.worktree, env, "add", "work.txt")
	orphanGit(t, bed.worktree, env, "commit", "-q", "-m", "orphan work")
	bed.commit = orphanGit(t, bed.worktree, nil, "rev-parse", "HEAD")
	if registered {
		bed.record, err = diskstore.AdmitDelegate(context.Background(), diskstore.CheckoutRegistry(top), top, top, bed.worktree, ExecWorkspaceGit, time.Now())
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(top, "artifacts", "agents", "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	return bed
}
func (b orphanGitBed) borrowed(t *testing.T, want bool) {
	t.Helper()
	out, err := os.ReadFile(filepath.Join(b.top, ".git", "objects", "info", "alternates"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if got := strings.Contains(string(out), b.quarantine); got != want {
		t.Fatalf("borrowed=%v want %v: %s", got, want, out)
	}
}
func (b orphanGitBed) stewardPass(t *testing.T, mode diskstore.Mode) diskstore.Report {
	t.Helper()
	now := time.Now()
	proof, err := DelegateProof(b.top, now)
	if err != nil {
		t.Fatal(err)
	}
	result, err := SweepDiskStores(context.Background(), b.top, DiskPass{Mode: mode, Now: now, Clock: time.Now, SkipMachine: true,
		Home: filepath.Join(b.top, "home"), Processes: identity.ListedProcessTable{},
		Proofs: map[diskstore.OwnerKind]diskstore.OwnerProof{diskstore.OwnerDelegate: proof}})
	if err != nil {
		t.Fatal(err)
	}
	return result.Checkout
}
func (b orphanGitBed) censusPass(t *testing.T) diskstore.Report {
	t.Helper()
	proof, err := DelegateProof(b.top, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	registry := diskstore.CheckoutRegistry(b.top)
	census := &diskstore.CensusReader{Pids: func() ([]int64, error) { return []int64{42}, nil }, ProcessUID: func(int64) (uint32, bool) { return 501, true },
		Use: func(int64) (identity.ProcessUse, error) { return identity.ProcessUse{Cwd: b.worktree}, nil }, Command: func(int64) string { return "fixture holder" }}
	report, err := diskstore.RunPass(context.Background(), diskstore.PassOptions{Kind: "checkout", Name: b.top, Registry: registry,
		LockPath: filepath.Join(registry.Dir, ".sweep.flock"), ReportPath: diskstore.CheckoutReportPath(b.top), Mode: diskstore.ModeApply,
		Now: time.Now(), Clock: time.Now, Entropy: rand.Reader, CensusReader: census, Classes: []diskstore.Class{
			diskstore.RegisteredStores{Registry: registry, Proofs: map[diskstore.OwnerKind]diskstore.OwnerProof{diskstore.OwnerDelegate: proof}}, DelegateWorktrees{Top: b.top}}})
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func TestDelegateRecordsGoneThroughStewardPass(t *testing.T) {
	t.Parallel()
	for _, registered := range []bool{false, true} {
		for _, scenario := range []string{"empty inventory", "jobs absent", "holder", "unreadable record", "listing denied", "stat error", "follow-up", "workspace follow-up", "unresolved ancestry", "unresolved record identity", "renamed root", "duplicate record identity"} {
			name := map[bool]string{false: "legacy", true: "registered"}[registered] + "/" + scenario
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				b := newOrphanGitBed(t, registered)
				jobs := filepath.Join(b.top, "artifacts", "agents", "jobs")
				want := ""
				switch scenario {
				case "jobs absent":
					if err := os.Remove(jobs); err != nil {
						t.Fatal(err)
					}
				case "unreadable record":
					path := filepath.Join(jobs, "orphan-1.json")
					orphanWrite(t, path, `{"jobId":"orphan-1"}`)
					if err := os.Chmod(path, 0); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
					want = "permission denied"
				case "listing denied":
					if err := os.Chmod(jobs, 0o111); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = os.Chmod(jobs, 0o755) })
					want = "permission denied"
				case "stat error":
					if err := os.Remove(jobs); err != nil {
						t.Fatal(err)
					}
					orphanWrite(t, jobs, "not a directory")
					want = "not a directory"
				case "follow-up":
					orphanWrite(t, filepath.Join(jobs, "round-2.json"), `{"jobId":"round-2","parentJob":"orphan-1","status":"running"}`)
					want = "surviving round"
				case "workspace follow-up":
					orphanWrite(t, filepath.Join(jobs, "round-3.json"), `{"jobId":"round-3","parentJob":"round-2","workspaceRoot":"`+b.worktree+`","status":"running"}`)
					want = "names this workspace"
				case "unresolved ancestry":
					orphanWrite(t, filepath.Join(jobs, "round-3.json"), `{"jobId":"round-3","parentJob":"round-2"}`)
					want = "unresolved ancestry"
				case "unresolved record identity":
					orphanWrite(t, filepath.Join(jobs, "unknown.json"), `{"status":"running"}`)
					want = "unresolved identity"
				case "duplicate record identity":
					orphanWrite(t, filepath.Join(jobs, "a.json"), `{"jobId":"same","parentJob":"orphan-1"}`)
					orphanWrite(t, filepath.Join(jobs, "z.json"), `{"jobId":"same"}`)
					want = "unresolved identity"
				case "renamed root":
					orphanWrite(t, filepath.Join(jobs, "renamed.json"), `{"jobId":"orphan-1","status":"running"}`)
					want = "surviving round"
				case "holder":
					want = "in use by pid 42"
				}
				var report diskstore.Report
				if scenario == "holder" {
					report = b.censusPass(t)
				} else {
					report = b.stewardPass(t, diskstore.ModeApply)
				}
				if _, err := os.Stat(filepath.Join(b.worktree, "work.txt")); err != nil {
					t.Fatalf("workspace removed: %v", err)
				}
				if want != "" {
					b.borrowed(t, true)
					if len(report.Pending) != 1 || !strings.Contains(report.Pending[0].Reason, want) {
						t.Fatalf("pending cause %q missing: %+v", want, report)
					}
					if _, err := os.Stat(filepath.Join(b.top, ".git", "objects", b.commit[:2], b.commit[2:])); !os.IsNotExist(err) {
						t.Fatalf("published objects while pending: %v", err)
					}
					return
				}
				b.borrowed(t, false)
				if len(report.Kept) != 1 || !strings.Contains(report.Kept[0].Command, "metasystem disk clean --discard '") {
					t.Fatalf("discard remedy missing: %+v", report)
				}
				text := strings.Join(report.VerboseLines(), "\n")
				if strings.Contains(text, "git worktree remove") || !strings.Contains(text, "its objects are in the repository") {
					t.Fatalf("unsafe wording: %s", text)
				}
				err := filepath.WalkDir(b.quarantine, func(path string, entry os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					rel, _ := filepath.Rel(b.quarantine, path)
					if entry.IsDir() || len(filepath.Dir(rel)) != 2 || len(entry.Name()) != 38 {
						return nil
					}
					src, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					dst, err := os.ReadFile(filepath.Join(b.top, ".git", "objects", rel))
					if err != nil {
						return err
					}
					if !bytes.Equal(src, dst) {
						return errors.New("common object content differs")
					}
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				// Both forms are admitted for the guarded discard after absorption.
				record, err := diskstore.AdmitDelegate(context.Background(), diskstore.CheckoutRegistry(b.top), b.top, b.top, b.worktree, ExecWorkspaceGit, time.Now())
				if err != nil {
					t.Fatal(err)
				}
				proof, err := DelegateProof(b.top, time.Now())
				if err != nil {
					t.Fatal(err)
				}
				verdict, err := proof.ReleaseDiscarded(context.Background(), diskstore.CheckoutRegistry(b.top), record.ID, &diskstore.UseCensus{Taken: true}, diskstore.Discard{By: "tester", Reason: "fixture", At: time.Now()})
				if err != nil || verdict.Decision != diskstore.Release {
					t.Fatalf("guarded discard: %+v %v", verdict, err)
				}
			})
		}
	}
}

func TestDelegateClosedObjectsThroughStewardModes(t *testing.T) {
	t.Parallel()
	for _, mode := range []diskstore.Mode{diskstore.ModePreview, diskstore.ModeReport, diskstore.ModeApply} {
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()
			b := newOrphanGitBed(t, true)
			orphanWrite(t, filepath.Join(b.top, "artifacts", "agents", "jobs", "orphan-1.json"), `{"jobId":"orphan-1","chainClosed":true}`)
			orphanWrite(t, filepath.Join(b.worktree, "dirty"), "keep\n")
			now := time.Now()
			proof, err := DelegateProof(b.top, now)
			if err != nil {
				t.Fatal(err)
			}
			proof.Closed = func(string, string) error { return nil }
			proof.CustodyDead = func(map[string]any) dispatch.CustodyDeathResult {
				return dispatch.CustodyDeathResult{Outcome: dispatch.CustodyDeathProven}
			}
			result, err := SweepDiskStores(context.Background(), b.top, DiskPass{Mode: mode, Now: now, Clock: time.Now, SkipMachine: true,
				Home: filepath.Join(b.top, "home"), Processes: identity.ListedProcessTable{}, Proofs: map[diskstore.OwnerKind]diskstore.OwnerProof{diskstore.OwnerDelegate: proof}})
			if err != nil {
				t.Fatal(err)
			}
			b.borrowed(t, mode != diskstore.ModeApply)
			if len(result.Checkout.Kept) != 1 || !result.Checkout.Kept[0].Item.Verdict.Absorbable {
				t.Fatalf("kept absorbable verdict missing: %+v", result.Checkout)
			}
			_, err = os.Stat(filepath.Join(b.top, ".git", "objects", b.commit[:2], b.commit[2:]))
			if mode == diskstore.ModeApply && err != nil || mode != diskstore.ModeApply && !os.IsNotExist(err) {
				t.Fatalf("published object in mode %s: %v", mode, err)
			}
		})
	}
}
