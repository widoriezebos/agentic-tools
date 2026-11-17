package goal

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Git owns directory-relative path interpretation, linked-worktree metadata
// and object replacement. These adapter fixtures need those real semantics.
func TestCommittedFilesGitAdapterDirectories(t *testing.T) {
	t.Parallel()
	trace, child := committedFilesTraceProcess(t)
	if !child {
		return
	}
	for _, nestedFirst := range []bool{false, true} {
		root := t.TempDir()
		mustGit(t, root, "init", "-q")
		nested := filepath.Join(root, "nested")
		writeFile(t, filepath.Join(root, goalsPrefix, "backlog.md"), "top\n")
		writeFile(t, filepath.Join(nested, goalsPrefix, "backlog.md"), "nested\n")
		mustGit(t, root, "add", ".")
		mustGit(t, root, "commit", "-qm", "ledger directories")
		tip := mustGit(t, root, "rev-parse", "HEAD")
		linked := filepath.Join(t.TempDir(), "linked")
		mustGit(t, root, "worktree", "add", "--detach", linked, tip)
		beforeListing, beforeBatch := committedTraceCounts(t, trace)
		roots := []string{root, nested}
		if nestedFirst {
			roots[0], roots[1] = roots[1], roots[0]
		}
		for _, dir := range append(roots, filepath.Join(linked, "nested"), linked) {
			files, err := ReadCommitGoals(dir, tip)
			want := "top\n"
			if filepath.Base(dir) == "nested" {
				want = "nested\n"
			}
			if err != nil || string(files[goalsPrefix+"backlog.md"]) != want {
				t.Fatalf("%s files=%q err=%v", dir, files, err)
			}
		}
		listing, batch := committedTraceCounts(t, trace)
		if listing-beforeListing != 2 || batch-beforeBatch != 2 {
			t.Fatalf("directory identity/reuse: listings=%d batches=%d", listing, batch)
		}
	}
}

func committedTraceCounts(t *testing.T, path string) (listing, batch int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var event struct {
			Event string   `json:"event"`
			Argv  []string `json:"argv"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event.Event != "start" {
			continue
		}
		for i, arg := range event.Argv {
			if arg == "ls-tree" && i+2 < len(event.Argv) && event.Argv[i+1] == "-r" && event.Argv[i+2] == "--name-only" {
				listing++
			}
			if arg == "cat-file" && i+1 < len(event.Argv) && event.Argv[i+1] == "--batch" {
				batch++
			}
		}
	}
	return
}

func TestCommittedFilesGitAdapterReplacements(t *testing.T) {
	t.Parallel()
	_, child := committedFilesTraceProcess(t)
	if !child {
		return
	}
	for _, kind := range []string{"commit", "tree", "blob"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			mustGit(t, root, "init", "-q")
			path := goalsPrefix + "backlog.md"
			writeFile(t, filepath.Join(root, path), "original\n")
			mustGit(t, root, "add", ".")
			mustGit(t, root, "commit", "-qm", "original")
			original := mustGit(t, root, "rev-parse", "HEAD")
			writeFile(t, filepath.Join(root, path), "replacement\n")
			mustGit(t, root, "add", ".")
			mustGit(t, root, "commit", "-qm", "replacement")
			replacement := mustGit(t, root, "rev-parse", "HEAD")
			suffix := ""
			if kind == "tree" {
				suffix = "^{tree}"
			}
			if kind == "blob" {
				suffix = ":" + path
			}
			oldObject := mustGit(t, root, "rev-parse", original+suffix)
			newObject := mustGit(t, root, "rev-parse", replacement+suffix)
			mustGit(t, root, "replace", oldObject, newObject)
			// Prove the fixture changes a replacement-enabled content read.
			env := []string{}
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, "GIT_NO_REPLACE_OBJECTS=") {
					env = append(env, entry)
				}
			}
			got, err := gitInWithEnvironment(root, env, "cat-file", "-p", original+":"+path)
			if err != nil || got != "replacement\n" {
				t.Fatalf("replacement control=%q err=%v", got, err)
			}
			check := func(rev string, prefixes ...string) {
				t.Helper()
				files, err := (gitRepository{endpoint: Endpoint{Root: root, commandEnv: env}}).Files(rev, prefixes...)
				if err != nil || string(files[path]) != "original\n" {
					t.Fatalf("%s %v files=%q err=%v", kind, prefixes, files, err)
				}
			}
			check(original, goalsPrefix)
			check(original, goalsPrefix)
			mustGit(t, root, "update-ref", "refs/heads/original", original)
			check("refs/heads/original", goalsPrefix)
			mustGit(t, root, "replace", "-d", oldObject)
			check(original, goalsPrefix)
			mustGit(t, root, "replace", oldObject, newObject)
			env = append(env, "GIT_NO_REPLACE_OBJECTS=1")
			check(original, goalsPrefix)
			// A different prefix is cold; it must still read committed objects.
			ctx := withWaitGitDependencies(context.Background(), waitGitDependencies{run: func(ctx context.Context, root string, input []byte, args ...string) (string, error) {
				return runAttentionGitWithEnvironment(ctx, root, input, env, args...)
			}})
			files, err := readWaitFiles(ctx, root, original, path)
			if err != nil || string(files[path]) != "original\n" {
				t.Fatalf("wait read=%q err=%v", files, err)
			}
		})
	}
}

func TestCommittedFilesGitAdapterSharesAllLedgerReaders(t *testing.T) {
	t.Parallel()
	trace, child := committedFilesTraceProcess(t)
	if !child {
		return
	}
	root := t.TempDir()
	mustGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, goalsPrefix, "backlog.md"), string(RenderRoot(vRoot())))
	writeFile(t, filepath.Join(root, ChannelPrefix, "example"), "channel\n")
	mustGit(t, root, "add", ".")
	mustGit(t, root, "commit", "-qm", "ledger")
	tip := mustGit(t, root, "rev-parse", "HEAD")
	mustGit(t, root, "update-ref", AcceptedRef, tip)

	e := Endpoint{Root: root}
	for i := 0; i < 2; i++ {
		if ExistingLedgerIdentity(root) != vRoot().Identity {
			t.Fatal("wrong identity")
		}
		if _, err := readCommitGoals(e, tip); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadCommitGoals(root, tip); err != nil {
			t.Fatal(err)
		}
		if _, err := readWaitFiles(context.Background(), root, tip, goalsPrefix, recordsGoalsPrefix); err != nil {
			t.Fatal(err)
		}
		ValidateChannelTree(root, tip)
		if present, err := tipCarriesLedger(e, tip); err != nil || !present {
			t.Fatalf("present=%v err=%v", present, err)
		}
		if _, err := waitTreeIdentity(context.Background(), root, tip); err != nil {
			t.Fatal(err)
		}
	}
	listing, batch := committedTraceCounts(t, trace)
	if listing != 3 || batch != 3 {
		t.Fatalf("shared reader listings=%d batches=%d, want full ledger, channel, backlog once each", listing, batch)
	}
	mustGit(t, root, "update-ref", "-d", AcceptedRef)
	if identity := ExistingLedgerIdentity(root); identity != "" {
		t.Fatalf("identity without an accepted ref = %q", identity)
	}
}

// Each real-Git witness owns a process so trace environment and the process
// cache cannot be shared with concurrently running tests.
func committedFilesTraceProcess(t *testing.T) (string, bool) {
	t.Helper()
	const marker = "METASYSTEM_COMMITTED_FILES_TRACE_TEST"
	if os.Getenv(marker) == t.Name() {
		return os.Getenv("GIT_TRACE2_EVENT"), true
	}
	trace := filepath.Join(t.TempDir(), "trace.jsonl")
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+regexp.QuoteMeta(t.Name())+"$", "-test.count=1", "-test.timeout=10m")
	cmd.Env = append(os.Environ(), marker+"="+t.Name(), "GIT_TRACE2_EVENT="+trace)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("isolated committed-files witness: %v\n%s", err, out)
	}
	return trace, false
}
