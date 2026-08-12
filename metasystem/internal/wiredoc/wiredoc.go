// Package wiredoc owns the MECHANISM of typed on-disk documents: a lossless
// envelope pairing a typed projection with the raw decoded document it came
// from, plus the one canonical encoder and the grammar-preserving decoder
// (plans/typed-documents-design.md; go-production-grade Phase 5).
//
// The records this package carries are named API surfaces shared with the
// shell. Two invariants define everything here:
//
//   - LOSSLESS: unknown keys, unknown nested structure, and null-vs-absent
//     states survive every read-modify-write cycle byte-for-byte. The typed
//     projection is a LENS over the document, never the document itself.
//   - STRUCTS NEVER TOUCH THE WIRE: encoding/json emits struct fields in
//     declaration order, which is not the canonical sorted order. A write
//     renders the projection's known fields into a COPY of the raw map and
//     encodes the merged map with the canonical encoder.
//
// Each family's projection type lives with its owning package; only the
// mechanism lives here (the atomicfile precedent).
package wiredoc

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Doc is the lossless envelope: the raw decoded document plus nothing else.
// Family packages wrap it with their typed projections; Doc itself never
// interprets a field.
type Doc struct {
	raw map[string]any
}

// Decode parses a document with the frozen grammar the current readers
// define: UseNumber (literal number spellings preserved), duplicate keys
// last-wins, and trailing bytes after the top-level value tolerated — a
// single Decode with no EOF check, which is the dispatch reader's
// long-standing contract (record.go:291). Narrowing any of this is a
// behavior change Phase 5 does not carry.
func Decode(data []byte) (*Doc, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("not a JSON object")
	}
	return &Doc{raw: object}, nil
}

// Get reads one raw field: the value and whether the key is present, so
// null-vs-absent stays distinguishable.
func (d *Doc) Get(key string) (any, bool) {
	value, present := d.raw[key]
	return value, present
}

// Set writes one raw field on a copy-on-write basis for Render; the
// envelope's own map is mutated because the envelope IS the pending write.
func (d *Doc) Set(key string, value any) { d.raw[key] = value }

// Delete removes a field, making it ABSENT (distinct from setting null).
func (d *Doc) Delete(key string) { delete(d.raw, key) }

// Raw exposes the underlying document for the permissive paths (CAS applies
// arbitrary patches under its own rules; the projection never filters it).
func (d *Doc) Raw() map[string]any { return d.raw }

// Render encodes the document in the canonical wire format every on-disk
// artifact uses: two-space indent, sorted keys (encoding/json sorts map
// keys), HTML left intact, one trailing newline.
func (d *Doc) Render() ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(d.raw); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
