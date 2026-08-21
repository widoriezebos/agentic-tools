package main

// The multi-machine goal verbs (BGS CLI wiring): migrate is the
// cutover entry point, fetch is the read-side advance, and the
// dual-world detection routes the read surface — the legacy verbs
// keep their meaning until the migration commit lands, and the new
// projection owns the reads after.

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"

	"os/exec"
	"path/filepath"
	"strings"
)

// ensureGuardEnrolled installs or composes the pre-commit guard
// before any goal mutation publishes (R2-11): git does not clone
// hooks, so a fresh clone would otherwise mutate the ledger with no
// accidental-edit fence. An existing hook is preserved as
// pre-commit.local behind the guard; a hook already referencing the
// guard is left alone; BOTH files existing without the guard
// refuses toward manual composition — enrollment never clobbers
// (R2-15's rule, held here too). The composer pins the guard's
// absolute path: hooks are per-clone by nature, and the vendored
// layout keeps the guard below the git toplevel.
func ensureGuardEnrolled(root string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	guard := filepath.Join(absRoot, "scripts", "agents", "pre-commit-guard.sh")
	info, statErr := os.Stat(guard)
	if statErr != nil || info.Mode()&0o111 == 0 {
		// Fail closed: a checkout without an
		// EXECUTABLE guard cannot claim the fence exists, and a
		// mutation without the fence is exactly what R2-11 forbids.
		return fmt.Errorf("this checkout ships no executable pre-commit guard at %s; the ledger fence cannot be enrolled, so the mutation refuses", guard)
	}
	// The probes run with git's steering env scrubbed (GIT_DIR and
	// friends): an inherited broken GIT_DIR must neither read as
	// "nothing to enroll" nor answer for some OTHER repository. With
	// the scrub, "not a repository" is a true fact about the root —
	// and a target with no git has nothing that commits, so there is
	// no fence to enroll (adoption installs it when git init lands).
	probe := exec.Command("git", "-C", absRoot, "rev-parse", "--is-inside-work-tree")
	probe.Env = environWithoutGitSteeringCLI()
	if probeErr := probe.Run(); probeErr != nil {
		var exit *exec.ExitError
		if errors.As(probeErr, &exit) && exit.ExitCode() == 128 {
			return nil
		}
		return fmt.Errorf("the target's repository shape cannot be proven: %v", probeErr)
	}
	// --git-path hooks honors core.hooksPath: writing under
	// .git/hooks while git reads a configured hooks path elsewhere
	// would enroll a hook git never invokes. A probe that cannot
	// answer refuses.
	hooksCmd := exec.Command("git", "-C", absRoot, "rev-parse", "--path-format=absolute", "--git-path", "hooks")
	hooksCmd.Env = environWithoutGitSteeringCLI()
	out, err := hooksCmd.Output()
	if err != nil {
		return fmt.Errorf("the hooks directory cannot be resolved for %s: %v", absRoot, err)
	}
	hookDir := strings.TrimSpace(string(out))
	hookPath := filepath.Join(hookDir, "pre-commit")
	existing, readErr := os.ReadFile(hookPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		// An unreadable hook is not an absent hook: falling through
		// would overwrite a file whose content was never seen.
		return fmt.Errorf("the existing pre-commit hook cannot be read: %v", readErr)
	}
	// Enrollment means THIS checkout's guard: after resolving the
	// composer's dynamic toplevel reference, the hook must name the
	// current absolute guard path. A mere mention of the guard's
	// basename — a stale path from before a checkout move, or an
	// inert comment — leaves git running no fence at all.
	enrolled := false
	if readErr == nil {
		resolved := string(existing)
		topCmd := exec.Command("git", "-C", absRoot, "rev-parse", "--show-toplevel")
		topCmd.Env = environWithoutGitSteeringCLI()
		if topOut, topErr := topCmd.Output(); topErr == nil {
			resolved = strings.ReplaceAll(resolved, "$(git rev-parse --show-toplevel)", strings.TrimSpace(string(topOut)))
		}
		enrolled = strings.Contains(resolved, guard)
	}
	if enrolled {
		hookInfo, hookStatErr := os.Stat(hookPath)
		if hookStatErr != nil {
			return fmt.Errorf("the pre-commit hook's mode cannot be proven: %v", hookStatErr)
		}
		if hookInfo.Mode()&0o111 == 0 {
			return fmt.Errorf("the pre-commit hook references the guard but is not executable; fix its mode before mutating the ledger")
		}
		return nil
	}
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		return err
	}
	localPath := filepath.Join(hookDir, "pre-commit.local")
	if readErr == nil && !isOurComposer(string(existing)) {
		if _, localErr := os.Stat(localPath); localErr == nil {
			return fmt.Errorf("pre-commit and pre-commit.local both exist and neither enrolls the guard; compose them by hand before mutating the ledger")
		}
		if err := os.Rename(hookPath, localPath); err != nil {
			return err
		}
	}
	// A hook WE wrote earlier (recognized by its own shape) carries a
	// stale guard path after a checkout move; rewriting our own
	// composer is not a clobber — the local dispatch stays intact.
	composer := "#!/usr/bin/env bash\nguard=" + shellSingleQuote(guard) + "\n" + `if [[ -x "$guard" ]]; then
  "$guard" || exit $?
fi
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
if [[ -x "$here/pre-commit.local" ]]; then
  exec "$here/pre-commit.local" "$@"
fi
exit 0
`
	if err := os.WriteFile(hookPath, []byte(composer), 0o755); err != nil {
		return err
	}
	// WriteFile's mode is umask-filtered; the fence exists only if
	// git can actually execute the hook, so set and re-verify.
	if err := os.Chmod(hookPath, 0o755); err != nil {
		return err
	}
	hookInfo, hookStatErr := os.Stat(hookPath)
	if hookStatErr != nil || hookInfo.Mode()&0o111 == 0 {
		return fmt.Errorf("the enrolled pre-commit hook did not come out executable; fix the filesystem before mutating the ledger")
	}
	return nil
}

// isOurComposer recognizes the enrollment composer this program (and
// adopt.sh) writes: a guard= assignment on its own line plus the
// pre-commit.local dispatch. Only a hook of that shape may be
// rewritten in place; anything else is a human's file.
func isOurComposer(hook string) bool {
	hasAssign := false
	for _, line := range strings.Split(hook, "\n") {
		if strings.HasPrefix(line, "guard=") && strings.Contains(line, "pre-commit-guard.sh") {
			hasAssign = true
			break
		}
	}
	return hasAssign && strings.Contains(hook, `exec "$here/pre-commit.local"`)
}

// shellSingleQuote renders s as one single-quoted shell word: inside
// single quotes the shell expands nothing, so paths carrying $,
// backticks, or spaces survive verbatim. A Go strconv.Quote here
// would hand bash a double-quoted string it expands.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// environWithoutGitSteeringCLI mirrors the goal package's scrub: the
// enrollment probes must answer about the ROOT, never about whatever
// repository an inherited GIT_DIR happens to point at.
func environWithoutGitSteeringCLI() []string {
	steering := map[string]bool{
		"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_COMMON_DIR": true,
		"GIT_INDEX_FILE": true, "GIT_CEILING_DIRECTORIES": true,
		"GIT_OBJECT_DIRECTORY": true, "GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
	}
	var out []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !steering[name] {
			out = append(out, entry)
		}
	}
	return out
}

func goalActor(root string, human string) (goal.Actor, error) {
	machine, err := goal.ResolveMachine(root)
	if err != nil {
		return goal.Actor{}, err
	}
	// METASYSTEM_OWNER_LINEAGE is the runner's real export (the same
	// variable arming and succession read); a second spelling here
	// collapsed every real session to the literal "session" (F16).
	lineage := os.Getenv("METASYSTEM_OWNER_LINEAGE")
	if lineage == "" {
		lineage = "session"
	}
	return goal.Actor{Machine: machine, Lineage: lineage, Human: human}, nil
}

func goalUlid() (string, error) {
	// A ULID-shaped random id: 26 chars, Crockford base32. The
	// timestamp prefix is not load-bearing for uniqueness here — the
	// opid appends machine and lineage — so randomness suffices.
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	raw := make([]byte, 26)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	for i := range raw {
		raw[i] = alphabet[int(raw[i])%len(alphabet)]
	}
	return string(raw), nil
}

// runGoalMigrate is the cutover: one commit, one opid, the reviewed
// source digest gating everything.
func runGoalMigrate(args []string) int {
	flags := flag.NewFlagSet("goal migrate", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	sourceDigest := flags.String("source-digest", "", "the reviewed goals.md sha256 literal")
	manifest := flags.String("manifest", "", "amendment manifest path (omit for a bare migration)")
	identity := flags.String("identity", "", "adoption ULID (minted when omitted)")
	syncMode := flags.String("sync-mode", "remote", "remote or local")
	by := flags.String("by", "", "the human directing the cutover")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *sourceDigest == "" || *by == "" {
		fmt.Fprintln(os.Stderr, "goal migrate: --root, --source-digest, and --by are required — the cutover is a human act on reviewed bytes")
		return 2
	}
	adoption := *identity
	if adoption == "" {
		// A rerun never mints a second identity: the ledger's own
		// standing identity is adopted when one exists (F4 residue).
		if existing := goal.ExistingLedgerIdentity(*root); existing != "" {
			adoption = existing
		} else {
			minted, err := goalUlid()
			if err != nil {
				fmt.Fprintf(os.Stderr, "goal migrate: %v\n", err)
				return 1
			}
			adoption = minted
		}
	}
	if err := ensureGuardEnrolled(*root); err != nil {
		fmt.Fprintf(os.Stderr, "goal migrate: %v\n", err)
		return 1
	}
	actor, err := goalActor(*root, *by)
	if err != nil {
		fmt.Fprintf(os.Stderr, "goal migrate: %v\n", err)
		return 1
	}
	ulid, err := goalUlid()
	if err != nil {
		fmt.Fprintf(os.Stderr, "goal migrate: %v\n", err)
		return 1
	}
	endpoint, err := goal.ResolveEndpoint(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "goal migrate: %v\n", err)
		return 1
	}
	res, err := goal.Migrate(goal.VerbRequest{
		Endpoint: endpoint, Actor: actor, Ulid: ulid, Now: time.Now(),
	}, goal.MigrateOptions{
		SourceDigest: *sourceDigest, ManifestPath: *manifest,
		Identity: adoption, SyncMode: *syncMode,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "goal migrate: %v\n", err)
		return 1
	}
	out, _ := json.MarshalIndent(map[string]any{
		"outcome": res.Outcome, "tip": res.Tip, "identity": adoption, "detail": res.Detail,
	}, "", "  ")
	fmt.Println(string(out))
	if res.Outcome != goal.OutcomeConfirmed {
		return 1
	}
	return 0
}

// runGoalFetch is the read-side advance: validate, then CAS the
// accepted ref — how this machine observes the fleet.
func runGoalFetch(args []string) int {
	flags := flag.NewFlagSet("goal fetch", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" {
		fmt.Fprintln(os.Stderr, "goal fetch: --root is required")
		return 2
	}
	endpoint, err := goal.ResolveEndpoint(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "goal fetch: %v\n", err)
		return 1
	}
	res, err := goal.FetchAdvance(endpoint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "goal fetch: %v\n", err)
		return 1
	}
	fmt.Printf("advanced=%v tip=%s %s\n", res.Advanced, res.Tip, res.Detail)
	return 0
}

// hexDigestOf helps the rehearsal scripts compute the reviewed
// literal without a python detour.
func runGoalSourceDigest(args []string) int {
	flags := flag.NewFlagSet("goal source-digest", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" {
		fmt.Fprintln(os.Stderr, "goal source-digest: --root is required")
		return 2
	}
	data, err := os.ReadFile(*root + "/plans/goals.md")
	if err != nil {
		fmt.Fprintf(os.Stderr, "goal source-digest: %v\n", err)
		return 1
	}
	fmt.Println(goal.SourceDigestOf(data))
	return 0
}

// runGoalRecover executes the one recovery rule over the journal —
// the verb a stranded clone runs to move again.
func runGoalRecover(args []string) int {
	flags := flag.NewFlagSet("goal recover", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" {
		fmt.Fprintln(os.Stderr, "goal recover: --root is required")
		return 2
	}
	if err := ensureGuardEnrolled(*root); err != nil {
		fmt.Fprintf(os.Stderr, "goal recover: %v\n", err)
		return 1
	}
	endpoint, err := goal.ResolveEndpoint(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "goal recover: %v\n", err)
		return 1
	}
	reports, err := goal.Recover(endpoint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "goal recover: %v\n", err)
		return 1
	}
	if len(reports) == 0 {
		fmt.Println("the journal is clean; nothing to recover")
		return 0
	}
	for _, rep := range reports {
		fmt.Printf("%s: %s — %s\n", rep.Opid, rep.Action, rep.Detail)
	}
	return 0
}
