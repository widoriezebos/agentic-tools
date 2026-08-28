# The pinned ACP schema artifact

- File: acp-v1-schema.json (vendored verbatim)
- Source: https://raw.githubusercontent.com/agentclientprotocol/agent-client-protocol/main/schema/v1/schema.json
- Upstream commit: 896ab28c7a5c (2026-08-16T10:42:06Z)
- SHA-256: 7f1fba1561163729115247df75b67aeed02085115fbc7ef0131fb01d456c08f9

This is the ONE schema release the implementation and every
fixture are written against (records/acp/acp-transport-design.md,
"Protocol pins"). The digest authenticates the LOCAL artifact
before builds trust it — initialize verifies only the negotiated
wire version, because InitializeResponse carries no digest.
Adopting a newer artifact is a deliberate change with a fresh
conformance pass, never a dependency bump. The pin test
(schema_pin_test.go) fails the build if the vendored bytes drift
from this digest.
