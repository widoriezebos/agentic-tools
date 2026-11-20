package goal

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A receiver may unpack a small push or keep its pack. Only Git creates those names.
func TestLedgerDurabilityGitAdapter(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"local", "pack", "nested", "external", "uncreated", "copied", "injected", "recovered", "recovered-doubt", "doubt"} {
		t.Run(mode, func(t *testing.T) {
			origin, root := oneClone(t)
			e := endpointFor(root)
			dest, ref := origin, e.Branch
			if mode == "local" || mode == "doubt" || mode == "injected" {
				e.Remote, dest, ref = "local", root, LocalLedgerBranch
			} else if mode == "pack" {
				mustGit(t, origin, "config", "receive.unpackLimit", "1")
			}
			if mode != "external" {
				const id = "01J5X0000000000000000000K0"
				dir := filepath.Join(root, "artifacts", "agents", "ui", "launches")
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				record := fmt.Sprintf(`{"launch":%q,"destination":%q,"created":{"destination":%t}}`, id, dest, mode != "uncreated")
				if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(record), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "nested" {
				e.Root = filepath.Join(root, "metasystem")
				if err := os.Mkdir(e.Root, 0755); err != nil {
					t.Fatal(err)
				}
				relative, err := filepath.Rel(root, origin)
				if err != nil {
					t.Fatal(err)
				}
				mustGit(t, root, "remote", "set-url", "origin", relative)
			}
			if mode == "copied" {
				mustGit(t, root, "add", "-f", "artifacts/agents/ui/launches")
			}
			if mode == "injected" {
				e.Repository = e.repository()
				e = e.WithCASPublisher(func(endpoint Endpoint, parent, commit string) (CASOutcome, error) {
					return endpoint.Repository.Publish(parent, commit)
				})
			}
			if strings.HasPrefix(mode, "recovered") {
				e = e.WithCASPublisher(func(endpoint Endpoint, parent, commit string) (CASOutcome, error) {
					endpoint.casPublisher = nil
					if outcome, err := PublishCAS(endpoint, parent, commit); outcome != CASLanded {
						return outcome, err
					}
					return CASUnknown, fmt.Errorf("acknowledgement lost")
				})
			}
			dirs := map[string]bool{}
			var warnings bytes.Buffer
			e.durabilityWarnings = &warnings
			e.syncDirectory = func(path string) error {
				dirs[path] = true
				if strings.HasSuffix(mode, "doubt") {
					return fmt.Errorf("injected sync error")
				}
				return nil
			}
			result, err := Publish(e, PublishRequest{Opid: "small-update", Machine: "seat", Lineage: "test", Intent: testIntentFor("open"), Message: "record payload", Mutate: func(string) ([]Change, error) {
				return []Change{{Path: "ledger-payload", Content: []byte("payload\n")}}, nil
			}})
			if strings.HasPrefix(mode, "recovered") {
				if err == nil || len(dirs) != 0 {
					t.Fatal("lost acknowledgement should await recovery")
				}
				reports, err := Recover(e)
				if err != nil || len(reports) != 1 || reports[0].Action != ActionConfirm {
					t.Fatal(reports, err)
				}
			} else if err != nil || result.Outcome != OutcomeConfirmed {
				t.Fatalf("%+v %v", result, err)
			}
			payloadPath := "ledger-payload"
			if mode == "nested" {
				payloadPath = "metasystem/ledger-payload"
			}
			payload := readOperation(t, dest, ref, "small-update", payloadPath)
			if payload != "payload\n" || mustGit(t, dest, "rev-parse", ref) != result.Commit {
				t.Fatal("receiving ledger differs")
			}
			if runtime.GOOS != "linux" || mode == "external" || mode == "uncreated" || mode == "copied" {
				if len(dirs) != 0 || warnings.Len() != 0 {
					t.Fatal(dirs, warnings.String())
				}
				return
			}
			if strings.HasSuffix(mode, "doubt") {
				if strings.Count(warnings.String(), "published; durability in doubt") != 1 || !strings.Contains(warnings.String(), dest) {
					t.Fatal(warnings.String())
				}
				return
			}
			if warnings.Len() != 0 {
				t.Fatal(warnings.String())
			}
			common := mustGit(t, dest, "rev-parse", "--path-format=absolute", "--git-common-dir")
			want := []string{".", "objects", filepath.Dir(ref), "refs", "refs/heads"}
			if mode == "pack" {
				want = append(want, "objects/pack")
			} else {
				for _, obj := range []string{result.Commit, result.Commit + "^{tree}", result.Commit + ":" + payloadPath} {
					want = append(want, "objects/"+mustGit(t, dest, "rev-parse", obj)[:2])
				}
			}
			for _, d := range want {
				if !dirs[filepath.Join(common, d)] {
					t.Errorf("missing %s in %v", d, dirs)
				}
			}
		})
	}
}

// readOperation follows the named receiving reference through the ledger commit
// and its tree to the payload. The crash rehearsal can use the same fixture reader.
func readOperation(t *testing.T, root, ref, opid, path string) string {
	t.Helper()
	commit, err := commitWithTrailer(root, ref, "Goal-Transaction", opid)
	if err != nil || commit == "" {
		t.Fatalf("operation %s on %s: %s %v", opid, ref, commit, err)
	}
	payload, err := goalGit(root, nil, "--no-lazy-fetch", "--no-replace-objects", "cat-file", "blob", commit+":"+path)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}
