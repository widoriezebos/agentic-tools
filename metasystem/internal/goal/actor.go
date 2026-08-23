package goal

// ResolveActor names this machine's executing identity for the
// multi-machine verbs: the machine from git config
// metasystem.goal.machine (set once at enrollment; the sanitized
// hostname is the fallback), the lineage from the session context
// the caller carries. The identity must be stable across sessions —
// claims and quotas key on it.

import (
	"fmt"
	"strings"
)

// ResolveMachine reads the durable machine nickname. There is no
// hostname fallback on purpose: everything this name reaches — ledger
// claims, publication history, landing trailers — is pushed to shared
// remotes, and a hostname is what a machine IS, not what it is called.
// Enrollment is one local command, and refusing until it happens is
// the privacy holding its ground.
func ResolveMachine(root string) (string, error) {
	if out, err := gitIn(root, "config", "--get", "metasystem.goal.machine"); err == nil {
		if name := strings.TrimSpace(out); name != "" {
			return name, nil
		}
	}
	return "", fmt.Errorf("no machine nickname is enrolled and hostnames are never published: run  git config metasystem.goal.machine <nickname>  once on this machine")
}

// NewWorld reports whether this repository has migrated: the
// accepted ref exists and its tree carries the root record. The
// dual-world CLI routes on this — legacy verbs keep their meaning
// until the migration commit, and the new surface owns everything
// after.
// ExistingLedgerIdentity returns the migrated ledger's adoption
// identity, or "" before migration. The CLI's rerun path adopts it
// instead of minting a second identity: the identity
// is minted ONCE, and a rerun that invents another is a confusion
// the engine would refuse anyway — this keeps the rerun honest and
// idempotent without the caller re-supplying it.
func ExistingLedgerIdentity(root string) string {
	out, err := gitIn(root, "rev-parse", "--verify", "--quiet", AcceptedRef)
	if err != nil {
		return ""
	}
	content, err := gitIn(root, "cat-file", "-p", strings.TrimSpace(out)+":./"+goalsPrefix+"backlog.md")
	if err != nil {
		return ""
	}
	record, _ := ParseRoot([]byte(content))
	if record == nil {
		return ""
	}
	return record.Identity
}

func NewWorld(root string) bool {
	out, err := gitIn(root, "rev-parse", "--verify", "--quiet", AcceptedRef)
	if err != nil {
		return false
	}
	_, err = gitIn(root, "cat-file", "-e", strings.TrimSpace(out)+":./"+goalsPrefix+"backlog.md")
	return err == nil
}
