package lane

// The lane checkout's pre-push hook (design r10 §1 and K3). landing set
// installs it in the lane checkout only, never in a seat; landing unset
// removes it. It runs the lane installation's engine (internal pre-push),
// which admits a push from the lane checkout only when it is exactly the
// one ref update a publication token was minted for.

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// The hook's refusal codes at landing set.
const (
	// CodeHookOccupied is a lane checkout that already has a pre-push hook
	// of its own: it is never overwritten.
	CodeHookOccupied = "LANDING_LANE_HOOK_OCCUPIED"
	// CodeHookShared is a lane checkout whose hooks live outside its own
	// git directory (core.hooksPath), where a seat could share them.
	CodeHookShared = "LANDING_LANE_HOOK_SHARED"
)

// hookMarker is the line that makes a pre-push hook the lane's own.
const hookMarker = "# the landing lane's pre-push hook, installed by landing set"

// HookScript is the pre-push hook of the lane whose installation's engine
// is engine, under home.
func HookScript(engine, home string) string {
	return "#!/bin/sh\n" + hookMarker + ": only a landing publication pushes from this checkout.\n" +
		"exec " + shellQuote(engine) + " internal pre-push --home " + shellQuote(home) + " \"$@\"\n"
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'" }

// hookPath is the pre-push hook git runs in checkout, refused when the
// hooks directory is not inside the checkout's own git directory.
func hookPath(checkout string) (string, error) {
	hooks, err := laneGit(checkout, nil, "rev-parse", "--path-format=absolute", "--git-path", "hooks")
	if err != nil {
		return "", fmt.Errorf("the hooks folder of %s can't be read: %w", checkout, err)
	}
	common, err := laneGit(checkout, nil, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("the git folder of %s can't be read: %w", checkout, err)
	}
	if rel, err := filepath.Rel(resolved(common), resolved(hooks)); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", &Refusal{Code: CodeHookShared,
			Message: fmt.Sprintf("the landing checkout %s takes its git hooks from %s, outside its own git folder, where other checkouts may share them; nothing was registered", checkout, hooks),
			Fix:     "remove the checkout's core.hooksPath (git -C " + checkout + " config --unset core.hooksPath), then run metasystem landing set again",
			Argv:    []string{"git", "-C", checkout, "config", "--unset", "core.hooksPath"}}
	}
	return filepath.Join(hooks, "pre-push"), nil
}

// installHook writes the lane's pre-push hook into layout's checkout, once.
// A pre-push hook that is not the lane's is never overwritten.
func installHook(home string, layout Layout) error {
	path, err := hookPath(string(layout.Checkout))
	if err != nil {
		return err
	}
	script := HookScript(filepath.Join(string(layout.Install), "bin", "metasystem"), home)
	existing, err := os.ReadFile(path)
	switch {
	case err == nil && string(existing) == script:
		return nil
	case err == nil && !bytes.Contains(existing, []byte(hookMarker)):
		return &Refusal{Code: CodeHookOccupied,
			Message: fmt.Sprintf("the landing checkout already has a pre-push hook of its own (%s), and the lane's would replace it; nothing was registered", path),
			Fix:     "move that hook out of the landing checkout (no seat works there), then run metasystem landing set again"}
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary := path + ".metasystem-new"
	if err := os.WriteFile(temporary, []byte(script), 0o755); err != nil {
		return err
	}
	if err := os.Chmod(temporary, 0o755); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

// removeHook removes the lane's pre-push hook from checkout; a hook that is
// not the lane's, or a checkout that is gone, is left as it is.
func removeHook(checkout string) error {
	if checkout == "" || gone(checkout) {
		return nil
	}
	path, err := hookPath(checkout)
	if err != nil {
		var refusal *Refusal
		if errors.As(err, &refusal) {
			return nil
		}
		return err
	}
	existing, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(existing, []byte(hookMarker)) {
		return nil
	}
	return removeIfPresent(path)
}

// gitDir is the git directory dir's repository shares across its
// worktrees, resolved.
func gitDir(dir string) (string, error) {
	common, err := laneGit(dir, nil, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("the git folder of %s can't be read: %w", dir, err)
	}
	return resolved(common), nil
}

// PushUpdate is one line git hands a pre-push hook on its standard input.
type PushUpdate struct {
	LocalRef, LocalOID, RemoteRef, RemoteOID string
}

// ParsePushUpdates reads a pre-push hook's standard input.
func ParsePushUpdates(input io.Reader) ([]PushUpdate, error) {
	var updates []PushUpdate
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 4 {
			return nil, fmt.Errorf("git handed the pre-push hook a line it does not recognize: %q", scanner.Text())
		}
		updates = append(updates, PushUpdate{LocalRef: fields[0], LocalOID: fields[1], RemoteRef: fields[2], RemoteOID: fields[3]})
	}
	return updates, scanner.Err()
}

// Push is one push the hook judges: the git directory of the repository it
// leaves from (shared by all its worktrees), the remote it goes to, the
// nonce its environment carries and its ref updates.
type Push struct {
	GitDir    string
	RemoteURL string
	Nonce     string
	Updates   []PushUpdate
}

// AdmitPush is the hook's decision (K3), under the lane flock. A push from
// a checkout that is not the registered lane's (a seat's), or on a computer
// with no lane, is not the lane's to judge and is admitted. A push from the
// lane checkout is admitted only when its nonce names a minted token, the
// lane still admits the token's operation (K2, read again here), and the
// push is exactly the token's tuple: one update of refs/heads/main from the
// expected old commit to the exact new commit, to the tuple's remote. An
// admitted token is spent. Anything unreadable refuses.
func AdmitPush(home string, push Push) error {
	return withLock(home, func() error {
		record, ok, err := Read(home)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		laneDir, err := gitDir(record.Root)
		if err != nil {
			return err
		}
		if laneDir != resolved(push.GitDir) {
			return nil
		}
		notAdmitted := func(why string) error {
			return &Refusal{Code: CodePushNotAdmitted,
				Message: "only a landing publication pushes from the landing checkout " + record.Root + ", and " + why + "; nothing was pushed",
				Fix:     "the landing lane publishes its work itself: metasystem landing status shows it",
				Argv:    []string{"metasystem", "landing", "status"}}
		}
		minted, found, err := readToken(home, push.Nonce)
		if err != nil {
			return err
		}
		if !found {
			return notAdmitted("this push carries no publication token")
		}
		cleanup := minted.Authority == AuthorityPerson && minted.Operation == OpReturn
		if journal, fenced, _ := ReadUnset(home); fenced && !cleanup {
			return unsettingRefusal(journal)
		}
		if pause, paused := ReadPause(home); paused && !cleanup {
			return pausedRefusal(pause, OpPublish)
		}
		tuple := minted.Tuple
		switch {
		case resolved(tuple.Repo) != resolved(record.Root):
			return notAdmitted("its token was minted for " + tuple.Repo)
		case push.RemoteURL != tuple.RemoteURL:
			return notAdmitted("it goes to " + push.RemoteURL + ", not " + tuple.RemoteURL)
		case len(push.Updates) != 1:
			return notAdmitted(fmt.Sprintf("it updates %d refs, not the one its token names", len(push.Updates)))
		}
		update := push.Updates[0]
		switch {
		case update.RemoteRef != tuple.Ref:
			return notAdmitted("it updates " + update.RemoteRef + ", not " + tuple.Ref)
		case update.LocalOID != tuple.New:
			return notAdmitted("it pushes " + short(update.LocalOID) + ", not the published " + short(tuple.New))
		case update.RemoteOID != tuple.Old:
			return notAdmitted("main is " + short(update.RemoteOID) + ", not the expected " + short(tuple.Old))
		}
		return removeIfPresent(tokenPath(home, push.Nonce))
	})
}

// RunPrePush is the decision of the internal pre-push entry the lane's hook
// runs, for the push git makes from the current directory to remoteURL,
// with its updates on input. It prints a refusal as two lines and returns
// git's exit status.
func RunPrePush(home, remoteURL string, input io.Reader, stderr io.Writer) int {
	// Git runs the hook at the top of the worktree that pushes.
	here, err := os.Getwd()
	pushing := ""
	if err == nil {
		pushing, err = gitDir(here)
	}
	updates, parseErr := ParsePushUpdates(input)
	if err = errors.Join(err, parseErr); err == nil {
		err = AdmitPush(home, Push{GitDir: pushing, RemoteURL: remoteURL, Nonce: os.Getenv(TokenEnv), Updates: updates})
	}
	if err == nil {
		return 0
	}
	var refusal *Refusal
	if errors.As(err, &refusal) {
		fmt.Fprintln(stderr, refusal.Message)
		if len(refusal.Argv) > 0 {
			fmt.Fprintln(stderr, "run: "+strings.Join(refusal.Argv, " "))
		}
		return 1
	}
	fmt.Fprintln(stderr, "the landing lane's pre-push hook could not decide ("+err.Error()+"), so nothing was pushed")
	fmt.Fprintln(stderr, "run: metasystem landing status")
	return 1
}
