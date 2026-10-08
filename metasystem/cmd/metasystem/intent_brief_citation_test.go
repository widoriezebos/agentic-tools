package main

import (
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// This command bed supplies only immutable Git objects. The files on disk
// deliberately have more lines, so a checkout read cannot pass as base proof.
func TestWorkBriefResolvesAndChecksBaseCitations(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, citation, problem                                                           string
		ambiguous, treeError, startError, blobError, catError, baseError, person, ignored bool
		plain                                                                             bool
	}{
		{name: "person-unchecked", citation: "feature/code.go:3", problem: "line 3 does not exist", person: true},
		{name: "cat-error-present", citation: "feature/code.go:2", problem: "object missing", catError: true},
		{name: "invalid-base", citation: "feature/code.go:2", problem: "invalid base", baseError: true},
		{name: "terminal-newline", citation: "feature/newline.go:2"},
		{name: "terminal-newline-no-extra-line", citation: "feature/newline.go:3", problem: "line 3 does not exist"},
		{name: "empty-blob", citation: "feature/empty.go:1", problem: "line 1 does not exist"},
		{name: "single-file", citation: "root.go:2"},
		{name: "ignored-code", citation: "feature/dirty.go", ignored: true, problem: "path absent"},
		{name: "ignored-local-config", citation: "metasystem.conf.local", ignored: true},
		{name: "prose-and-or", citation: "and/or", plain: true},
		{name: "prose-read-write", citation: "read/write", plain: true},
		{name: "prose-bare-go", citation: "renamed.go", plain: true},
		{name: "prose-bare-md", citation: "renamed.md", plain: true},
		{name: "prose-local-config", citation: "Never open any metasystem.conf.local", plain: true},
		{name: "prefixed-local-config", citation: "metasystem/metasystem.conf.local"},
		{name: "linked-local-config", citation: "[settings](metasystem.conf.local)"},
		{name: "plain-custom-path", citation: "feature/code.go", plain: true},
		{name: "plain-custom-absent", citation: "feature/moved.go", plain: true, problem: "path absent"},
		{name: "inline-bare-absent", citation: "renamed.go", problem: "path absent"},
		{name: "linked-unknown-absent", citation: "[source](unknown/code.go)", problem: "path absent"},
		{name: "plain-bare-line-absent", citation: "renamed.go:2", plain: true, problem: "path absent"},
		{name: "settings-shaped-brief", plain: true, citation: "Working Mode: Implement\n" +
			"Build only U1; Decisions 4 and 5 are other units.\n" +
			"Sites, read on this tree: feature/code.go:1 and feature/newline.go:1-2.\n" +
			"The settings reader handles read/write and/or policy choices; renamed.go and renamed.md are proposed names.\n" +
			"# Defect classes the reads keep finding (avoid each; the read checks them)\n" +
			"1. A refusal remedy that cannot succeed when followed, or that undoes the gate.\n" +
			"2. An agent given a person's power, or a person treated as an agent.\n" +
			"3. An older or records entry hiding current state.\n" +
			"4. A test seam hiding production behavior; every new function has a production caller.\n" +
			"# Check\nRun `go build ./...` and `go vet` on the packages changed.\n" +
			"Never open any metasystem.conf.local. Do not touch memory/ or records/. Leave uncommitted."},
		{name: "reviewed-base", citation: "feature/code.go:2"},
		{name: "short", citation: "feature/code.go:1-2"},
		{name: "prefixed", citation: "metasystem/feature/code.go:2"},
		{name: "absolute", citation: "ABS:2"},
		{name: "document-relative", citation: "./../../feature/code.go:2"},
		{name: "caller-relative", citation: "./feature/code.go:2"},
		{name: "linked-spaces", citation: "[source](../../feature/code file.go#L1-L2)"},
		{name: "quoted-spaces", citation: "\"feature/code file.go:2\""},
		{name: "plain-custom-directory", citation: "feature/code.go:2"},
		{name: "missing-line", citation: "feature/code.go:3", problem: "line 3 does not exist"},
		{name: "dirty-only-line", citation: "feature/code.go:4", problem: "line 4 does not exist"},
		{name: "zero-line", citation: "feature/code.go:0", problem: "invalid line range"},
		{name: "reversed-range", citation: "feature/code.go:2-1", problem: "invalid line range"},
		{name: "absent", citation: "feature/moved.go:12", problem: "path absent"},
		{name: "ambiguous", citation: "feature/code.go:2", ambiguous: true, problem: "ambiguous citation"},
		{name: "tree-error", citation: "feature/code.go:2", treeError: true, problem: "tree read"},
		{name: "start-error", citation: "feature/code.go:2", startError: true, problem: "process could not start"},
		{name: "blob-error", citation: "feature/code.go:2", blobError: true, problem: "corrupt blob"},
		{name: "outside", citation: "/outside-repository/code.go:2", problem: "outside the repository"},
		{name: "local-config", citation: "metasystem.conf.local:1", problem: "local configuration"},
		{name: "proposed-output", citation: "new file: feature/new.go:30"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			root := bed.root()
			install := filepath.Join(root, "metasystem")
			if err := os.MkdirAll(install, 0700); err != nil {
				t.Fatal(err)
			}
			configuration, err := os.ReadFile(filepath.Join(root, "metasystem.conf"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), configuration, 0600); err != nil {
				t.Fatal(err)
			}

			blobs := map[string]string{"metasystem/feature/code.go": "one\ntwo", "metasystem/feature/code file.go": "one\ntwo", "metasystem/feature/newline.go": "one\ntwo\n", "metasystem/feature/empty.go": "", "root.go": "one\ntwo"}
			if tc.ambiguous {
				blobs["feature/code.go"] = "other\nfile"
			}
			for name := range blobs {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("one\ntwo\nthree\nfour\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.ignored {
				path := filepath.Join(root, "feature/dirty.go")
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("dirty code\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			program := filepath.Join(t.TempDir(), "exit128")
			if err := testexec.WriteFile(program, []byte("#!/bin/sh\nexit 128\n"), 0700); err != nil {
				t.Fatal(err)
			}
			_, exit := exec.Command(program).Output()
			if exit == nil {
				t.Fatal("fixture must return Git's nonzero process status")
			}
			treeListings := 0
			bed.workOwnersHook = func(owners *intentWorkOwners) {
				previous := owners.git
				owners.git = func(dir string, args ...string) ([]byte, error) {
					if args[0] == "cat-file" && strings.Contains(strings.Join(args, " "), "metasystem.conf.local") {
						t.Fatalf("local configuration reached Git: %v", args)
					}
					switch args[0] {
					case "rev-parse":
						if strings.Join(args, " ") == "rev-parse --show-prefix" {
							return []byte("metasystem/\n"), nil
						}
						if strings.Join(args, " ") == "rev-parse --verify HEAD^{commit}" {
							if tc.baseError {
								return nil, fmt.Errorf("fatal: invalid base: %w", exit)
							}
							return []byte("base-commit\n"), nil
						}
					case "ls-tree":
						if args[1] == "-d" {
							return []byte("metasystem\nfeature\n"), nil
						}
						treeListings++
						if tc.treeError {
							return nil, fmt.Errorf("fatal: corrupt tree: %w", exit)
						}
						var names []string
						for name := range blobs {
							names = append(names, name)
						}
						sort.Strings(names)
						return []byte(strings.Join(names, "\x00") + "\x00"), nil
					case "cat-file":
						name := strings.TrimPrefix(args[2], "base-commit:")
						if args[1] == "-e" {
							if tc.startError {
								return nil, fmt.Errorf("process could not start")
							}
							if _, ok := blobs[name]; ok && !tc.treeError && !tc.catError {
								return nil, nil
							}
							return nil, fmt.Errorf("fatal: object missing: %w", exit)
						}
						if tc.blobError {
							return nil, fmt.Errorf("corrupt blob: %w", exit)
						}
						if value, ok := blobs[name]; ok {
							return []byte(value), nil
						}
						t.Fatalf("blob read outside the declared base: %v", args)
					case "check-ignore":
						return nil, exit
					}
					return previous(dir, args...)
				}
			}
			citation := strings.ReplaceAll(tc.citation, "ABS", filepath.Join(install, "feature/code.go"))
			if !tc.plain && !strings.HasPrefix(citation, "[") && !strings.HasPrefix(citation, "\"") && !strings.HasPrefix(citation, "new file:") && tc.name != "plain-custom-directory" {
				citation = "`" + citation + "`"
			}
			home := filepath.Join(bed.stateRoot(), "plans/designs")
			relative, err := filepath.Rel(home, filepath.Join(install, "feature"))
			if err != nil {
				t.Fatal(err)
			}
			citation = strings.ReplaceAll(citation, "../../feature", filepath.ToSlash(relative))
			body := "## Constraints\n\nRead " + citation + ".\n"
			if err := os.MkdirAll(home, 0700); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(home, "citation.md")
			design := "# Citation\n\n- Kind: design\n- Id: citation-design\n- Status: accepted\n- Goals: " + bed.id + "\n\n" + body
			if err := os.WriteFile(source, []byte(design), 0600); err != nil {
				t.Fatal(err)
			}
			owners := bed.workOwners()
			owners.dependencies.endpoint = func(selected string) (goal.Endpoint, error) {
				if selected != install {
					t.Fatalf("endpoint root = %q, want installation %q", selected, install)
				}
				return goal.Endpoint{Root: selected, Remote: "local", Branch: "refs/heads/main", Repository: bed.repo}, nil
			}
			layout, err := owners.resolver.ResolveLayout(install)
			if err != nil || layout.InstallationRoot.Path() != install || layout.GitRoot != root {
				t.Fatalf("nested installation: %+v, %v", layout, err)
			}
			if tc.person {
				tree := person()
				if _, err := humanauthority.Enroll(root, 20, tree, "Wido", helmNow); err != nil {
					t.Fatal(err)
				}
				owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
					return humanauthority.Prove(root, 20, tree, at)
				}
			}
			code, result := bed.runJSON(owners, "work", "brief", bed.id, "--out", "composed.md", "--repo", install)
			if treeListings > 1 {
				t.Fatalf("work brief listed the same base tree %d times", treeListings)
			}
			if tc.problem == "" || tc.person {
				if code != 0 {
					t.Fatalf("work brief exit=%d: %+v", code, result)
				}
				written, err := os.ReadFile(filepath.Join(root, "composed.md"))
				carried := citation
				if tc.name == "settings-shaped-brief" {
					carried, _, _ = strings.Cut(citation, "# Defect classes")
				}
				if err != nil || !strings.Contains(string(written), carried) {
					t.Fatalf("citation not carried: %q, %v", written, err)
				}
			} else {
				if code != 1 || !strings.Contains(resultWords(result), tc.problem) {
					t.Fatalf("work brief exit=%d: %+v, want %q", code, result, tc.problem)
				}
				if _, err := os.Stat(filepath.Join(root, "composed.md")); !os.IsNotExist(err) {
					t.Fatalf("invalid citation published output: %v", err)
				}
				if tc.treeError || tc.startError || tc.catError || tc.baseError || tc.blobError {
					if result.Next == nil || !strings.Contains(result.Next.Reason, "base tree is readable") {
						t.Fatalf("tree failure remedy edits the source: %+v", result.Next)
					}
				} else if result.Next == nil || !strings.Contains(result.Next.Reason, "correcting the named citation") {
					t.Fatalf("citation remedy: %+v", result.Next)
				}
				if tc.name != "start-error" && tc.name != "tree-error" && tc.name != "invalid-base" && !strings.Contains(resultWords(result), "base-commit") {
					t.Fatalf("refusal omits exact base: %+v", result)
				}
			}
			brief := filepath.Join(root, bed.brief("request.md", body))
			// Give the design caller the same document-relative context as the
			// accepted source page, while retaining the public caller directory.
			if tc.name == "document-relative" || tc.name == "linked-spaces" {
				brief = filepath.Join(home, "request.md")
				if err := os.WriteFile(brief, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			author := fakeAuthor(t, "A draft design.\n")
			bed.starter.author = func(record launch.Record) {
				if tc.person {
					content, err := os.ReadFile(record.Inputs[0].Path)
					if err != nil || !strings.Contains(string(content), "Unchecked citation evidence:") || !strings.Contains(string(content), tc.problem) {
						t.Errorf("unchecked evidence not retained: %q, %v", content, err)
					}
				}
				author(record)
			}
			before := len(bed.starter.launched())
			treeListings = 0
			code, result = bed.runJSON(owners, "design", "write", bed.id, "--brief", brief, "--out", filepath.Join(home, "new-design.md"), "--repo", install)
			if treeListings > 1 {
				t.Fatalf("design write listed the same base tree %d times", treeListings)
			}
			if tc.problem == "" || tc.person {
				if code != 0 || len(bed.starter.launched()) != before+1 {
					t.Fatalf("design write exit=%d: %+v; launches=%v", code, result, bed.starter.launched())
				}
			} else if code != 1 || !strings.Contains(resultWords(result), tc.problem) || len(bed.starter.launched()) != before {
				t.Fatalf("design write exit=%d: %+v; launches=%v", code, result, bed.starter.launched())
			}
			// Exercise dispatch's actual reader with this same Git port too.
			git := bed.workOwners().work.git
			treeListings = 0
			if _, err := dispatchcore.ReadReviewBriefAdmission(brief, install, root, root, "", func(dir string, args ...string) (string, error) { b, e := git(dir, args...); return string(b), e }); (err != nil) != (tc.problem != "") {
				t.Fatalf("dispatch admission: %v, problem=%q", err, tc.problem)
			}
			if treeListings > 1 {
				t.Fatalf("dispatch admission listed the same base tree %d times", treeListings)
			}
			if tc.name == "reviewed-base" {
				subject := filepath.Join(root, "subject.md")
				if err := os.WriteFile(subject, []byte("Working Mode: implement\nRead `feature/code.go:3`.\n"), 0600); err != nil {
					t.Fatal(err)
				}
				_, err := dispatchcore.ReadReviewBriefAdmission(subject, install, root, root, "commit:reviewed-commit", func(dir string, args ...string) (string, error) {
					if strings.Join(args, " ") == "rev-parse --verify --end-of-options reviewed-commit^{commit}" {
						return "reviewed-commit", nil
					}
					args = append([]string(nil), args...)
					if args[0] == "cat-file" {
						if args[1] == "blob" && strings.HasPrefix(args[2], "base-commit:") {
							return "one\ntwo\nthree", nil
						}
						args[2] = strings.Replace(args[2], "reviewed-commit:", "base-commit:", 1)
					}
					b, e := git(dir, args...)
					return string(b), e
				})
				if err == nil || !strings.Contains(err.Error(), "line 3 does not exist") || !strings.Contains(err.Error(), "base reviewed-commit") {
					t.Fatalf("review borrowed a line from HEAD: %v", err)
				}
			}

		})
	}
}
