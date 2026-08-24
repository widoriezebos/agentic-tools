#!/usr/bin/env bash
# Evidence-schema drift fixtures: the measuring kit's evidence schemas
# must evolve WITH the engine. Born 2026-08-23 after bm-2d rep 1 was
# ruled invalid by a kit still validating pre-snapshot-scope mission
# states (KI-adjacent: the third engine/kit drift that weekend). The
# leg creates a REAL mission state with the CURRENT engine and
# validates it against the kit's own evidence schema, so an engine
# landing that moves the state shape without moving the ruler cannot
# land green. Boundary: the fixture proves the fresh-state and
# open-marker-free shapes; deeper shapes (resolutions, verification
# entries) are guarded by the schemas' own review until a cheap
# generator exists for them.
set -euo pipefail

kit=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
top=$(cd "$kit/.." && pwd -P)
ms="$top/metasystem/bin/metasystem"
[[ -x "$ms" ]] || { echo "evidence drift: no engine binary at $ms" >&2; exit 1; }

tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-evidence-drift.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
root="$tmp/checkout"
mkdir -p "$root"
git -C "$root" init -q -b main
git -C "$root" config user.name fixture
git -C "$root" config user.email fixture@example.invalid
# artifacts/ stays outside the wall's shippable snapshot, exactly as the
# deployment's projection boundary rules — without this the mission's own
# runtime files would enter the baseline projection and refuse admission.
printf 'artifacts/\nbin/\nmetasystem.conf\n' >"$root/.gitignore"
printf 'seed\n' >"$root/README"
git -C "$root" add .gitignore README
git -C "$root" commit -qm seed

mission_dir="$root/artifacts/agents/missions/drift"
mkdir -p "$mission_dir"
cat >"$mission_dir/mission-drift.contract.md" <<'CONTRACT'
# Drift fixture mission

```mission
candidate.branch=main
stream.alpha=Prove the evidence schema tracks the engine.
```
CONTRACT
"$ms" mission ledger-init --file "$mission_dir/ledger.md" --cycle-budget 5 --no-gain-budget 3 >/dev/null
"$ms" mission state-init \
  --state "$mission_dir/state.json" \
  --contract "$mission_dir/mission-drift.contract.md" \
  --ledger "$mission_dir/ledger.md" \
  --branch main \
  --baseline "$(git -C "$root" rev-parse 'HEAD^{tree}')" >/dev/null \
  || { echo "evidence drift: the CURRENT engine could not create a state" >&2; exit 1; }

python3 - "$kit" "$mission_dir/state.json" <<'PY'
import json, sys
from pathlib import Path
kit = Path(sys.argv[1])
sys.path.insert(0, str(kit))
from extractor import schema_violations, read_schema
state = json.loads(Path(sys.argv[2]).read_text(encoding="utf-8"))
violations = schema_violations(state, read_schema("evidence/mission-state.schema.json"))
if violations:
    print("evidence drift: the CURRENT engine's fresh state violates the kit's evidence schema:", file=sys.stderr)
    for v in violations[:10]:
        print("  " + v, file=sys.stderr)
    print("the measuring kit's ruler no longer matches the engine; align benchmark/schemas/evidence/ in THIS landing", file=sys.stderr)
    sys.exit(1)
print("evidence drift: the current engine's state validates under the kit's evidence schema")
PY
echo "evidence drift fixtures: PASSED"
