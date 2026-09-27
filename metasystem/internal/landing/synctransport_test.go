package landing

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

type transportCall struct {
	dir  string
	args []string
}

func stubTransportGit(fetchErr, pushErr error, pushOutput string, calls *[]transportCall) TransportGit {
	return func(dir string, args ...string) (string, error) {
		*calls = append(*calls, transportCall{dir: dir, args: append([]string(nil), args...)})
		switch args[0] {
		case "fetch":
			return "", fetchErr
		case "push":
			return pushOutput, pushErr
		}
		return "", errors.New("unexpected git command")
	}
}

func TestSyncTransportMirrorsOriginsFullBranchRef(t *testing.T) {
	t.Parallel()
	var calls []transportCall
	last, err := SyncTransport("/checkout", "main", stubTransportGit(nil, nil, "To transport\n   a..b  origin/main -> main\n", &calls))
	if err != nil {
		t.Fatal(err)
	}
	want := []transportCall{
		{dir: "/checkout", args: []string{"fetch", "--quiet", "origin", "+refs/heads/main:refs/remotes/origin/main"}},
		{dir: "/checkout", args: []string{"push", "transport", "refs/remotes/origin/main:refs/heads/main"}},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("git calls = %#v, want %#v", calls, want)
	}
	if last != "   a..b  origin/main -> main" {
		t.Fatalf("last push line = %q", last)
	}
}

func TestSyncTransportRefusesUnlawfulBranchesBeforeGit(t *testing.T) {
	t.Parallel()
	for _, branch := range []string{"", "-main", "a..b", "a//b", "a b", "main\nevil", "a\tb", ".hidden", "main;rm"} {
		var calls []transportCall
		_, err := SyncTransport("/checkout", branch, stubTransportGit(nil, nil, "", &calls))
		var refusal *TransportError
		if !errors.As(err, &refusal) || refusal.Code != 2 || !strings.Contains(refusal.Detail, "is not a plain branch") {
			t.Fatalf("branch %q: err = %v", branch, err)
		}
		if len(calls) != 0 {
			t.Fatalf("branch %q reached git: %#v", branch, calls)
		}
	}
}

func TestSyncTransportNamesAMissingOriginBranchAndAFailedPush(t *testing.T) {
	t.Parallel()
	var calls []transportCall
	_, err := SyncTransport("/checkout", "release/1", stubTransportGit(errors.New("exit 128"), nil, "", &calls))
	var refusal *TransportError
	if !errors.As(err, &refusal) || refusal.Code != 1 || refusal.Detail != "sync-transport refused: origin has no branch 'release/1'" {
		t.Fatalf("missing origin branch: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("a failed fetch must not push: %#v", calls)
	}
	calls = nil
	_, err = SyncTransport("/checkout", "main", stubTransportGit(nil, errors.New("exit 1"), "error: failed to push some refs\n", &calls))
	if !errors.As(err, &refusal) || refusal.Code != 1 || !strings.Contains(refusal.Detail, "failed to push some refs") {
		t.Fatalf("failed push: %v", err)
	}
}
