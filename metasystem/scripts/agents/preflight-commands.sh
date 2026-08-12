#!/usr/bin/env bash
# Production command preflight (go-production-grade Phase 1, the
# supported-platforms contract): the command inventory is a contract, not
# "a POSIX userland", so arming and adoption name anything missing up
# front instead of failing mid-operation. The inventory is derived from
# the production scripts (fixture drivers excluded; their extra tools —
# python3, perl — are suite-host concerns). shasum is deliberately absent:
# production hashing moved onto `metasystem util sha256`.
set -euo pipefail

missing=()
require() { # command, source package on Debian-family Linux
  command -v "$1" >/dev/null 2>&1 || missing+=("$1 (package: $2)")
}

require git git
require ps procps
require pgrep procps
require awk "mawk or gawk"
require sed sed
require grep grep
require tar tar
require date coreutils
require mktemp coreutils
require stat coreutils
require cksum coreutils
require find findutils
require install coreutils
require ln coreutils
require tr coreutils
require sort coreutils
require wc coreutils
require head coreutils
require tail coreutils
require cut coreutils
require tee coreutils
require uname coreutils
require basename coreutils
require dirname coreutils
require touch coreutils
require chmod coreutils
require mkdir coreutils
require rmdir coreutils
require cat coreutils
require cp coreutils
require mv coreutils
require rm coreutils

if ((${#missing[@]})); then
  echo "command preflight: this host is missing production commands:" >&2
  printf '  %s\n' "${missing[@]}" >&2
  exit 1
fi
