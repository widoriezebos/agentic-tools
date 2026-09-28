#!/usr/bin/env bash
# Stub for callers outside Go that name this path (the gate scripts,
# fixtures): the fenced, pinned, stamped engine build is
# `go run ./cmd/devgate build` (verbs-object-action 3.3). It resolves its own
# installation directory so a caller in any directory still builds this tree.
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
exec go -C "$root" run -trimpath ./cmd/devgate build "$@"
