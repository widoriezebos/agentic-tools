package config

import "testing"

func TestHostProofVMIsOptionalAndNotAProofInput(t *testing.T) {
	t.Parallel()
	value, known := CompiledDefault("host.proof-vm")
	if !known || value != "" || ProofInput("host.proof-vm") || CommittedOnly("host.proof-vm") {
		t.Fatalf("host proof VM: default=%q known=%t proof input=%t committed only=%t", value, known, ProofInput("host.proof-vm"), CommittedOnly("host.proof-vm"))
	}
}
