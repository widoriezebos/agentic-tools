package lane

// The lane's one publication boundary (design r10 §2, K3): every
// lane-scoped write to main, a batch landing or one of the lane's own goal
// ledger transactions, is published by Publish and nothing else. Publish
// mints a token bound to the exact tuple under the lane flock (the gate,
// K2), pushes with an explicit lease on the expected old commit, and the
// pre-push hook installed in the lane checkout admits exactly that one ref
// update against the token (hook.go). A moved base is never republished:
// it comes back as LANE_BASE_MOVED, and the agent recomposes.

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// The publication kinds a tuple names.
const (
	KindLanding = "landing"
	KindLedger  = "ledger"
)

// MainRef is the only ref a lane publication updates.
const MainRef = "refs/heads/main"

// The publication boundary's refusal codes (the refusal register names
// their sites).
const (
	// CodeBaseMoved is a publication whose expected old commit is no longer
	// main: nothing was published, and the lane recomposes on the new main.
	CodeBaseMoved = "LANE_BASE_MOVED"
	// CodePublishRefused is a publication main did not take for a reason
	// other than a moved base (the tuple is malformed, the hook refused).
	CodePublishRefused = "LANE_PUBLISH_REFUSED"
	// CodePublishUnknown is a push whose outcome could not be read back.
	CodePublishUnknown = "LANE_PUBLISH_UNKNOWN"
	// CodePushNotAdmitted is the hook's refusal of a push from the lane
	// checkout that no publication token admits.
	CodePushNotAdmitted = "LANE_PUSH_NOT_ADMITTED"
)

// TokenEnv carries a publication's token to the lane checkout's pre-push
// hook, through git push's environment.
const TokenEnv = "METASYSTEM_LANE_PUBLISH_TOKEN"

// Tuple is one lane publication, exactly (K3): the repository and remote it
// pushes to, the one ref it updates, the expected old commit (the lease),
// the exact new commit and its tree, what kind of write it is, the batch or
// ledger operation it belongs to, the proof attempt a landing publishes, and
// the lane engine's generation.
type Tuple struct {
	Repo         string `json:"repo"`
	RemoteURL    string `json:"remoteUrl"`
	Ref          string `json:"ref"`
	Old          string `json:"old"`
	New          string `json:"new"`
	Tree         string `json:"tree"`
	Kind         string `json:"kind"`
	Op           string `json:"op"`
	ProofAttempt string `json:"proofAttempt,omitempty"`
	Generation   int    `json:"generation"`
}

var objectID = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

func (t Tuple) problem() string {
	switch {
	case t.Repo == "" || !filepath.IsAbs(t.Repo):
		return "it names no lane checkout"
	case t.RemoteURL == "":
		return "it names no remote"
	case t.Ref != MainRef:
		return fmt.Sprintf("it updates %q, and a lane publication updates only %s", t.Ref, MainRef)
	case !objectID.MatchString(t.Old) || !objectID.MatchString(t.New) || !objectID.MatchString(t.Tree):
		return "its old commit, new commit or tree is not a full object id"
	case t.Kind != KindLanding && t.Kind != KindLedger:
		return fmt.Sprintf("its kind %q is neither %s nor %s", t.Kind, KindLanding, KindLedger)
	case t.Op == "":
		return "it names no batch or operation"
	case t.Kind == KindLanding && t.ProofAttempt == "":
		return "a landing names the test attempt it publishes"
	case t.Generation < 1:
		return "it names no lane engine number"
	}
	return ""
}

// PublishError is a publication main did not take, typed for the caller:
// Code is one of the boundary's codes, Current the main the push found when
// the base moved. It stops a goal transaction's retry loop (the goal
// package reads StopsRetry): the lane never rebuilds on a moved base by
// itself.
type PublishError struct {
	Code    string
	Message string
	// Expected is the tuple's old commit; Current is main as read after the
	// refusal (empty when it could not be read).
	Expected, Current string
	Detail            string
}

func (e *PublishError) Error() string {
	text := e.Code + ": " + e.Message
	if e.Detail != "" {
		text += " (" + e.Detail + ")"
	}
	return text
}

// RefusalCode is the error's code, for goal.RefusalCode and records.
func (e *PublishError) RefusalCode() string { return e.Code }

// StopsRetry tells a goal transaction not to rebuild and push again: the
// boundary's answer is final for this attempt.
func (e *PublishError) StopsRetry() bool { return true }

// IsBaseMoved reports whether err is a publication refused because main
// moved past the expected old commit.
func IsBaseMoved(err error) bool {
	var refused *PublishError
	return errors.As(err, &refused) && refused.Code == CodeBaseMoved
}

// token is the minted admission of one tuple, kept in host state (outside
// every checkout) for the hook to find by its nonce.
type token struct {
	Tuple     Tuple     `json:"tuple"`
	Operation Operation `json:"operation"`
	Authority Authority `json:"authority"`
	Pid       int       `json:"pid"`
}

var nonceShape = regexp.MustCompile(`^[0-9a-f]{64}$`)

func tokenDir(home string) string { return filepath.Join(HostDir(home), "landing-publish-tokens") }

func tokenPath(home, nonce string) string { return filepath.Join(tokenDir(home), nonce+".json") }

// mint writes the token of tuple and returns its nonce; the caller holds the
// lane flock (it runs inside the gate).
func mint(home string, tuple Tuple, op Operation, authority Authority) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	nonce := hex.EncodeToString(raw)
	if err := os.MkdirAll(tokenDir(home), 0o700); err != nil {
		return "", err
	}
	// A token outlives no process: the tokens of ended processes go.
	if entries, err := os.ReadDir(tokenDir(home)); err == nil {
		for _, entry := range entries {
			var stale token
			path := filepath.Join(tokenDir(home), entry.Name())
			if _, err := readJSON(path, &stale); err != nil || !processAlive(stale.Pid) {
				_ = removeIfPresent(path)
			}
		}
	}
	return nonce, writeJSON(home, tokenPath(home, nonce), token{Tuple: tuple, Operation: op, Authority: authority, Pid: os.Getpid()})
}

// Publish publishes tuple as op for authority (K3). It checks the tuple
// against the repository, passes the gate (K2: the lane is registered, is
// this checkout, is not paused or being unset) and mints the tuple's token
// there, pushes with the lease on tuple.Old, and reads main back. Main
// that moved past tuple.Old is a *PublishError with CodeBaseMoved and the
// main it found; it is never republished.
func Publish(home string, tuple Tuple, op Operation, authority Authority) error {
	if problem := tuple.problem(); problem != "" {
		return &PublishError{Code: CodePublishRefused, Expected: tuple.Old,
			Message: "the publication was not well formed (" + problem + "), so nothing was published"}
	}
	tree, err := laneGit(tuple.Repo, nil, "rev-parse", "--verify", "--quiet", tuple.New+"^{tree}")
	if err != nil || tree != tuple.Tree {
		return &PublishError{Code: CodePublishRefused, Expected: tuple.Old,
			Message: "the commit to publish is not the tree the publication names, so nothing was published",
			Detail:  fmt.Sprintf("commit %s has tree %q, the publication names %s: %v", tuple.New, tree, tuple.Tree, err)}
	}
	var nonce string
	err = Gate(home, op, authority, func(record Record) error {
		if resolved(record.Root) != resolved(tuple.Repo) {
			return &PublishError{Code: CodePublishRefused, Expected: tuple.Old,
				Message: "only the landing checkout " + record.Root + " publishes for the lane, so nothing was published",
				Detail:  "the publication names " + tuple.Repo}
		}
		minted, err := mint(home, tuple, op, authority)
		nonce = minted
		if err != nil {
			return err
		}
		// The publication is recorded before it is pushed: the lane watch
		// (K10) finds every lane-trailed commit on main in one of these.
		return recordPublication(home, tuple, time.Now())
	})
	if err != nil {
		return err
	}
	defer removeIfPresent(tokenPath(home, nonce))
	output, pushErr := laneGit(tuple.Repo, []string{TokenEnv + "=" + nonce},
		"push", "--porcelain", "--force-with-lease="+tuple.Ref+":"+tuple.Old, tuple.RemoteURL, tuple.New+":"+tuple.Ref)
	current, readErr := remoteMain(tuple)
	switch {
	case readErr != nil:
		return &PublishError{Code: CodePublishUnknown, Expected: tuple.Old,
			Message: "whether main took the publication can't be read, so it is not known to have landed",
			Detail:  fmt.Sprintf("push: %v %s; reading main back: %v", pushErr, output, readErr)}
	case current == tuple.New:
		return nil
	case current != tuple.Old:
		return &PublishError{Code: CodeBaseMoved, Expected: tuple.Old, Current: current,
			Message: fmt.Sprintf("main moved to %s since the series was composed on %s, so nothing was published", short(current), short(tuple.Old))}
	case pushErr == nil:
		return &PublishError{Code: CodePublishUnknown, Expected: tuple.Old, Current: current,
			Message: "the push reported success but main is unchanged, so nothing is known to have landed", Detail: output}
	}
	return &PublishError{Code: CodePublishRefused, Expected: tuple.Old, Current: current,
		Message: "main refused the publication, so nothing was published", Detail: strings.TrimSpace(output)}
}

func remoteMain(tuple Tuple) (string, error) { return RemoteMain(tuple.Repo, tuple.RemoteURL) }

// RemoteMain reads main at remoteURL from repo; empty when it has none.
func RemoteMain(repo, remoteURL string) (string, error) {
	out, err := laneGit(repo, nil, "ls-remote", "--refs", remoteURL, MainRef)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], nil
}

func short(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// gitSteering is the environment that would point git at another
// repository, configuration or object store than the one named: a lane
// publication never inherits it.
func gitSteering(name string) bool {
	switch name {
	case "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE", "GIT_CEILING_DIRECTORIES",
		"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_CONFIG", "GIT_CONFIG_PARAMETERS",
		"GIT_CONFIG_COUNT", "GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM", "GIT_CONFIG_NOSYSTEM",
		"GIT_GRAFT_FILE", "GIT_SHALLOW_FILE", "GIT_REPLACE_REF_BASE", TokenEnv:
		return true
	}
	return strings.HasPrefix(name, "GIT_CONFIG_KEY_") || strings.HasPrefix(name, "GIT_CONFIG_VALUE_")
}

// laneGit runs git in dir with the steering environment removed and extra
// added, and returns its trimmed standard output (standard error with it
// when it fails).
func laneGit(dir string, extra []string, args ...string) (string, error) {
	return laneGitInput(dir, extra, "", args...)
}

// laneGitInput is laneGit with input on git's standard input.
func laneGitInput(dir string, extra []string, input string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Stdin = strings.NewReader(input)
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !gitSteering(name) {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "LC_ALL=C")
	command.Env = append(command.Env, extra...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return strings.TrimSpace(stdout.String() + "\n" + stderr.String()), fmt.Errorf("git %s: %w", args[0], err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// readToken reads the token of nonce; false when there is none.
func readToken(home, nonce string) (token, bool, error) {
	var minted token
	if !nonceShape.MatchString(nonce) {
		return token{}, false, nil
	}
	ok, err := readJSON(tokenPath(home, nonce), &minted)
	if errors.Is(err, fs.ErrNotExist) {
		return token{}, false, nil
	}
	return minted, ok, err
}

// Publication is one lane publication as the boundary recorded it before
// its push: what the lane watch matches lane-trailed commits on main
// against.
type Publication struct {
	At           string `json:"at"`
	Kind         string `json:"kind"`
	Op           string `json:"op"`
	ProofAttempt string `json:"proofAttempt,omitempty"`
	Old          string `json:"old"`
	New          string `json:"new"`
}

type publications struct {
	Schema       int           `json:"schema"`
	Publications []Publication `json:"publications"`
}

// publicationsKept bounds the record: the newest publications are kept.
const publicationsKept = 500

func publicationsPath(home string) string {
	return filepath.Join(HostDir(home), "landing-publications.json")
}

// recordPublication appends tuple to the publication record; the caller
// holds the lane flock.
func recordPublication(home string, tuple Tuple, now time.Time) error {
	var record publications
	if _, err := readJSON(publicationsPath(home), &record); err != nil {
		return err
	}
	record.Schema = 1
	record.Publications = append(record.Publications, Publication{At: now.UTC().Format(time.RFC3339), Kind: tuple.Kind, Op: tuple.Op,
		ProofAttempt: tuple.ProofAttempt, Old: tuple.Old, New: tuple.New})
	if len(record.Publications) > publicationsKept {
		record.Publications = record.Publications[len(record.Publications)-publicationsKept:]
	}
	return writeJSON(home, publicationsPath(home), record)
}

// ReadPublications is the publication record, oldest first.
func ReadPublications(home string) ([]Publication, error) {
	var record publications
	if _, err := readJSON(publicationsPath(home), &record); err != nil {
		return nil, err
	}
	return record.Publications, nil
}

// publishInFlight says whether a lane publication's push runs now: a token
// whose minting process still lives. The caller holds the lane flock.
func publishInFlight(home string) bool {
	entries, err := os.ReadDir(tokenDir(home))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		var minted token
		if _, err := readJSON(filepath.Join(tokenDir(home), entry.Name()), &minted); err == nil && processAlive(minted.Pid) {
			return true
		}
	}
	return false
}
