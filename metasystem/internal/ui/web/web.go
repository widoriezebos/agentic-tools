package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
)

// ErrNoManifest reports that this executable carries no bundle. bundle.json is
// the publish point: the bundle script removes it before its first destructive
// step and writes it last, so a failed build leaves none and every reader sees
// the same absent bundle.
var ErrNoManifest = errors.New("the interface bundle manifest is absent")

// ManifestPath is where the manifest sits inside the embedded tree.
const ManifestPath = "bundle/bundle.json"

// SchemaVersion is the manifest shape this package reads and the bundle script
// writes.
const SchemaVersion = 1

// FileDigest names one file and the digest of what it contributes. Path uses
// "/" separators and is ASCII; SHA256 is lower-case hexadecimal.
type FileDigest struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	SourceDigest  string            `json:"sourceDigest"`
	Source        []FileDigest      `json:"source"`
	Files         []FileDigest      `json:"files"`
	Tools         map[string]string `json:"tools"`
}

func ReadManifest() (Manifest, error) { return readManifest(bundle) }

func readManifest(fsys fs.FS) (Manifest, error) {
	data, err := fs.ReadFile(fsys, ManifestPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Manifest{}, ErrNoManifest
		}
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("the interface bundle manifest is unparsable: %w", err)
	}
	return manifest, nil
}
