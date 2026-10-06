package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func TestLandingProveRunsCommittedFullAndSettingsShowExplainsIgnoredLocal(t *testing.T) {
	t.Parallel()
	b := newResolveVerbFixture(t)
	conf := filepath.Join(b.install, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("proof.full=printf committed-proof\nproof.cheap=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conf+".local", []byte("proof.full=exit 9\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	b.owners.landing.plainProve = plain.ProveSeams{Now: func() time.Time { return laneTestNow }, Git: func(_ string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --verify HEAD^{commit}":
			return "head", nil
		case "rev-parse --verify HEAD^{tree}":
			return "tree", nil
		case "show head:metasystem/metasystem.conf":
			return "proof.full=printf committed-proof\nproof.cheap=true\n", nil
		case "cat-file -e head^{commit}", "worktree prune":
			return "", nil
		case "show origin/main:metasystem/testing.json", "show origin/main:metasystem/plans/goals/trunk-red.json":
			return "", errors.New("not declared")
		}
		if len(args) == 5 && args[0] == "worktree" && args[1] == "add" {
			return "", os.MkdirAll(filepath.Join(args[3], "metasystem"), 0o755)
		}
		if len(args) == 4 && args[0] == "worktree" && args[1] == "remove" {
			return "", os.RemoveAll(args[3])
		}
		return "", fmt.Errorf("unexpected stub Git: %v", args)
	}}
	code, output := b.run(t, b.root, "prove", "--wait", "--json")
	var result struct{ Data plain.Result }
	if err := json.Unmarshal([]byte(output), &result); err != nil || code != 0 || result.Data.Result != plain.Green {
		t.Fatalf("prove = %d %s, %v", code, output, err)
	}
	log, err := os.ReadFile(result.Data.Log)
	if err != nil || !strings.Contains(string(log), "committed-proof") {
		t.Fatalf("committed command did not run: %q, %v", log, err)
	}
	b.owners.resolver = stateroot.NewResolver(fakeTop(b.root), noExecutable)
	b.owners.work.settings = func(string) (launch.Settings, error) { return launch.Settings{}, nil }
	var stdout, stderr bytes.Buffer
	code = runIntentIn(mustIntentCommand(t, "settings show"), []string{"proof.full"}, &stdout, &stderr, b.install, b.owners)
	if code != 0 || !strings.Contains(stdout.String(), "printf committed-proof") || !strings.Contains(stdout.String(), "conf") || !strings.Contains(stdout.String(), "local value ignored") {
		t.Fatalf("show = %d %s%s", code, &stdout, &stderr)
	}
}

func TestSettingsProofDeclarationsRefuseSeatWritesAndCheckMissingKeys(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	root, owners := bed.root(), bed.workOwners()
	local := filepath.Join(root, "metasystem.conf.local")
	if err := os.WriteFile(local, []byte("proof.full=local\nproof.cheap=local\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(mustIntentCommand(t, "settings check"), nil, &stdout, &stderr, root, owners)
	if code == 0 || !strings.Contains(stdout.String()+stderr.String(), "proof.full is required") || !strings.Contains(stdout.String()+stderr.String(), "proof.cheap is required") {
		t.Fatalf("check = %d %s%s", code, &stdout, &stderr)
	}
	conf := filepath.Join(root, "metasystem.conf")
	before, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"proof.full", "proof.cheap"} {
		for _, value := range []string{"printf proof", "", "first\nsecond", "first\rsecond"} {
			code, result := runSettingsSet(t, root, owners, key, value)
			if code != 2 || result.Outcome != intentRefused || !strings.Contains(result.Summary, key) || !strings.Contains(result.Decision, "declare "+key+" in metasystem.conf through a goal and land it on main") || strings.Contains(result.Decision, "settings set") {
				t.Fatalf("set %s = %d %+v", key, code, result)
			}
		}
	}
	committed, err := os.ReadFile(conf)
	if err != nil || !bytes.Equal(before, committed) {
		t.Fatalf("committed file changed: %q, %v", committed, err)
	}
	after, err := os.ReadFile(local)
	if err != nil || string(after) != "proof.full=local\nproof.cheap=local\n" {
		t.Fatalf("local changed: %q, %v", after, err)
	}
	if code, result := runSettingsSet(t, root, owners, "landing.prove.command", "true"); code == 0 || !strings.Contains(result.Summary, "proof.full") {
		t.Fatalf("retired key: %d %+v", code, result)
	}
}

func TestLandingProveRequiresAValidFullDeclaration(t *testing.T) {
	t.Parallel()
	for _, body := range []string{"", "landing.prove.command=true\n", "proof.full=\n", "proof.full=first\rsecond\n", "proof.full=true\nproof.full=false\n"} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			b := newResolveVerbFixture(t)
			conf := filepath.Join(b.install, "metasystem.conf")
			if err := os.WriteFile(conf, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(conf+".local", []byte("proof.full=true\nlanding.prove.command=true\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			b.owners.landing.plainProve.Git = func(_ string, args ...string) (string, error) {
				switch strings.Join(args, " ") {
				case "rev-parse --verify HEAD^{commit}":
					return "head", nil
				case "show head:metasystem/metasystem.conf":
					return body, nil
				}
				t.Errorf("a missing or invalid proof command reached execution: %v", args)
				return "", errors.New("not allowed")
			}
			code, out := b.run(t, b.root, "prove", "--wait")
			if code != 1 || !strings.Contains(out, "proof.full") || !strings.Contains(out, "declare proof.full in metasystem.conf through a goal and land it on main") || strings.Contains(out, "settings set") {
				t.Fatalf("missing or invalid proof was accepted: %d %s", code, out)
			}
		})
	}
}
