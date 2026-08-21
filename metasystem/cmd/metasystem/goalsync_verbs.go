package main

// The multi-machine goal verbs (BGS CLI wiring): migrate is the
// cutover entry point, fetch is the read-side advance, and the
// dual-world detection routes the read surface — the legacy verbs
// keep their meaning until the migration commit lands, and the new
// projection owns the reads after.

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"

	"os/exec"
	"path/filepath"
	"strconv"
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
	// --git-path hooks honors core.hooksPath:
	// writing under .git/hooks while git reads a configured hooks
	// path elsewhere would enroll a hook git never invokes.
	out, err := exec.Command("git", "-C", absRoot, "rev-parse", "--path-format=absolute", "--git-path", "hooks").Output()
	if err != nil {
		return nil // not a git repository; the verbs refuse on their own terms
	}
	hookDir := strings.TrimSpace(string(out))
	hookPath := filepath.Join(hookDir, "pre-commit")
	existing, readErr := os.ReadFile(hookPath)
	if readErr == nil && strings.Contains(string(existing), "pre-commit-guard.sh") {
		if hookInfo, hookStatErr := os.Stat(hookPath); hookStatErr == nil && hookInfo.Mode()&0o111 != 0 {
			return nil
		}
		return fmt.Errorf("the pre-commit hook references the guard but is not executable; fix its mode before mutating the ledger")
	}
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		return err
	}
	localPath := filepath.Join(hookDir, "pre-commit.local")
	if readErr == nil {
		if _, localErr := os.Stat(localPath); localErr == nil {
			return fmt.Errorf("pre-commit and pre-commit.local both exist and neither enrolls the guard; compose them by hand before mutating the ledger")
		}
		if err := os.Rename(hookPath, localPath); err != nil {
			return err
		}
	}
	composer := "#!/usr/bin/env bash\nguard=" + strconv.Quote(guard) + "\n" + `if [[ -x "$guard" ]]; then
  "$guard" || exit $?
fi
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
if [[ -x "$here/pre-commit.local" ]]; then
  exec "$here/pre-commit.local" "$@"
fi
exit 0
`
	return os.WriteFile(hookPath, []byte(composer), 0o755)
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
