package goal

// The read-side advance (BGS-7, D118): the accepted ref moves only
// onto a fetched tip whose TREE validates whole, whose ledger
// identity matches, and which DESCENDS from the accepted tip. On
// any refusal the projection stays at the accepted tree and the
// refusal names the file and the rule — a torn, foreign, or rewound
// tip can never become the world this machine acts on. The
// validator every machine runs is the enforcement point commit-tree
// cannot dodge.

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// AdvanceResult reports one read-side pass.
type AdvanceResult struct {
	Tip      string // the accepted tip after the pass
	Advanced bool   // the pass moved the ref
	Detail   string
}

// FetchAdvance fetches the canonical branch on an ephemeral ref,
// applies the acceptance rules, and CAS-advances the accepted ref.
// A refusal returns an error naming the file and rule; the accepted
// ref is untouched.
func FetchAdvance(e Endpoint) (AdvanceResult, error) {
	nonce, err := readNonce()
	if err != nil {
		return AdvanceResult{}, err
	}
	fetched, err := CaptureTip(e, nonce)
	if err != nil {
		return AdvanceResult{}, err
	}
	defer CleanupRefs(e, nonce)

	acceptedOut, acceptedErr := goalGit(e.Root, nil, "rev-parse", "--verify", "--quiet", AcceptedRef)
	accepted := strings.TrimSpace(acceptedOut)

	if acceptedErr == nil && accepted == fetched {
		return AdvanceResult{Tip: accepted, Detail: "already at the canonical tip"}, nil
	}

	// The ledger identity binds "same ledger" semantically (R7-12):
	// re-pointing config at a different remote or branch cannot
	// silently select another ledger, whatever the strings say.
	if acceptedErr == nil {
		if err := AcceptanceGates(e.Root, accepted, fetched); err != nil {
			return AdvanceResult{}, err
		}
	}

	// The whole tree validates or nothing moves.
	if err := ValidateCommit(e.Root, fetched); err != nil {
		return AdvanceResult{}, err
	}

	// The rollback DISCRIMINATION (R8-11): a descendant revert
	// restoring an older valid state is accepted — the tree is the
	// truth — with the prefix diagnosis REPORTED, never gating.
	detail := "accepted " + short(fetched)
	if acceptedErr == nil {
		if diagnosed, diagErr := PrefixDiagnosis(e.Root, accepted, fetched); diagErr == nil && len(diagnosed) > 0 {
			detail += "; " + strings.Join(diagnosed, "; ")
		}
	}
	if err := AdvanceAccepted(e.Root, fetched); err != nil {
		return AdvanceResult{}, err
	}
	return AdvanceResult{Tip: fetched, Advanced: true, Detail: detail}, nil
}

// AcceptanceGates are the rules that stand between ANY operation
// and a fetched tip (F5: the read side had them, mutations bypassed
// them). Both facts first, strongest name wins: a READABLE foreign
// identity is a foreign ledger whatever its ancestry; a tip that
// does not descend (a rewound branch, or a tip with no ledger at
// all) is a rewind; a descendant whose root record is torn falls
// through to the tree validator, which names the file and rule.
func AcceptanceGates(root, accepted, fetched string) error {
	acceptedIdentity, idErr := treeIdentity(root, accepted)
	if idErr != nil {
		return fmt.Errorf("the accepted tree's identity cannot be read: %w", idErr)
	}
	fetchedIdentity, _ := treeIdentity(root, fetched)
	if fetchedIdentity != "" && fetchedIdentity != acceptedIdentity {
		return fmt.Errorf("foreign ledger refused: the fetched tree's identity %s is not this ledger's %s — config cannot silently change what the ledger is", fetchedIdentity, acceptedIdentity)
	}
	if _, ancErr := goalGit(root, nil, "merge-base", "--is-ancestor", accepted, fetched); ancErr != nil {
		return fmt.Errorf("rewound canonical branch refused: %s does not descend from the accepted tip %s; the projection stays pinned — repair --accept-remote is the deliberate path", short(fetched), short(accepted))
	}
	return nil
}

// treeIdentity reads the root record's adoption identity at a
// commit — the one fact that means "same ledger".
func treeIdentity(root, commit string) (string, error) {
	out, err := gitIn(root, "cat-file", "-p", commit+":"+goalsPrefix+"backlog.md")
	if err != nil {
		return "", fmt.Errorf("no root record at %s: %w", short(commit), err)
	}
	rootRecord, problems := ParseRoot([]byte(out))
	if len(problems) > 0 || rootRecord.Identity == "" {
		return "", fmt.Errorf("the root record at %s does not parse to an identity", short(commit))
	}
	return rootRecord.Identity, nil
}

func readNonce() (string, error) {
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "read-" + hex.EncodeToString(raw), nil
}

func short(oid string) string {
	if len(oid) > 12 {
		return oid[:12]
	}
	return oid
}
