package main

import "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"

// batchLedgerOwner supplies the owner bound by the verb that builds a batch store.
var batchLedgerOwner = func(root string) (batch.LedgerOwner, error) {
	return batch.UnboundLedgerOwner{}, nil
}
