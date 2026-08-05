# Calibration probe

This repository is a deliberately flawed calibration program, not a reference
implementation. Its exact flaw and preservation vector are in `probe.json`.

The small local build script exists because Maven-dependent metrics are outside
every probe vector. It packages the Java probe without network access.
