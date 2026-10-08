package main

import (
	"crypto/sha1"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestWorkBriefShowsReadersDeletionAndEffort(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"complete", "blob failure", "batch failure", "tree failure", "no deletion", "unclear deletion", "B3 wording", "B5 wording", "identifier noise"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			program := filepath.Join(t.TempDir(), "missing-object")
			if err := testexec.WriteFile(program, []byte("#!/bin/sh\nexit 128\n"), 0700); err != nil {
				t.Fatal(err)
			}
			_, absent := exec.Command(program).Output()
			if absent == nil {
				t.Fatal("missing Git object fixture must exit nonzero")
			}
			blobs := map[string]string{
				"feature/declaration.go":      "package feature\nfunc PurgeEntry() {}\n",
				"elsewhere/consumer.go":       "package elsewhere\nPurgeEntry(); PurgeEntry()\nlaunch.build.effort\nPlainReader RowReader Qualified.Reader\n",
				"feature/declaration_test.go": "package feature\nPurgeEntry()\n",
				"other/untouched.go":          "OtherUnitReader\n",
				"noise.txt":                   "highlight xhigh PurgeEntryExtra prefixPurgeEntry PurgeEntry_suffix\n",
				"binary.bin":                  "PurgeEntry\x00",
				"metasystem.conf.local":       "PurgeEntry secret",
				"artifacts/ignored.txt":       "PurgeEntry artifact",
			}
			var scanned []string
			baseReads, batchReads := 0, 0
			bed.workOwnersHook = func(owners *intentWorkOwners) {
				previous := owners.git
				owners.git = func(dir string, args ...string) ([]byte, error) {
					switch strings.Join(args, " ") {
					case "rev-parse --show-prefix":
						return nil, nil
					case "rev-parse --verify HEAD^{commit}":
						baseReads++
						if baseReads > 1 {
							return []byte("moved-commit\n"), nil
						}
						return []byte("base-commit\n"), nil
					}
					if args[0] == "ls-tree" {
						if args[1] == "-d" {
							return []byte("feature\nelsewhere\n"), nil
						}
						if scenario == "tree failure" {
							return nil, fmt.Errorf("cannot read tree")
						}
						var names []string
						for name := range blobs {
							names = append(names, fmt.Sprintf("100644 blob %x\t%s", sha1.Sum([]byte(name)), name))
						}
						return []byte(strings.Join(names, "\x00") + "\x00"), nil
					}
					if args[0] == "cat-file" {
						if strings.Contains(args[2], "metasystem.conf.local") {
							t.Fatalf("local configuration reached Git: %v", args)
						}
						if !strings.HasPrefix(args[2], "base-commit:") {
							t.Fatalf("scan borrowed another base: %v", args)
						}
						name := strings.TrimPrefix(args[2], "base-commit:")
						if args[1] == "-e" {
							if _, ok := blobs[name]; !ok {
								return nil, fmt.Errorf("object missing: %w", absent)
							}
							return nil, nil
						}
						if _, ok := blobs[name]; !ok {
							t.Fatalf("unexpected base blob: %v", args)
						}
						t.Fatalf("reader scan used a per-file call: %v", args)
						return nil, nil
					}
					return previous(dir, args...)
				}
				owners.gitInput = func(_ string, input []byte, args ...string) ([]byte, error) {
					batchReads++
					if batchReads != 1 || !slices.Equal(args, []string{"cat-file", "--batch"}) {
						t.Fatalf("readers did not use one batch: %v (%d calls)", args, batchReads)
					}
					if scenario == "batch failure" {
						return nil, fmt.Errorf("batch process failed: %w", absent)
					}
					var output strings.Builder
					for _, object := range strings.Fields(string(input)) {
						name := ""
						for candidate := range blobs {
							if fmt.Sprintf("%x", sha1.Sum([]byte(candidate))) == object {
								name = candidate
							}
						}
						if name == "" || name == "metasystem.conf.local" || strings.HasPrefix(name, "artifacts/") {
							t.Fatalf("unexpected or excluded batch object: %q (%q)", object, name)
						}
						scanned = append(scanned, name)
						if scenario == "blob failure" && name == "elsewhere/consumer.go" {
							fmt.Fprintf(&output, "%s missing\n", object)
							continue
						}
						fmt.Fprintf(&output, "%s blob %d\n%s\n", object, len(blobs[name]), blobs[name])
					}
					return []byte(output.String()), nil
				}
			}
			intent := "Deletes: Delete only `feature/old.go`; protected paths: `feature/keep.go`."
			if scenario == "tree failure" {
				intent = "Deletes: Delete only ids `retired`; protected ids: `standing`."
			}
			if scenario == "no deletion" {
				intent = "Inspect the existing behavior. Do not delete any files."
			}
			if scenario == "unclear deletion" {
				intent = "| Deletion | Delete obsolete entries. |"
			}
			if scenario == "B3 wording" {
				intent = "Carry explicit unit deletion intent from its Decision/constraints into Deletion rules (empty ids refuse, errors are not discarded, paths never widen, protected paths are explicit, dry run precedes); unclear deletion scope is a missing decision, and no files are deleted by composition."
			}
			if scenario == "B5 wording" {
				intent = "Emit the mechanism's ordered sections: Goal, correction Decisions, Workspace, selected Units/size, What this unit builds, Not in this unit, Readers, A test through the public verb, Check, optional Deletion rules, Constraints including B4's fallback or record-derived “avoid these” and effort, Expected Return, Acceptance Criteria.\nThis unit adds no roster audit, new spending authority, deletion executor or reference database."
			}
			if scenario == "identifier noise" {
				intent = "Configured `launch.build.effort` values are `high` and `xhigh`."
				blobs["many.txt"] = strings.Repeat("PurgeEntry\n", 300)
			}
			source := "# Reader design\n\n- Kind: design\n- Id: reader-design\n- Status: accepted\n- Goals: " + bed.id + "\n\n" +
				"## Units\n\n| Unit | Lines | Readers |\n| --- | ---: | --- |\n| u1 | 20 | RowReader |\n| u2 | 20 | OtherUnitReader |\n\n" +
				"## u1 — Reader unit\n\nInspect `PurgeEntry`, `launch.build.effort` and `UnknownReader`; new `ProposedReader`. Qualified.Reader is also cited. Commands `go`, `git`, `go test ./...` and number `123` are not identifiers.\n" + intent + "\n\n" +
				"## u2 — Other unit\n\nInspect `OtherUnitReader`.\n\n" +
				"## Readers\n\n| Unit | Names |\n| --- | --- |\n| u1 | PlainReader |\n| u2 | OtherUnitReader |\n\n" +
				"## Acceptance\n\nObserve the named readers.\n"
			// Deletion targets are output intent rather than existing code claims.
			blobs["feature/old.go"], blobs["feature/keep.go"] = "old\n", "keep\n"
			home := filepath.Join(bed.stateRoot(), "plans/designs")
			if err := os.MkdirAll(home, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, "reader.md"), []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			// Dirty content must never supply reference evidence.
			if err := os.WriteFile(filepath.Join(bed.root(), "dirty.go"), []byte("PurgeEntry\n"), 0600); err != nil {
				t.Fatal(err)
			}
			code, result, _ := bed.work("work", "brief", bed.id, "--work", "u1", "--out", "composed.md")
			if code != 0 {
				t.Fatalf("brief exit %d: %+v", code, result)
			}
			if baseReads != 1 {
				t.Fatalf("citations and readers did not share one base: %d resolutions", baseReads)
			}
			written, err := os.ReadFile(filepath.Join(bed.root(), "composed.md"))
			if err != nil {
				t.Fatal(err)
			}
			text := string(written)
			for _, absent := range []string{"OtherUnitReader →", "dirty.go:", "binary.bin:", "artifacts/ignored.txt:", "metasystem.conf.local:", "secret", "noise.txt:"} {
				if strings.Contains(text, absent) {
					t.Errorf("unselected or non-source match %q", absent)
				}
			}
			for _, want := range []string{"Composer effort: empty", "base base-commit", "not a proved call graph"} {
				if !strings.Contains(text, want) {
					t.Errorf("missing %q: %s", want, text)
				}
			}
			if scenario == "tree failure" {
				if strings.Contains(text, "go: unresolved") || strings.Contains(text, "git: unresolved") || strings.Contains(text, "123: unresolved") {
					t.Fatal("command or number searched as an identifier")
				}
				if !strings.Contains(text, "Coverage incomplete: cannot list") {
					t.Fatalf("failed scan claimed complete: %s", text)
				}
			} else {
				proposed := "ProposedReader: no existing match; proposed name"
				if scenario == "blob failure" || scenario == "batch failure" {
					proposed = "ProposedReader: no observed match; proposed name; coverage incomplete"
				}
				wants := []string{proposed, "UnknownReader: unresolved-reader warning"}
				if scenario != "batch failure" {
					wants = append(wants, "PurgeEntry → feature/declaration.go:2", "PurgeEntry → feature/declaration_test.go:2", "Skipped binary blob")
				}
				for _, want := range wants {
					if !strings.Contains(text, want) {
						t.Errorf("missing %q: %s", want, text)
					}
				}
				if scenario == "batch failure" {
					if !strings.Contains(text, "Coverage incomplete: cannot read base blobs") {
						t.Fatal("failed batch hidden")
					}
				} else if scenario == "blob failure" {
					if !strings.Contains(text, "Coverage incomplete: cannot read elsewhere/consumer.go") {
						t.Fatalf("failed blob hidden: %s", text)
					}
				} else {
					for _, want := range []string{"PurgeEntry → elsewhere/consumer.go:2", "PlainReader → elsewhere/consumer.go:4", "RowReader → elsewhere/consumer.go:4", "Qualified.Reader → elsewhere/consumer.go:4", "launch.build.effort → elsewhere/consumer.go:3"} {
						if strings.Count(text, want) != 1 {
							t.Errorf("missing or duplicate %q: %s", want, text)
						}
					}
				}
			}
			if scenario != "tree failure" && batchReads != 1 {
				t.Fatalf("scan made %d batches, want one", batchReads)
			}
			if scenario == "complete" {
				previous := -1
				for _, path := range []string{"elsewhere/consumer.go", "feature/declaration.go", "feature/declaration_test.go"} {
					index := strings.Index(text, "References in "+path+":")
					if index < 0 || index <= previous {
						t.Fatalf("reader matches not grouped and sorted by file: %q", path)
					}
					previous = index
				}
			}
			if slices.Contains(scanned, "artifacts/ignored.txt") {
				t.Fatal("scan read ignored artifacts")
			}
			if scenario == "no deletion" || scenario == "B3 wording" || scenario == "B5 wording" || scenario == "identifier noise" {
				if !strings.Contains(text, "This unit declares no deletion intent") || strings.Contains(text, "A dry run must") {
					t.Fatalf("non-deleting unit gained deletion rules: %s", text)
				}
			} else {
				for _, want := range []string{intent, "Refuse empty ids", "Report errors", "Never widen declared paths", "Name protected paths explicitly", "A dry run must precede deletion"} {
					if !strings.Contains(text, want) {
						t.Errorf("missing deletion rule %q", want)
					}
				}
				if scenario == "unclear deletion" && !strings.Contains(text, "MISSING DECISION: the deletion scope") {
					t.Fatal("unclear scope became deletion authority")
				}
			}
			if scenario == "identifier noise" {
				if strings.Contains(text, "high →") || strings.Contains(text, "high: unresolved") || strings.Contains(text, "high, ") || strings.Contains(text, "xhigh") {
					t.Fatal("setting values searched as identifiers")
				}
				if strings.Count(text, "PurgeEntry → many.txt:") != 300 {
					t.Fatal("true reader matches were truncated")
				}
			}
			if scenario != "complete" && scenario != "B3 wording" && scenario != "B5 wording" {
				return
			}
			for index, effort := range []string{launch.DefaultSettings().BuildEffort, "high", "low"} {
				bed.manager.Settings.BuildEffort = effort
				args := []string{"work", "build", bed.id, "--work", fmt.Sprintf("build-%d", index), "--brief", "composed.md", "--lines", "20"}
				want := effort
				if index == 2 {
					args, want = append(args, "--effort", "xhigh"), "xhigh"
				}
				code, built, _ := bed.work(append(args, workCheck...)...)
				if code != 0 {
					t.Fatalf("build exit %d: %+v", code, built)
				}
				data := resultData(t, built)
				if index != 2 && data["buildEffort"] != "" {
					t.Fatalf("composer overrode configured effort: %+v", built)
				}
				id := data["steps"].([]any)[0].(map[string]any)["launchId"].(string)
				record, err := bed.manager.Store.Read(id)
				if err != nil || string(record.AdapterData["effort"]) != fmt.Sprintf("%q", want) {
					t.Fatalf("launch effort %s, want %q: %v", record.AdapterData["effort"], want, err)
				}
			}
		})
	}
}

// The native adapter must send object ids to Git's batch stdin and parse its
// byte-sized responses. A stub cannot establish that process boundary.
func TestWorkBriefBatchGitAdapterIntegration(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("native Git adapter requires Git")
	}
	native := (&intentInvocation{}).work()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rootBytes, err := native.git(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatal(err)
	}
	root := strings.TrimSpace(string(rootBytes))
	module, err := filepath.Rel(root, filepath.Dir(filepath.Dir(cwd)))
	if err != nil {
		t.Fatal(err)
	}
	bed := newWorkBed(t)
	bed.workOwnersHook = func(owners *intentWorkOwners) {
		previous := owners.git
		owners.git = func(dir string, args ...string) ([]byte, error) {
			if args[0] == "ls-tree" || strings.Join(args, " ") == "rev-parse --verify HEAD^{commit}" {
				return native.git(root, args...)
			}
			if strings.Join(args, " ") == "rev-parse --show-prefix" {
				return nil, nil
			}
			return previous(dir, args...)
		}
		owners.gitInput = func(_ string, input []byte, args ...string) ([]byte, error) {
			return native.gitInput(root, input, args...)
		}
	}
	source := "# Native reader design\n\n- Kind: design\n- Id: native-reader\n- Status: accepted\n- Goals: " + bed.id + "\n\n" +
		"## Units\n\n| Unit | Lines |\n| --- | ---: |\n| u1 | 20 |\n\n" +
		"## u1 — Readers\n\nInspect `launch.build.effort`; its values are `high` and `xhigh`.\n\n" +
		"## Acceptance\n\nReport the actual reader.\n"
	home := filepath.Join(bed.stateRoot(), "plans/designs")
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "reader.md"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	code, result, _ := bed.work("work", "brief", bed.id, "--work", "u1", "--out", "native.md")
	if code != 0 {
		t.Fatalf("native brief exit %d: %+v", code, result)
	}
	written, err := os.ReadFile(filepath.Join(bed.root(), "native.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "launch.build.effort → " + filepath.ToSlash(filepath.Join(module, "internal/config/defaults.go")) + ":"
	if !strings.Contains(string(written), want) || strings.Contains(string(written), "Coverage incomplete") {
		t.Fatalf("native batch did not report the committed setting declaration %q", want)
	}
}
