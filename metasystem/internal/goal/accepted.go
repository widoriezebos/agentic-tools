package goal

// The accepted-ref integrity lifecycle (BGS-5, D118): the
// human-reserved clone-local repair that deliberately accepts a
// rewound remote, and the opid-History rollback DISCRIMINATION — a
// descendant revert restoring an older valid state is ACCEPTED with
// the prefix diagnosis reported (R8-11); the diagnosis is never an
// acceptance gate. Terminals are beliefs, the tree is the truth:
// validation gates every acceptance, history is not authenticated.

import (
	"fmt"
	"strings"
)

// RepairAcceptRemote is the deliberate, human-attributed path past a
// rewind refusal: it accepts the CURRENT remote tip as this clone's
// accepted tree, bypassing the descent rule and nothing else — the
// tree still validates whole, and a foreign ledger still refuses.
// Purely local: no push, no canonical mutation; the journal records
// the old and new tips under the human's name.
func RepairAcceptRemote(e Endpoint, by string) (AdvanceResult, error) {
	if by == "" {
		return AdvanceResult{}, fmt.Errorf("repair --accept-remote is a human-reserved act and names its human (--by)")
	}
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

	// The repair waives DESCENT, never identity: accepting a foreign
	// ledger is a different act with no sanctioned path.
	if acceptedErr == nil {
		acceptedIdentity, idErr := treeIdentity(e.Root, accepted)
		if idErr != nil {
			return AdvanceResult{}, fmt.Errorf("the accepted tree's identity cannot be read: %w", idErr)
		}
		fetchedIdentity, idErr := treeIdentity(e.Root, fetched)
		if idErr != nil {
			return AdvanceResult{}, fmt.Errorf("repair refused: the remote tip's identity cannot be read: %w", idErr)
		}
		if fetchedIdentity != acceptedIdentity {
			return AdvanceResult{}, fmt.Errorf("repair refused: the remote tip is a foreign ledger (%s, not %s)", fetchedIdentity, acceptedIdentity)
		}
	}
	if err := ValidateCommit(e.Root, fetched); err != nil {
		return AdvanceResult{}, err
	}

	// Journal the act durably before the ref moves: clone-local,
	// created and concluded in one breath — there is no push whose
	// outcome could be unknown.
	opid := "repair-" + nonce
	intent := Intent{
		Verb: "repair-accept-remote", Args: map[string]string{
			"by": by, "oldTip": accepted, "newTip": fetched,
		},
	}
	if _, err := CreateEntry(e.Root, opid, "local", "repair", intent); err != nil {
		return AdvanceResult{}, err
	}
	if err := setAcceptedTo(e.Root, fetched, accepted); err != nil {
		_ = MarkTerminal(e.Root, opid, OutcomeAbandoned, "the accepted ref did not move: "+err.Error())
		return AdvanceResult{}, err
	}
	if err := MarkTerminal(e.Root, opid, OutcomeConfirmed,
		fmt.Sprintf("accepted ref moved %s -> %s by %s", short(accepted), short(fetched), by)); err != nil {
		return AdvanceResult{}, err
	}
	return AdvanceResult{Tip: fetched, Advanced: true,
		Detail: fmt.Sprintf("repair by %s accepted %s (was %s)", by, short(fetched), short(accepted))}, nil
}

// setAcceptedTo moves the accepted ref with the old-value assertion
// when an old value exists — the repair's LOCAL postcondition is
// "the ref equals the target", and a CAS loss surfaces rather than
// silently overwriting a concurrent advance.
func setAcceptedTo(root, newTip, oldTip string) error {
	if oldTip == "" {
		// Creation IS the compare: the empty
		// old-value refuses a concurrent creator, so a repair that
		// validated tip A can never overwrite another creator's
		// descendant B with B's own ancestor.
		_, err := goalGit(root, nil, "update-ref", AcceptedRef, newTip, "")
		return err
	}
	_, err := goalGit(root, nil, "update-ref", AcceptedRef, newTip, oldTip)
	return err
}

// PrefixDiagnosis reports every goal file in the new tree whose
// History is a strict PREFIX of the same file's History in the old
// tree — the shape of a descendant revert restoring an older state.
// A DIAGNOSTIC, never a gate (R8-11): the caller reports it and
// accepts anyway.
func PrefixDiagnosis(root, oldTip, newTip string) ([]string, error) {
	oldFiles, err := ReadCommitGoals(root, oldTip)
	if err != nil {
		return nil, err
	}
	newFiles, err := ReadCommitGoals(root, newTip)
	if err != nil {
		return nil, err
	}
	var diagnosed []string
	for _, p := range sortedKeys(newFiles) {
		oldData, existed := oldFiles[p]
		if !existed || p == goalsPrefix+"backlog.md" {
			continue
		}
		newParsed, newProblems := ParseFile(newFiles[p])
		oldParsed, oldProblems := ParseFile(oldData)
		if len(newProblems) > 0 || len(oldProblems) > 0 {
			continue
		}
		if HistoryIsPrefix(newParsed.History, oldParsed.History) {
			diagnosed = append(diagnosed, fmt.Sprintf("%s: history is a strict prefix of the accepted tree's (%d of %d lines) — a descendant revert restored an older state", p, len(newParsed.History), len(oldParsed.History)))
		}
	}
	return diagnosed, nil
}
