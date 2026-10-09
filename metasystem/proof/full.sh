#!/bin/sh
# Each invocation keeps its reporter in a directory of its own.
cd "$(dirname "$0")/.." || { printf 'LANDING-NOT-RUN\tenvironment\n'; exit 1; }
reporter_dir=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-full-reporter.XXXXXX") || { printf 'LANDING-NOT-RUN\tenvironment\n'; exit 1; }
trap 'exit 1' HUP INT TERM
if ! go build -o "$reporter_dir/reporter" ./proof; then
    printf 'LANDING-NOT-RUN\tenvironment\n'
    exit 1
fi
METASYSTEM_FULL_REPORTER="$reporter_dir/reporter"
export METASYSTEM_FULL_REPORTER
"$reporter_dir/reporter" "$@"
exit $?
