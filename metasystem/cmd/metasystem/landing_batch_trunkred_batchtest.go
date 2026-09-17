//go:build batchtest

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

func init() { productionTrunkRedLedgerOwner = newBatchTestFileLedgerOwner }

type batchTestFileLedgerOwner struct{ path string }

func newBatchTestFileLedgerOwner(root string) batch.LedgerOwner {
	return &batchTestFileLedgerOwner{path: filepath.Join(root, "memory", "flake-registry.md")}
}

func (owner *batchTestFileLedgerOwner) Record(_ string, red batch.TrunkRed) ([]batch.EntryRef, error) {
	data, err := os.ReadFile(owner.path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(owner.path), 0o755); err != nil {
		return nil, err
	}
	refs := make([]batch.EntryRef, 0, len(red.Groups))
	text := string(data)
	for _, group := range red.Groups {
		id := batch.TrunkRedID(group)
		refs = append(refs, batch.EntryRef{ID: id, Group: group.ID})
		marker := "<!-- batch-trunk-red:" + id + " -->"
		if !strings.Contains(text, marker) {
			text += fmt.Sprintf("\n%s\n- OPEN %s batch=%s attempt=%s tree=%s\n", marker, group.ID, red.BatchID, red.AttemptID, red.BaseTree)
		}
	}
	if err := os.WriteFile(owner.path, []byte(text), 0o644); err != nil {
		return nil, err
	}
	return refs, nil
}

func (*batchTestFileLedgerOwner) Clear(string, batch.EntryRef, batch.Green) error { return nil }
func (*batchTestFileLedgerOwner) Open() ([]batch.OpenEntry, error)                { return nil, nil }
