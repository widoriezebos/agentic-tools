# Held-out grader

Nothing in this directory is copied into the repository being built. The
grader builds and drives the produced repository only through its documented
Maven and command-line surfaces, except for the declared `pom.xml` dependency
inspection in requirement 24.

Run:

```sh
./grade.sh <path-to-produced-repository>
```

Successful measurement exits zero and writes only the metric and watch line
grammars declared in `../manifest.json` to standard output. A product failure
is a score, not a grader failure. `checks.md` freezes the separately counted
acceptance battery. `calibrate.sh` runs the five declared probes three times
and rewrites `calibration.md` only after every target and must-not-disturb
assertion passes.

There is no reference implementation and no mutation testing in version 0.1.
