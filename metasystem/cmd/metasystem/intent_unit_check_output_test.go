package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// These fixtures retain the public legacy output, including stderr and exit
// status. Both an omitted kind and an explicit legacy declaration use it.
func TestUnitCheckLegacyOutput(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"default", "legacy"} {
		for _, row := range []struct {
			name         string
			cheap, audit int
		}{{"pass", 0, 0}, {"test-failure", 9, 0}, {"audit-failure", 0, 17}} {
			for _, verbose := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/verbose=%t", kind, row.name, verbose), func(t *testing.T) {
					t.Parallel()
					body := fmt.Sprintf("proof.cheap=printf 'cheap fixture\\n'; exit %d\nproof.audits=printf 'audit fixture\\n'; exit %d\nproof.deadline=15\n", row.cheap, row.audit)
					if kind == "legacy" {
						body += "proof.check-kind=legacy\n"
					}
					b := declaredCheckBed(t, body)
					code, built, _ := b.work("work", "build", b.id, "legacy-output", "--brief", b.brief("build.md", "Build.\n"), "--lines", "100")
					if code != 0 {
						t.Fatalf("prepare frozen check: exit %d", code)
					}
					args := []string{"test", "run", "--unit-run", resultData(t, built)["run"].(string)}
					if verbose {
						args = append(args, "--verbose")
					}
					command, rest, ok := resolveIntentArgv(args)
					if !ok {
						t.Fatal("public check route is unavailable")
					}
					var stdout, stderr bytes.Buffer
					owners := b.workOwners()
					git := owners.work.git
					owners.work.git = func(root string, args ...string) ([]byte, error) {
						if slices.Equal(args, []string{"rev-parse", "--show-toplevel"}) {
							return []byte(b.root()), nil
						}
						return git(root, args...)
					}
					owners.lookupEnv = func(string) (string, bool) { return "", false }
					writeTestingFixtureFile(t, filepath.Join(b.root(), "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0600)
					code = runIntentIn(command, rest, &stdout, &stderr, b.root(), owners)
					wantCode := 0
					if row.cheap != 0 || row.audit != 0 {
						wantCode = 1
					}
					if code != wantCode || !bytes.Contains(stderr.Bytes(), []byte("cheap fixture\naudit fixture\n")) {
						t.Fatalf("check commands did not finish as declared: exit=%d want=%d", code, wantCode)
					}
					got := struct {
						Code           int
						Stdout, Stderr string
					}{code, stdout.String(), stderr.String()}
					actual, err := json.MarshalIndent(got, "", "  ")
					if err != nil {
						t.Fatal(err)
					}
					actual = append(actual, '\n')
					fixture := filepath.Join("testdata", "legacy-check-output", fmt.Sprintf("%s-verbose-%t.json", row.name, verbose))
					want, err := os.ReadFile(fixture)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(actual, want) {
						t.Fatalf("legacy public output differs from fixture %s: exit %d; stdout matches=%t; stderr bytes=%d", fixture, code, bytes.Contains(want, []byte(fmt.Sprintf("%q", stdout.String()))), stderr.Len())
					}
				})
			}
		}
	}
}
