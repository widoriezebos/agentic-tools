#!/usr/bin/env bash
# prime-benchmark-m2.sh — fill ~/.m2 so a benchmark spec's OFFLINE gate can pass.
#
#   ./prime-benchmark-m2.sh                  prime for benchmark/specs/bm-1
#   ./prime-benchmark-m2.sh --seed <dir>     prime for another spec's seed
#   ./prime-benchmark-m2.sh --check          verify only; change nothing
#
# Run it on the host, or inside a guest after `limactl shell`. It needs network
# once; everything it fetches is what the gate then finds offline.
#
# WHY THIS EXISTS
#   bm-1's gate is `./mvnw -o test` -- deliberately offline, so a run is
#   reproducible and cannot silently acquire dependencies. Offline only works
#   if ~/.m2 already holds every artifact the build resolves. Two traps:
#
#   1. Surefire downloads its test PROVIDER (surefire-junit-platform) at test
#      run time, not at package time. A repository warmed by a build that
#      skipped tests does not contain it, and `mvnw -o test` then fails to
#      resolve the provider. The gate goes green ONLY if the project ships zero
#      tests -- i.e. the environment rewards deleting the tests. That is why
#      this script primes by running a real (trivial) test, not by packaging.
#
#   2. ./mvnw is not the system mvn. The wrapper downloads its OWN pinned Maven
#      distribution into ~/.m2/wrapper/dists on first use. Priming with system
#      mvn leaves that missing, so the gate still reaches for the network.
#      This script primes through the seed's own ./mvnw for that reason.
#
#   3. gate.sh runs BOTH `./mvnw -o -q package` and `./mvnw -o test`. Priming
#      through `test` alone is not enough: maven-jar-plugin resolves its own
#      dependencies (file-management, plexus-utils, ...) only when the jar goal
#      executes, which is the package phase. So prime with `package` -- it runs
#      the tests on the way -- and verify BOTH commands offline.
#
#   Seen for real: the VM's ~/.m2 was a side effect of the descartes-mcp build
#   (`mvn ... package -DskipTests`), so it carried junit 5.11.x, jar-plugin
#   3.5.0 and no test provider, while bm-1's pom asks for 5.13.4, 3.4.2 and
#   surefire 3.5.2. The mission parked rather than delete its tests to pass.
#   Then a first version of THIS script primed via `test` and a fresh VM built
#   from the template passed `-o test` but failed `-o package` on the jar
#   plugin's dependencies -- trap 3 above.
set -euo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
root=$(cd "$here/../.." && pwd -P)
seed=$root/benchmark/specs/bm-1/seed
check_only=0

while (($#)); do
  case "$1" in
    --seed)  [[ $# -ge 2 ]] || { echo "--seed needs a directory" >&2; exit 2; }; seed=$2; shift 2 ;;
    --check) check_only=1; shift ;;
    -h|--help) sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

[[ -d $seed ]] || { echo "no such seed directory: $seed" >&2; exit 2; }
[[ -f $seed/pom.xml && -x $seed/mvnw ]] \
  || { echo "seed carries no Maven build: $seed" >&2; exit 2; }

work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
# The seed itself is the source of truth for every version -- nothing is
# restated here, so this cannot drift from the spec the way a pinned list can.
cp -R "$seed/." "$work/"
cd "$work"

# A trivial test is the whole point: it is what pulls the Surefire provider.
mkdir -p src/test/java
cat >src/test/java/PrimeProbeTest.java <<'JAVA'
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.assertTrue;

class PrimeProbeTest {
  @Test
  void primesTheSurefireProvider() {
    assertTrue(true);
  }
}
JAVA

# The gate's two commands, verbatim from gate.sh. Both must pass offline.
verify_offline() {
  ./mvnw -o -B -q package >"$work/offline-package.log" 2>&1 \
    || { echo "OFFLINE FAIL: ./mvnw -o -q package" >&2; tail -20 "$work/offline-package.log" >&2; return 1; }
  ./mvnw -o -B test >"$work/offline-test.log" 2>&1 \
    || { echo "OFFLINE FAIL: ./mvnw -o test" >&2; tail -20 "$work/offline-test.log" >&2; return 1; }
}

if ((check_only)); then
  echo "==> checking OFFLINE resolution against $seed"
  if verify_offline; then
    echo "OK: ./mvnw -o -q package and ./mvnw -o test both pass with a test present"
    exit 0
  fi
  echo "FAIL: the offline gate cannot pass in this environment" >&2
  echo "     run this script without --check to prime ~/.m2" >&2
  exit 1
fi

echo "==> priming ~/.m2 online (wrapper distribution, plugins + their deps, test provider)"
./mvnw -B package >"$work/online.log" 2>&1 || {
  echo "priming failed -- the online build did not pass" >&2
  tail -30 "$work/online.log" >&2
  exit 1
}

# Priming that is not verified offline has not proven anything: this second
# pass is the gate's actual condition.
echo "==> verifying the same build OFFLINE"
verify_offline || {
  echo "primed, but the offline gate still fails -- something resolves at run time" >&2
  exit 1
}

echo "==> primed and verified"
printf '    repository: %s\n' "${HOME}/.m2/repository"
printf '    wrapper:    %s\n' "$(ls "${HOME}/.m2/wrapper/dists" 2>/dev/null | tr '\n' ' ')"
