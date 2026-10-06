#!/bin/sh
# Builds the reporter to one fixed, ignored path and overwrites it each run;
# nothing is removed (no rm on a variable path).
cd "$(dirname "$0")/.." || { printf 'LANDING-NOT-RUN\tenvironment\n'; exit 1; }
if ! go build -o proof/.full-reporter ./proof; then
    printf 'LANDING-NOT-RUN\tenvironment\n'
    exit 1
fi
exec proof/.full-reporter "$@"
