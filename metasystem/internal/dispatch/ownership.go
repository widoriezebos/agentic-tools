package dispatch

import (
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

var ownershipPatchFields = map[string]bool{
	"pid": true, "pidStartedAt": true, "pidStartedAtExactMicro": true,
	"pidStartTicks": true, "bootId": true, "pgid": true,
	"ownershipProof": true,
}

type RecordedGroupProofGrant interface {
	AllowsRecordedGroupProof() bool
}

// RecordedGroupProofMatches verifies the fake-runtime launch proof used when
// the kernel cannot inspect current group members. New launch records still
// need the platform-exact identity; a legacy seconds-only record never grants
// signal authority.
func RecordedGroupProofMatches(path string, pgid int64, tag string, grant RecordedGroupProofGrant) (bool, error) {
	if grant == nil || !grant.AllowsRecordedGroupProof() {
		return false, nil
	}
	record, err := readObject(path)
	if err != nil {
		return false, err
	}
	if asString(record["runtime"]) != "fake" || asString(record["instanceTag"]) != tag {
		return false, nil
	}
	recordedPgid, ok := numInt(record["pgid"])
	if !ok || recordedPgid != pgid {
		return false, nil
	}
	ref, ok := identityRefFromObject(record)
	if !ok || !ref.NativeExact() || ref.Pid != pgid {
		return false, nil
	}
	proof, ok := record["ownershipProof"].(map[string]any)
	if !ok || asString(proof["instanceTag"]) != tag || asString(proof["source"]) != "trusted-launcher" ||
		asString(proof["provenAt"]) == "" {
		return false, nil
	}
	proofPgid, ok := numInt(proof["pgid"])
	if !ok || proofPgid != pgid {
		return false, nil
	}
	proofRef, ok := identityRefFromObject(proof)
	return ok && proofRef.NativeExact() && sameRecordedIdentity(ref, proofRef), nil
}

// BuildOwnershipPatch records one live supervisor using this platform's exact
// identity shape. The compatibility second is carried alongside it but never
// decides a new-record comparison.
func BuildOwnershipPatch(output string, pid, pgid int64, tag, provenAt string, handshakeDeadline int64, reader identity.StartReader) error {
	if output == "" || pid < 1 || pgid < 2 || tag == "" || provenAt == "" || reader == nil {
		return fmt.Errorf("dispatch: ownership patch requires output, pid, pgid, tag, proven time, and identity reader")
	}
	exact, state, err := reader.ReadStart(pid)
	if err != nil || state != identity.Alive || exact.Pid != pid {
		return fmt.Errorf("dispatch: pid %d exact start identity is unavailable", pid)
	}
	ref := exact.Ref()
	if !ref.NativeExact() {
		return fmt.Errorf("dispatch: pid %d has no platform-exact start identity", pid)
	}
	patch := exactIdentityFields(ref)
	patch["pgid"] = pgid
	proof := exactIdentityFields(ref)
	proof["pgid"] = pgid
	proof["instanceTag"] = tag
	proof["provenAt"] = provenAt
	proof["source"] = "trusted-launcher"
	patch["ownershipProof"] = proof
	if handshakeDeadline > 0 {
		patch["handshakeDeadline"] = handshakeDeadline
	}
	return writeRecord(output, patch)
}

func exactIdentityFields(ref identity.Ref) map[string]any {
	fields := map[string]any{
		"pid":          ref.Pid,
		"pidStartedAt": ref.StartedAtSec,
	}
	switch ref.Mode() {
	case identity.CompareDarwinMicroseconds:
		fields["pidStartedAtExactMicro"] = ref.StartedAtUnixMicro
	case identity.CompareLinuxTicksBootID:
		fields["pidStartTicks"] = ref.StartTicks
		fields["bootId"] = ref.BootID
	}
	return fields
}

func identityRefFromObject(value map[string]any) (identity.Ref, bool) {
	pid, pidOK := numInt(value["pid"])
	started, startedOK := numInt(value["pidStartedAt"])
	if !pidOK || pid < 1 || !startedOK || started < 1 {
		return identity.Ref{}, false
	}
	ref := identity.Ref{Pid: pid, StartedAtSec: started}
	if micro, ok := numInt(value["pidStartedAtExactMicro"]); ok {
		ref.StartedAtUnixMicro = micro
	}
	if ticks, ok := numInt(value["pidStartTicks"]); ok {
		ref.StartTicks = ticks
	}
	if bootID, ok := value["bootId"].(string); ok {
		ref.BootID = bootID
	}
	return ref, ref.Mode() != identity.CompareInvalid
}

func sameRecordedIdentity(a, b identity.Ref) bool {
	if a.Pid != b.Pid || a.Mode() != b.Mode() {
		return false
	}
	switch a.Mode() {
	case identity.CompareDarwinMicroseconds:
		return a.StartedAtUnixMicro == b.StartedAtUnixMicro
	case identity.CompareLinuxTicksBootID:
		return a.StartTicks == b.StartTicks && a.BootID == b.BootID
	case identity.CompareLegacySeconds:
		return a.StartedAtSec == b.StartedAtSec
	default:
		return false
	}
}

// validateOwnershipPatch accepts the one atomic initial ownership write and
// prevents partial, mixed-platform, or later rewrites of the process identity.
func validateOwnershipPatch(record, patch map[string]any) error {
	hasOwnership := false
	for field := range patch {
		if ownershipPatchFields[field] {
			hasOwnership = true
			break
		}
	}
	if !hasOwnership {
		return nil
	}
	if record["pid"] != nil {
		return fmt.Errorf("record patch attempts to change immutable ownership identity")
	}
	ref, ok := identityRefFromObject(patch)
	if !ok || !ref.NativeExact() {
		return fmt.Errorf("new ownership identity must carry the platform-exact representation")
	}
	pgid, pgidOK := numInt(patch["pgid"])
	proof, proofOK := patch["ownershipProof"].(map[string]any)
	proofRef, proofRefOK := identityRefFromObject(proof)
	if !pgidOK || pgid < 2 || !proofOK || !proofRefOK || !proofRef.NativeExact() ||
		!sameRecordedIdentity(ref, proofRef) || !looseEqual(proof["pgid"], pgid) {
		return fmt.Errorf("ownership proof must repeat the exact primary identity and process group")
	}
	tag := asString(record["instanceTag"])
	if tag == "" || asString(proof["instanceTag"]) != tag ||
		asString(proof["source"]) != "trusted-launcher" || asString(proof["provenAt"]) == "" {
		return fmt.Errorf("ownership proof does not match the record tag or trusted launcher")
	}
	return nil
}
