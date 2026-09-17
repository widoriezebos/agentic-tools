package batch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

type certifiedChain struct {
	ID, ReviewedTree, Digest string
	Patch                    []byte
}

// CertifiedChain is the closed implementation output accepted by join.
type CertifiedChain = certifiedChain

func refuseBatch(code, detail string) error { return fmt.Errorf("%s: %s", code, detail) }
func readJob(root, id string) (dispatch.JobRecord, error) {
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "jobs", id+".json"))
	if err != nil {
		return dispatch.JobRecord{}, err
	}
	var record map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&record); err != nil {
		return dispatch.JobRecord{}, err
	}
	return dispatch.JobRecordOf(record), nil
}
func readCertifiedChain(root, goalID, chainID string, goalRevision uint64) (certifiedChain, error) {
	implementation, err := readJob(root, chainID)
	if os.IsNotExist(err) {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_REQUIRED", "join requires a delegate --role implementer chain with a closed code-critic chain")
	}
	if err != nil || implementation.JobID() != chainID || implementation.ParentJob() != "" {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "implementation chain root is unreadable")
	}
	if implementation.Role() != "implementer" {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_NOT_IMPLEMENTATION", "chain root is not an implementer job")
	}
	if closed, _ := implementation.Raw()["chainClosed"].(bool); !closed {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNCLOSED", "implementation chain is open")
	}
	if err := dispatch.CloseCheck(root, chainID); err != nil {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "implementation chain closure is invalid")
	}
	revision, present := implementation.GoalRevision()
	if !present || implementation.GoalID() != goalID {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "implementation chain does not bind the named goal")
	}
	if revision != goalRevision {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_REVISION_MOVED", fmt.Sprintf("chain goal revision is %d, claim revision is %d", revision, goalRevision))
	}
	criticID, _ := implementation.Raw()["independentCritiqueJobRef"].(string)
	critic, err := readJob(root, criticID)
	if err != nil || critic.JobID() != criticID || critic.ParentJob() != "" {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "closed code-critic root is unreadable")
	}
	if critic.Role() != "code-critic" {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "critic chain root is not a code-critic job")
	}
	if closed, _ := critic.Raw()["chainClosed"].(bool); !closed {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNCLOSED", "code-critic chain is open")
	}
	if err := dispatch.CloseCheck(root, criticID); err != nil {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "code-critic chain closure is invalid")
	}
	closure, present, err := dispatch.ReadClosure(critic.Raw())
	if err != nil || !present || closure.CriticRoot != criticID {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "code-critic closure is unreadable")
	}
	if closure.Subject.Kind != dispatch.SubjectLive {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "code-critic closure subject is not live")
	}
	if closure.Subject.ImplementerRoot != chainID {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "code-critic closure certifies another implementation chain")
	}
	member, err := readJob(root, closure.Subject.ReviewedMember)
	if err != nil {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "certified implementer round is unreadable")
	}
	round, roundOK := member.Round()
	if member.JobID() != closure.Subject.ReviewedMember || member.Role() != "implementer" || !roundOK || round < 1 {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "certified implementer round is unreadable")
	}
	members, err := dispatch.ChainMemberStatuses(filepath.Join(root, "artifacts", "agents", "jobs"), chainID, false)
	if err != nil || !slices.Contains(members, member.JobID()+"|"+member.Status()) {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "certified implementer is outside implementation chain")
	}
	patch, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", chainID, "rounds", fmt.Sprint(round), "diff.patch"))
	if digest := sha256.Sum256(patch); err != nil || hex.EncodeToString(digest[:]) != closure.Subject.DiffDigest {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "certified diff is unreadable or changed")
	}
	return certifiedChain{ID: chainID, ReviewedTree: closure.Subject.ReviewedProjectTree, Digest: closure.Subject.DiffDigest, Patch: patch}, nil
}
func transportChain(root string, chain certifiedChain) error {
	target := filepath.Join(root, "artifacts", "agents", "landing-batches", "chains", chain.ID, "diff.patch")
	if prior, err := os.ReadFile(target); err == nil {
		if bytes.Equal(prior, chain.Patch) {
			return nil
		}
		return refuseBatch("BATCH_CHAIN_CONFLICT", "landing root already carries different bytes for chain "+chain.ID)
	} else if !os.IsNotExist(err) {
		return err
	}
	durable, err := atomicfile.WriteText(target, string(chain.Patch), root)
	if err != nil || !durable {
		return fmt.Errorf("transport chain %s: durable=%v: %w", chain.ID, durable, err)
	}
	return nil
}

func ReadCertifiedChain(root, goalID, chainID string, goalRevision uint64) (CertifiedChain, error) {
	return readCertifiedChain(root, goalID, chainID, goalRevision)
}

func TransportChain(root string, chain CertifiedChain) error { return transportChain(root, chain) }
