package main

// The multi-machine goal verbs (BGS CLI wiring): migrate is the
// cutover entry point, fetch is the read-side advance, and the
// dual-world detection routes the read surface — the legacy verbs
// keep their meaning until the migration commit lands, and the new
// projection owns the reads after.

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ledgerfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// ensureGuardEnrolled is the command edge's name for the fence enrollment
// internal/ledgerfence owns, which the interface server's in-process ledger
// publications share. The root it is handed arrives as a flag value and is
// admitted as the installation whose engine the fence runs; a directory
// without metasystem.conf is refused before anything is enrolled.
func ensureGuardEnrolled(root string) error {
	installation, err := stateroot.ParseInstallation(root)
	if err != nil {
		return err
	}
	return ledgerfence.Ensure(installation)
}

// goalActorFromDependencies is the actor from request dependencies: their
// machine reader and the lineage they carry.
func goalActorFromDependencies(dependencies syncRequestDependencies, root, human string) (goal.Actor, error) {
	lineage := ""
	if dependencies.ownerLineage != nil {
		lineage = dependencies.ownerLineage()
	}
	return goalActorWith(dependencies.machine, root, human, lineage)
}

func goalActorWith(resolveMachine func(string) (string, error), root, human, lineage string) (goal.Actor, error) {
	machine, err := resolveMachine(root)
	if err != nil {
		return goal.Actor{}, err
	}
	if lineage == "" {
		lineage = "session"
	}
	return goal.Actor{Machine: machine, Lineage: lineage, Human: human}, nil
}

func goalUlid() (string, error) {
	return goal.NewOperationULID()
}

// goalMigrateWith is the cutover under explicit request dependencies: the
// human word classifies from their supplied caller, the actor carries their
// lineage and machine, and the report goes to the caller's streams.
func goalMigrateWith(dependencies syncRequestDependencies, stdout, stderr io.Writer, args []string) int {
	flags := newFlagSet("goal migrate", stdout, stderr)
	root := pathFlag(flags, "root", "", "checkout root")
	sourceDigest := flags.String("source-digest", "", "the reviewed goals.md sha256 literal")
	manifest := flags.String("manifest", "", "amendment manifest path (omit for a bare migration)")
	identity := flags.String("identity", "", "adoption ULID (minted when omitted)")
	syncMode := flags.String("sync-mode", "remote", "remote or local")
	by := flags.String("by", "", "the human directing the cutover")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *sourceDigest == "" || *by == "" {
		fmt.Fprintln(stderr, "goal migrate needs --root, --source-digest and --by: a person upgrades reviewed files")
		return 2
	}
	classification, classErr := brainHumanWordClassificationWithFacts("migrate", *root, *by, nil, dependencies.authorityFacts)
	if classErr != nil {
		fmt.Fprintf(stderr, "goal migrate: %v\n", classErr)
		return 1
	}
	adoption := *identity
	if adoption == "" {
		// A rerun never mints a second identity: the ledger's own
		// standing identity is adopted when one exists.
		if existing := goal.ExistingLedgerIdentity(*root); existing != "" {
			adoption = existing
		} else {
			minted, err := goalUlid()
			if err != nil {
				fmt.Fprintf(stderr, "goal migrate: %v\n", err)
				return 1
			}
			adoption = minted
		}
	}
	if err := dependencies.ensureGuard(*root); err != nil {
		fmt.Fprintf(stderr, "goal migrate: %v\n", err)
		return 1
	}
	actor, err := goalActorFromDependencies(dependencies, *root, *by)
	if err != nil {
		fmt.Fprintf(stderr, "goal migrate: %v\n", err)
		return 1
	}
	ulid, err := goalUlid()
	if err != nil {
		fmt.Fprintf(stderr, "goal migrate: %v\n", err)
		return 1
	}
	endpoint, err := dependencies.endpoint(*root)
	if err != nil {
		fmt.Fprintf(stderr, "goal migrate: %v\n", err)
		return 1
	}
	configureCarriedCounselor(&endpoint)
	endpoint.ClaimHolder = dependencies.claimHolder.Facts
	res, err := goal.Migrate(goal.VerbRequest{
		Endpoint: endpoint, Actor: actor, Ulid: ulid, Now: time.Now(), CallerClass: classification.Class,
	}, goal.MigrateOptions{
		SourceDigest: *sourceDigest, ManifestPath: *manifest,
		Identity: adoption, SyncMode: *syncMode,
	})
	if err != nil {
		fmt.Fprintf(stderr, "goal migrate: %v\n", err)
		return 1
	}
	out, _ := json.MarshalIndent(map[string]any{
		"outcome": res.Outcome, "tip": res.Tip, "identity": adoption, "detail": res.Detail,
	}, "", "  ")
	fmt.Fprintln(stdout, string(out))
	if res.Outcome != goal.OutcomeConfirmed {
		return 1
	}
	return 0
}

// runGoalFetch is the read-side advance: validate, then CAS the
// accepted ref — how this machine observes the fleet.
func runGoalFetch(args []string, stdout, stderr io.Writer) int {
	return runGoalFetchWithResolver(args, goal.ResolveEndpoint, stdout, stderr)
}

func runGoalFetchWithResolver(args []string, resolve func(string) (goal.Endpoint, error), stdout, stderr io.Writer) int {
	flags := newFlagSet("goal fetch", stdout, stderr)
	root := pathFlag(flags, "root", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	return goalFetchTo(stdout, stderr, *root, resolve)
}

// goalFetchTo is the read-side advance on the caller's streams.
func goalFetchTo(stdout, stderr io.Writer, root string, resolve func(string) (goal.Endpoint, error)) int {
	if root == "" {
		fmt.Fprintln(stderr, "goal fetch: --root is required")
		return 2
	}
	endpoint, err := resolve(root)
	if err != nil {
		fmt.Fprintf(stderr, "goal fetch: %v\n", err)
		return 1
	}
	res, err := goal.FetchAdvance(endpoint)
	if err != nil {
		fmt.Fprintf(stderr, "goal fetch: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "advanced=%v tip=%s %s\n", res.Advanced, res.Tip, res.Detail)
	return 0
}

// goalRepairAcceptRemoteTo is the accept-remote repair owner on the caller's
// streams; facts carry the supplied caller identity the brain's human-word
// gate classifies.
func goalRepairAcceptRemoteTo(stdout, stderr io.Writer, root, by string, facts goalAuthorityReadFacts, resolveEndpoint func(string) (goal.Endpoint, error)) int {
	if root == "" {
		fmt.Fprintln(stderr, "goal repair: --root is required")
		return 2
	}
	if _, classErr := brainHumanWordClassificationWithFacts("repair", root, by, nil, facts); classErr != nil {
		fmt.Fprintf(stderr, "goal repair: %v\n", classErr)
		return 1
	}
	endpoint, err := resolveEndpoint(root)
	if err != nil {
		fmt.Fprintf(stderr, "goal repair: %v\n", err)
		return 1
	}
	res, err := goal.RepairAcceptRemote(endpoint, by)
	if err != nil {
		fmt.Fprintf(stderr, "goal repair: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "advanced=%v tip=%s %s\n", res.Advanced, res.Tip, res.Detail)
	return 0
}

// recoverGoalJournal runs the one recovery rule over the journal and returns
// what it did to each stranded entry. The fence is enrolled for the
// installation, whose engine the commit hook runs; the journal is the state
// root's.
func recoverGoalJournal(installation stateroot.Installation, root string, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) ([]goal.RecoveryReport, error) {
	if err := dependencies.ensureGuard(installation.Path()); err != nil {
		return nil, err
	}
	endpoint, err := dependencies.endpoint(root)
	if err != nil {
		return nil, err
	}
	configureCarriedCounselor(&endpoint)
	now, err := commandNow(root)
	if err != nil {
		return nil, err
	}
	return goal.RecoverWithPolicy(endpoint, goalRecoveryPolicy{GoalRecoveryPolicy: dispatchcore.GoalRecoveryPolicy{Now: now}, root: root})
}
