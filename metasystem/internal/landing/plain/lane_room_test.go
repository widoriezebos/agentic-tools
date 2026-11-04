package plain

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/shellquote"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestTheLanesCheckCommandsHoldItsSlot(t *testing.T) {
	t.Parallel()
	for _, row := range []string{"ordinary", "person", "gate baseline", "classification and full"} {
		t.Run(row, func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			order := filepath.Join(b.checkout, "room-order")
			appendOrder := func(line string) {
				file, err := os.OpenFile(order, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
				if err != nil {
					t.Fatal(err)
				}
				_, err = fmt.Fprintln(file, line)
				closeErr := file.Close()
				if err != nil || closeErr != nil {
					t.Fatalf("order: %v %v", err, closeErr)
				}
			}
			file, err := os.CreateTemp(t.TempDir(), "custody")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			b.seams.Room = func(install string, person bool, _ io.Writer) (func(*exec.Cmd), func()) {
				if install != b.install {
					t.Fatalf("room installation = %s", install)
				}
				appendOrder(fmt.Sprintf("reserve %t", person))
				return func(command *exec.Cmd) {
					command.ExtraFiles = []*os.File{file}
					command.Env = append(command.Env, "METASYSTEM_PROOF_RESOURCE_FDS=3@custody")
				}, func() { appendOrder("release") }
			}
			command := "set -e; test -e /dev/fd/3; test \"$METASYSTEM_PROOF_RESOURCE_FDS\" = '3@custody'; printf 'check\\n' >> " + shellquote.Token(order) + "; printf 'LANDING-CHECKED\\t0\\n'"
			checks := 1
			if row == "person" || row == "classification and full" {
				b.seams.Person = &ActProvenance{Person: "fixture", Kind: "proof"}
			}
			if row == "gate baseline" {
				b.seams.Gate = true
				git := b.seams.Git
				b.seams.Git = func(dir string, args ...string) (string, error) {
					switch strings.Join(args, " ") {
					case "show -s --format=%P commit":
						return "parent goal", nil
					case "rev-parse --verify parent^{tree}":
						return "parent-tree", nil
					}
					return git(dir, args...)
				}
				engine := filepath.Join(b.checkout, "gate-engine")
				// Plan reads are separate from check commands and return only selection.
				script := "#!/bin/sh\nif test \"$2\" = impact; then\nprintf '%s\\n' '{\"schemaVersion\":1,\"verb\":\"test impact\",\"outcome\":\"confirmed\",\"data\":{\"base\":\"parent\",\"subject\":\"fixture\",\"selections\":[]}}'\nelse\n" + command + "; printf 'landing group fast-static-build green 1\\n'\nfi\n"
				if err := testexec.WriteFile(engine, []byte(script), 0o700); err != nil {
					t.Fatal(err)
				}
				b.seams.Executable = func() (string, error) { return engine, nil }
				b.seams.ImpactCost = func(string, string) (int, bool, int, string, error) { return 1, true, 1, "", nil }
				checks = 4 // Static and declared checks on both baseline and merge.
			}
			if row == "classification and full" {
				git := b.seams.Git
				b.seams.Git = func(dir string, args ...string) (string, error) {
					if strings.Join(args, " ") == "rev-parse --verify parent^{tree}" {
						return "parent-tree", nil
					}
					return git(dir, args...)
				}
				b.seams.Judge = nil
				command += "; printf '%s' " + shellquote.Quote(plainTestEvents("u/a", []string{"TestA"}, "pass"))
				b.seams.CommandForCommit = func(string) (string, error) { return command, nil }
				log := filepath.Join(b.checkout, "saved-red.log")
				if err := os.WriteFile(log, []byte(plainTestEvents("u/a", []string{"TestA"}, "fail")), 0o600); err != nil {
					t.Fatal(err)
				}
				red := Result{Attempt: "saved-red", Commit: "commit", Tree: "tree", Result: Red, Scope: "full", Log: log, Cause: &Cause{Kind: "unclassified"}, Failed: []FailedUnit{{Unit: "u/a", Tests: []string{"TestA"}}}}
				if err := os.MkdirAll(Dir(b.install), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := appendLine(resultsPath(b.install), red); err != nil {
					t.Fatal(err)
				}
				classified, err := ClassifyAttempt(b.install, b.checkout, red.Attempt, b.seams)
				if classified.NextFull != nil && classified.NextFull.Result != Green {
					t.Fatalf("classification's full check: %+v", *classified.NextFull)
				}
				if err != nil || classified.NextFull == nil || classified.NextFull.Result != Green {
					t.Fatalf("classification did not replay and run full: %+v %v", classified, err)
				}
				checks = 3 // Named replay, whole-unit confirmation, then full check.
			} else if got := b.run(command); got.Result != Green {
				t.Fatalf("lane check: %+v", got)
			}
			data, err := os.ReadFile(order)
			person := row == "person" || row == "classification and full"
			want := fmt.Sprintf("reserve %t\n", person) + strings.Repeat("check\n", checks) + "release\n"
			if err != nil || string(data) != want {
				t.Fatalf("reservation custody order = %q, want %q: %v", data, want, err)
			}
		})
	}
}
