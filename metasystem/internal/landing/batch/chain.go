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
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_REQUIRED", "goal "+goalID+" has no reviewed build to land; metasystem work build "+goalID+" makes one")
	}
	if err != nil || implementation.JobID() != chainID || implementation.ParentJob() != "" {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "the build of goal "+goalID+" can't be read; metasystem work status "+goalID+" shows its jobs")
	}
	if implementation.Role() != "implementer" {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_NOT_IMPLEMENTATION", "job "+chainID+" is not a build; metasystem work status "+goalID+" names the goal's builds")
	}
	if closed, _ := implementation.Raw()["chainClosed"].(bool); !closed {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNCLOSED", "the build of goal "+goalID+" is not reviewed yet; metasystem work review "+goalID+" reviews it")
	}
	if err := dispatch.CloseCheck(root, chainID); err != nil {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "the build of goal "+goalID+" did not close cleanly; metasystem work review "+goalID+" reviews it")
	}
	revision, present := implementation.GoalRevision()
	if !present || implementation.GoalID() != goalID {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "job "+chainID+" is not a build of goal "+goalID+"; metasystem work status "+goalID+" names its builds")
	}
	if revision != goalRevision {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_REVISION_MOVED", fmt.Sprintf("goal %s changed after its build (revision %d, now %d); metasystem work build %s builds it again", goalID, revision, goalRevision, goalID))
	}
	criticID, _ := implementation.Raw()["independentCritiqueJobRef"].(string)
	critic, err := readJob(root, criticID)
	if err != nil || critic.JobID() != criticID || critic.ParentJob() != "" {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "the review of goal "+goalID+" can't be read; metasystem work review "+goalID+" reviews it again")
	}
	if critic.Role() != "code-critic" {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "the review of goal "+goalID+" is not a code review; metasystem work review "+goalID+" reviews it")
	}
	if closed, _ := critic.Raw()["chainClosed"].(bool); !closed {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNCLOSED", "the review of goal "+goalID+" is still running; metasystem work wait "+goalID+" waits for it")
	}
	if err := dispatch.CloseCheck(root, criticID); err != nil {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "the review of goal "+goalID+" did not close cleanly; metasystem work review "+goalID+" reviews it again")
	}
	closure, present, err := dispatch.ReadClosure(critic.Raw())
	if err != nil || !present || closure.CriticRoot != criticID {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "the review of goal "+goalID+" has no readable verdict; metasystem work review "+goalID+" reviews it again")
	}
	if closure.Subject.Kind != dispatch.SubjectLive {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "the review of goal "+goalID+" read no live build; metasystem work review "+goalID+" reviews the build")
	}
	if closure.Subject.ImplementerRoot != chainID {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "the review of goal "+goalID+" certifies another build; metasystem work review "+goalID+" reviews this one")
	}
	member, err := readJob(root, closure.Subject.ReviewedMember)
	if err != nil {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "the reviewed round of goal "+goalID+" can't be read; metasystem work review "+goalID+" reviews it again")
	}
	round, roundOK := member.Round()
	if member.JobID() != closure.Subject.ReviewedMember || member.Role() != "implementer" || !roundOK || round < 1 {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "the reviewed round of goal "+goalID+" can't be read; metasystem work review "+goalID+" reviews it again")
	}
	members, err := dispatch.ChainMemberStatuses(filepath.Join(root, "artifacts", "agents", "jobs"), chainID, false)
	if err != nil || !slices.Contains(members, member.JobID()+"|"+member.Status()) {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "the reviewed round of goal "+goalID+" is not part of its build; metasystem work review "+goalID+" reviews it again")
	}
	patch, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", chainID, "rounds", fmt.Sprint(round), "diff.patch"))
	if digest := sha256.Sum256(patch); err != nil || hex.EncodeToString(digest[:]) != closure.Subject.DiffDigest {
		return certifiedChain{}, refuseBatch("BATCH_JOIN_CHAIN_UNREAD", "the reviewed change of goal "+goalID+" is missing or changed; metasystem work review "+goalID+" reviews it again")
	}
	return certifiedChain{ID: chainID, ReviewedTree: closure.Subject.ReviewedProjectTree, Digest: closure.Subject.DiffDigest, Patch: patch}, nil
}
func transportChain(root string, chain certifiedChain) error {
	target := filepath.Join(root, "artifacts", "agents", "landing-batches", "chains", chain.ID, "diff.patch")
	if prior, err := os.ReadFile(target); err == nil {
		if bytes.Equal(prior, chain.Patch) {
			return nil
		}
		return refuseBatch("BATCH_CHAIN_CONFLICT", "the landing lane holds another build "+chain.ID+"; metasystem landing status shows the batch that has it")
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
