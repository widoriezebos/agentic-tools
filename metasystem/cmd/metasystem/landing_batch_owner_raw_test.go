package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type batchRawConfigFact struct {
	root, key, value string
	err              error
}

type batchRawConfigRequest struct{ root, key string }

// batchRawFacts records all requests because the production resolvers may discard lookup errors.
type batchRawFacts struct {
	want   []batchRawConfigFact
	got    []batchRawConfigRequest
	extra  []string
	gitGot []string
	root   string
}

func batchFileOnlyRoots(t *testing.T) (seat, landing string) {
	t.Helper()
	seat, landing = t.TempDir(), t.TempDir()
	var err error
	landing, err = filepath.EvalSymlinks(landing)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seat, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return seat, landing
}

func batchConfigFacts(root string, present bool, command bool) *batchRawFacts {
	facts := &batchRawFacts{root: root}
	if present {
		facts.want = append(facts.want,
			batchRawConfigFact{root: root, key: "metasystem.goal.machine", value: "mac-cli\n"},
			batchRawConfigFact{root: root, key: "goal.sync-remote", value: "local\n"},
			batchRawConfigFact{root: root, key: "goal.sync-branch", value: "refs/heads/main\n"})
		if command {
			facts.want = append(facts.want, batchRawConfigFact{root: root, key: "metasystem.goal.machine", value: "mac-cli\n"})
		}
	} else {
		facts.want = append(facts.want, batchRawConfigFact{root: root, key: "metasystem.goal.machine", err: errors.New("git config --get metasystem.goal.machine: exit status 1")})
	}
	return facts
}

func (facts *batchRawFacts) config(root, key string) (string, error) {
	facts.got = append(facts.got, batchRawConfigRequest{root: root, key: key})
	index := len(facts.got) - 1
	if index >= len(facts.want) || facts.want[index].root != root || facts.want[index].key != key {
		facts.extra = append(facts.extra, fmt.Sprintf("config[%d] root=%q key=%q", index, root, key))
		return "", fmt.Errorf("unexpected raw config lookup")
	}
	return facts.want[index].value, facts.want[index].err
}

func (facts *batchRawFacts) landingGit(root string, args, environment []string) ([]byte, error) {
	facts.gitGot = append(facts.gitGot, fmt.Sprintf("root=%q args=%q environment=%q", root, args, environment))
	wantArgs := []string{"rev-parse", "--show-toplevel"}
	wantEnv := []string{"PATH=" + os.Getenv("PATH"), "LC_ALL=C"}
	if root != facts.root || !reflect.DeepEqual(args, wantArgs) || !reflect.DeepEqual(environment, wantEnv) {
		facts.extra = append(facts.extra, "unexpected landing Git request: "+facts.gitGot[len(facts.gitGot)-1])
		return nil, fmt.Errorf("unexpected raw landing Git request")
	}
	return []byte(root + "\n"), nil
}

func (facts *batchRawFacts) source() *batchOwnerSource {
	return &batchOwnerSource{commandNow: goalCommandNow, landingGit: facts.landingGit, goalConfig: facts.config}
}

func (facts *batchRawFacts) assertConsumed(t *testing.T, landingGit bool) {
	t.Helper()
	var want []batchRawConfigRequest
	for _, fact := range facts.want {
		want = append(want, batchRawConfigRequest{root: fact.root, key: fact.key})
	}
	wantGit := 0
	if landingGit {
		wantGit = 1
	}
	if len(facts.extra) != 0 || !reflect.DeepEqual(facts.got, want) || len(facts.gitGot) != wantGit {
		t.Fatalf("raw transcript: config got=%v want=%v, Git got=%v want count=%d, unexpected=%v", facts.got, want, facts.gitGot, wantGit, facts.extra)
	}
}
