# fixture-children: seat rulings on the custodian decisions

The decisions record `fixture-children-u5d-decisions.md` (units 5e, 5f, 5g) stands except where a ruling below
changes it. The later text wins. These rulings join the design page with the record.

## R1. The ready handshake has no deadline

The owner reads descriptor 4 until it gets the `ready` line or end of file. There is no five-second read
deadline. End of file without `ready` means the custodian exited during its checks: the owner waits for it
(`Wait`), and exits 2 quoting the exit status and the last lines of the custodian log. A custodian that hangs in
its checks holds the binary until `go test` kills it for its timeout, which names the binary.

Why: a read deadline is a bound that machine load alone can break, which makes the start a flaky gate for 82
packages. The pipe already reports every exit during the checks.

## R2. Retention lands before the start is turned on

The order is: **5e retention** (the per-owner log name, the quiet custodian removing its log, the aged-log sweep),
then **5f the start on** (record items 1, 2 and 6, with R1), then **5g records** (record item 4, with the record
file's removal at completion and its aged sweep).

Why: once the start is on, every test binary leaves a log. Retention first means no landing ever leaves one log
per binary behind, and retention can be proved while the start is still behind its variable, which the
custodian witnesses already set.

## R3. "Quiet" is decided by re-reading the log

At `action=complete` the custodian re-reads its whole log file. The log is quiet only when every line in it is a
line this custodian writes that records no action: today that is its own `owner=<owner> action=complete` line.
Anything else keeps the log: a kill, kill-owner, error or scan line, and any text another writer put there, such
as a race detector report or a panic on stderr. A custodian that ends any other way (an error return, its bound,
its halt) never removes its log. The re-read happens immediately before the removal, and the removal is its last
act before returning.

Why: the log is also the custodian's stderr. A flag the custodian tracks sees only what the custodian chose to
write, never what the runtime wrote for it.

## R4. No witness asserts a duration

A witness waits for the state it needs (a probe that reports the process dead, a line present in a log, a file
gone) and fails naming the state it saw. A cap on that wait exists only to stop a hang, and a witness never says
"within N seconds". The record's "gone within two seconds" (item 4) reads "gone, with a hang cap".

Why: the load rule. A timing assertion is a test that load alone can break.

## R5. The start is proved in the builder's sandbox before it is turned on

Before 5f's read, the seat runs one `testenv.Main` package with the start on inside the Codex sandbox and on the
host. If the sandbox refuses the start (the fork, the pipes, or the per-pid probe), 5f stops and the seat reports
to Wido before anything lands.

Why: builders run `go test` in that sandbox. A start that exits 2 there blocks every later build.

## R6. `proc custodian` keeps its log

Only a custodian started by `testenv.Main` removes its log. `metasystem proc custodian` (the harness beds) keeps
its log as today; beds are unit 10's.

Why: no existing behaviour changes (Wido, R-115-m1e), and nothing yet says what reads a bed's log afterwards.
