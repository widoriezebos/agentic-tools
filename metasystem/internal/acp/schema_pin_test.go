package acp

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

// The vendored schema artifact is THE protocol the package is
// written against (schema/PIN.md); drift fails the build so an
// upgrade is always a deliberate change with a fresh conformance
// pass.
const pinnedSchemaSHA256 = "7f1fba1561163729115247df75b67aeed02085115fbc7ef0131fb01d456c08f9"

func TestSchemaArtifactPin(t *testing.T) {
	body, err := os.ReadFile("schema/acp-v1-schema.json")
	if err != nil {
		t.Fatalf("the pinned schema artifact must be vendored: %v", err)
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != pinnedSchemaSHA256 {
		t.Fatalf("schema artifact drifted from the pin in schema/PIN.md: %s", hex.EncodeToString(sum[:]))
	}
}
